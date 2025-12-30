import time
from typing import AsyncGenerator

import grpc
from database.service import DocumentService
from llm import LLMProvider
from logger import AppLogger
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from ..embedding_generator import EmbeddingGenerator
from ..reranker_service import RerankerService
from ..vector_store import VectorStore

# gRPC metadata key for user ID
USER_ID_METADATA_KEY = "x-user-id"


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


class ChatService(rs_grpc.ChatServiceServicer):
    def __init__(
        self,
        logger: AppLogger,
        llm_provider: LLMProvider,
        vector_store: VectorStore,
        embedder: EmbeddingGenerator,
        reranker: RerankerService,
        document_service: DocumentService,
    ):
        self.logger = logger.get_logger(__name__)
        self.llm: LLMProvider = llm_provider
        self.vector_store: VectorStore = vector_store
        self.embedding_service: EmbeddingGenerator = embedder
        self.reranker_service: RerankerService = reranker
        self.document_service: DocumentService = document_service

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

        self.logger.info(
            f"[ChatService] Question: {request.query} | Session: {request.session_id} | User: {user_id}"
        )

        try:
            # 1) Generate embedding for the query
            query_vec = (await self.embedding_service.generate([request.query]))[0]

            # 2) Search for relevant vectors in Qdrant
            raw_hits = await self.vector_store.search(query_vec, limit=25)

            if not raw_hits:
                self.logger.info("[ChatService] No documents found in initial search.")
                yield rs.ChatResponse(
                    answer="I couldn't find any relevant documents to answer your question."
                )
                return

            # 3) Fetch chunk content from PostgreSQL database
            chunk_ids = [str(hit.id) for hit in raw_hits]
            chunks = await self.document_service.get_chunks_by_ids(chunk_ids)

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

            # 4) Prepare context documents
            context_docs = [res["text"] for res in ranked_results]
            self.logger.info(
                f"[ChatService] Selected {len(context_docs)} high-quality docs after rerank."
            )

            # 5) Generate answer using LLM
            llm_error = False
            async for chunk in self.llm.generate_response(
                query=request.query,
                context_docs=context_docs,
                history=[],
            ):
                # Check for LLM errors in the stream
                if chunk.startswith("Error"):
                    llm_error = True

                # Stream the response chunk
                yield rs.ChatResponse(
                    answer=chunk,
                    processing_time_ms=0.0,
                )

            # 6) Finalize response with sources if no LLM error
            if not llm_error:
                processing_time = (time.time() - start_time) * 1000

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
