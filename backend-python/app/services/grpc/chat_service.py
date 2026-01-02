import json
import time
from typing import AsyncGenerator

import grpc
from database.service import ChunkService
from llm import LLMProvider
from logger import AppLogger
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from ..embedding_generator import EmbeddingGenerator
from ..reranker_service import RerankerService
from ..vector_store import VectorStore

# gRPC metadata keys for tenant-scoped access
USER_ID_METADATA_KEY = "x-user-id"
ORGANIZATION_ID_METADATA_KEY = "x-organization-id"
GROUP_IDS_METADATA_KEY = "x-group-ids"
CHAT_HISTORY_METADATA_KEY = "x-chat-history"


def get_user_id_from_context(context: grpc.aio.ServicerContext) -> int | None:
    """Extract user ID from gRPC metadata headers."""
    invocation_metadata = context.invocation_metadata()
    if invocation_metadata is None:
        return None
    metadata = {key: value for key, value in invocation_metadata}
    user_id_str = metadata.get(USER_ID_METADATA_KEY)
    if user_id_str:
        try:
            return int(user_id_str)
        except ValueError:
            return None
    return None


def get_tenant_context_from_metadata(
    context: grpc.aio.ServicerContext,
) -> tuple[int | None, list[int] | None]:
    """
    Extract tenant context (organization_id, group_ids) from gRPC metadata headers.

    Returns:
        tuple: (organization_id, group_ids) where group_ids is a list of ints or None
    """
    invocation_metadata = context.invocation_metadata()
    if invocation_metadata is None:
        return None, None

    metadata = {key: value for key, value in invocation_metadata}

    # Extract organization_id
    org_id: int | None = None
    org_id_str = metadata.get(ORGANIZATION_ID_METADATA_KEY)
    if org_id_str:
        try:
            org_id = int(org_id_str)
        except ValueError:
            pass

    # Extract group_ids (comma-separated list)
    group_ids: list[int] | None = None
    group_ids_str = metadata.get(GROUP_IDS_METADATA_KEY)
    if group_ids_str:
        try:
            group_ids = [int(g.strip()) for g in group_ids_str.split(",") if g.strip()]
        except ValueError:
            pass

    return org_id, group_ids


def get_chat_history_from_metadata(
    context: grpc.aio.ServicerContext,
) -> list[dict[str, str]]:
    """
    Extract chat history from gRPC metadata headers.

    The chat history is passed as a JSON string in the x-chat-history header.
    Format: [{"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]

    Returns:
        list[dict[str, str]]: List of message dictionaries with 'role' and 'content' keys
    """
    invocation_metadata = context.invocation_metadata()
    if invocation_metadata is None:
        return []

    metadata = {key: value for key, value in invocation_metadata}
    history_json = metadata.get(CHAT_HISTORY_METADATA_KEY)

    if not history_json:
        return []

    try:
        history = json.loads(history_json)
        # Validate format
        if isinstance(history, list):
            # Filter to only include valid messages with role and content
            return [
                {"role": msg["role"], "content": msg["content"]}
                for msg in history
                if isinstance(msg, dict) and "role" in msg and "content" in msg
            ]
        return []
    except (json.JSONDecodeError, KeyError, TypeError):
        # Log parsing error but don't fail the request
        return []


class ChatService(rs_grpc.ChatServiceServicer):
    def __init__(
        self,
        logger: AppLogger,
        llm_provider: LLMProvider,
        vector_store: VectorStore,
        embedder: EmbeddingGenerator,
        reranker: RerankerService,
        chunk_service: ChunkService,
    ):
        self.logger = logger.get_logger(__name__)
        self.llm: LLMProvider = llm_provider
        self.vector_store: VectorStore = vector_store
        self.embedding_service: EmbeddingGenerator = embedder
        self.reranker_service: RerankerService = reranker
        self.chunk_service: ChunkService = chunk_service

    async def Chat(
        self, request: rs.ChatRequest, context: grpc.aio.ServicerContext
    ) -> AsyncGenerator[rs.ChatResponse, None]:
        start_time = time.time()

        # Extract user_id from gRPC metadata headers
        user_id = get_user_id_from_context(context)
        if user_id is None:
            self.logger.error("[ChatService] ❌ User ID not found in gRPC metadata")
            yield rs.ChatResponse(answer="Unauthorized: User ID not provided.")
            return

        # Extract tenant context for scoped search
        organization_id, group_ids = get_tenant_context_from_metadata(context)

        # Extract chat history from metadata
        chat_history = get_chat_history_from_metadata(context)

        self.logger.info(
            f"[ChatService] Question: {request.query} | Session: {request.session_id} | "
            f"User: {user_id} | Org: {organization_id} | Groups: {group_ids} | "
            f"History: {len(chat_history)} messages"
        )

        try:
            # 1) Generate embedding for the query
            query_vec = (await self.embedding_service.generate([request.query]))[0]

            # 2) Check semantic cache for similar queries
            cache_hit = await self.vector_store.search_cache(
                query_vector=query_vec,
                user_id=user_id,
                group_ids=group_ids,
                threshold=0.95,
            )

            if cache_hit:
                # Cache HIT: Return cached response immediately
                processing_time = (time.time() - start_time) * 1000
                self.logger.info(
                    f"[ChatService] 🚀 Cache HIT! Returning cached response "
                    f"(score={cache_hit.score:.4f}, time={processing_time:.2f}ms)"
                )

                # Stream the cached response
                yield rs.ChatResponse(
                    answer=cache_hit.response_text,
                    processing_time_ms=0.0,
                    is_cached=True,
                )

                # Final response with processing time (no sources for cached responses)
                yield rs.ChatResponse(
                    answer="",
                    processing_time_ms=processing_time,
                    is_cached=True,
                )
                return

            # 3) Cache MISS: Search for relevant vectors in Qdrant with tenant filtering
            raw_hits = await self.vector_store.search_with_tenant_filter(
                query_vec,
                organization_id=organization_id,
                group_ids=group_ids,
                user_id=user_id,
                limit=25,
            )

            if not raw_hits:
                self.logger.info("[ChatService] No documents found in initial search.")
                yield rs.ChatResponse(
                    answer="I couldn't find any relevant documents to answer your question."
                )
                return

            # 4) Fetch chunk content from PostgreSQL database
            chunk_ids = [str(hit.id) for hit in raw_hits]
            chunks = await self.chunk_service.get_chunks_by_ids(chunk_ids)

            # Create a mapping from chunk_id to chunk for easy lookup
            chunk_map = {str(chunk.id): chunk for chunk in chunks}

            # Build passages with content from database
            passages = []
            for hit in raw_hits:
                chunk = chunk_map.get(str(hit.id))
                if chunk:
                    passages.append(
                        {
                            "id": hit.id,
                            "text": chunk.content,
                            "meta": {
                                "chunk_id": str(chunk.id),
                                "document_id": chunk.document_id,
                                "filename": hit.payload.get("filename", "unknown")
                                if hit.payload
                                else "unknown",
                                "page": chunk.page_number,
                            },
                        }
                    )

            if not passages:
                self.logger.warning("[ChatService] No chunks found in database for vector hits")
                yield rs.ChatResponse(
                    answer="I couldn't find the document content. Please try again."
                )
                return

            self.logger.info(f"[ChatService] Reranking {len(passages)} documents...")
            ranked_results = self.reranker_service.rerank(request.query, passages, top_k=5)

            # 5) Prepare context documents
            context_docs = [res["text"] for res in ranked_results]
            self.logger.info(
                f"[ChatService] Selected {len(context_docs)} high-quality docs after rerank."
            )

            # 6) Generate answer using LLM with chat history
            llm_error = False
            full_response: list[str] = []  # Collect response for caching
            async for chunk in self.llm.generate_response(
                query=request.query,
                context_docs=context_docs,
                history=chat_history,
            ):
                # Check for LLM errors in the stream
                if chunk.startswith("Error"):
                    llm_error = True
                else:
                    full_response.append(chunk)

                # Stream the response chunk
                yield rs.ChatResponse(
                    answer=chunk,
                    processing_time_ms=0.0,
                )

            # 7) Finalize response with sources if no LLM error
            if not llm_error:
                processing_time = (time.time() - start_time) * 1000

                # Save successful response to semantic cache
                response_text = "".join(full_response)
                if response_text.strip():
                    await self.vector_store.save_to_cache(
                        query_vector=query_vec,
                        response_text=response_text,
                        user_id=user_id,
                        group_ids=group_ids,
                    )

                source_documents = []
                for res in ranked_results:
                    meta = res["meta"]
                    doc = rs.Source(
                        document_id=meta.get("document_id", ""),
                        filename=meta.get("filename", "unknown"),
                        page_number=int(meta.get("page") or 1),
                        snippet=res["text"][:100].replace("\n", " ") + "...",
                        score=res["score"],  # Reranker score
                    )
                    source_documents.append(doc)

                # Final response with sources and processing time
                yield rs.ChatResponse(
                    answer="", source_documents=source_documents, processing_time_ms=processing_time
                )

        except Exception as e:
            self.logger.error(f"❌ Chat Error: {e}")
            yield rs.ChatResponse(
                answer="Sorry, an internal error occurred while processing your request.",
                processing_time_ms=0.0,
            )
