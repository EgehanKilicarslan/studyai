import asyncio
import tempfile
import time
from pathlib import Path
from typing import AsyncGenerator

import grpc
from config import Settings
from llm import LLMProvider
from logger import AppLogger
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from .document_parser import DocumentParser
from .embedding_generator import EmbeddingGenerator
from .reranker_service import RerankerService
from .vector_store import VectorStore


class RagService(rs_grpc.RagServiceServicer):
    def __init__(
        self,
        settings: Settings,
        logger: AppLogger,
        llm_provider: LLMProvider,
        vector_store: VectorStore,
        embedder: EmbeddingGenerator,
        reranker: RerankerService,
        parser: DocumentParser,
    ):
        self.logger = logger.get_logger(__name__)
        self.llm: LLMProvider = llm_provider
        self.vector_store: VectorStore = vector_store
        self.embedding_service: EmbeddingGenerator = embedder
        self.reranker_service: RerankerService = reranker
        self.document_parser: DocumentParser = parser
        self.max_file_size = settings.maximum_file_size

    async def UploadDocument(
        self,
        request_iterator: AsyncGenerator[rs.UploadRequest, None],
        context: grpc.aio.ServicerContext,
    ) -> rs.UploadResponse:
        filename = None
        current_size = 0
        temp_file = None
        temp_file_path = None
        metadata_received = False

        self.logger.info("[RagService] UploadDocument stream started...")

        try:
            # 1) Create a temp file to store the uploaded content
            temp_file = tempfile.NamedTemporaryFile(mode="wb", delete=False, suffix=".tmp")
            temp_file_path = temp_file.name

            # 2) Process the incoming stream
            async for request in request_iterator:
                # A) Metadata check
                if request.HasField("metadata"):
                    if metadata_received:
                        return rs.UploadResponse(
                            status="error",
                            message="Metadata already received. Multiple metadata messages not allowed.",
                        )
                    filename = request.metadata.filename
                    metadata_received = True

                # B) Chunk check
                elif request.HasField("chunk"):
                    if not metadata_received:
                        return rs.UploadResponse(
                            status="error",
                            message="Security violation: Metadata must be sent before any file chunks.",
                        )

                    chunk_len = len(request.chunk)
                    if current_size + chunk_len > self.max_file_size:
                        return rs.UploadResponse(
                            status="error",
                            message=f"File size exceeds the maximum limit of {self.max_file_size} bytes.",
                        )

                    await asyncio.to_thread(temp_file.write, request.chunk)
                    current_size += chunk_len

            # EOF reached, finalize the file
            temp_file.close()

            # 3) Validate received data
            if not metadata_received or not filename:
                return rs.UploadResponse(status="error", message="No metadata received.")

            if current_size == 0:
                return rs.UploadResponse(status="warning", message="Received empty file.")

            # 4) Parse the document
            try:
                self.logger.info(f"[RagService] Parsing document: {filename}...")
                text_chunks, metadatas = await asyncio.to_thread(
                    self.document_parser.parse_file, temp_file_path, filename
                )
            except ValueError as ve:
                self.logger.warning(f"⚠️ Validation Error: {ve}")
                return rs.UploadResponse(status="error", message=str(ve))

            if not text_chunks:
                return rs.UploadResponse(
                    status="warning", message="No text extracted from document."
                )

            # 5) Generate embeddings
            self.logger.info(f"[RagService] Generating embeddings for {len(text_chunks)} chunks...")
            vectors = await self.embedding_service.generate(text_chunks)

            # 6) Upsert vectors into the vector store
            self.logger.info("[RagService] Upserting vectors into vector store...")
            count = await self.vector_store.upsert_vectors(vectors, text_chunks, metadatas)

            return rs.UploadResponse(
                status="success",
                chunks_count=count,
                message=f"Successfully processed and indexed {count} chunks.",
            )

        except Exception as e:
            self.logger.error(f"❌ Upload Critical Error: {e}")
            return rs.UploadResponse(status="error", message=str(e))

        finally:
            # Cleanup temp file
            if temp_file and not temp_file.closed:
                temp_file.close()
            if temp_file_path and Path(temp_file_path).exists():
                await asyncio.to_thread(Path(temp_file_path).unlink)
                self.logger.info(f"[RagService] Cleaned up temp file: {temp_file_path}")

    async def Chat(
        self, request: rs.ChatRequest, context: grpc.aio.ServicerContext
    ) -> AsyncGenerator[rs.ChatResponse, None]:
        start_time = time.time()
        self.logger.info(f"[RagService] Question: {request.query} | Session: {request.session_id}")

        try:
            # 1) Generate embedding for the query
            query_vec = (await self.embedding_service.generate([request.query]))[0]

            # 2) Search for relevant documents
            raw_hits = await self.vector_store.search(query_vec, limit=25)

            if not raw_hits:
                self.logger.info("[RagService] No documents found in initial search.")
                yield rs.ChatResponse(
                    answer="I couldn't find any relevant documents to answer your question."
                )
                return

            # 3) Rerank the retrieved documents
            passages = [
                {
                    "id": hit.id,
                    "text": hit.payload.get("content", "") if hit.payload else "",
                    "meta": hit.payload,
                }
                for hit in raw_hits
            ]

            self.logger.info(f"[RagService] Reranking {len(passages)} documents...")
            ranked_results = self.reranker_service.rerank(request.query, passages, top_k=5)

            # 4) Prepare context documents
            context_docs = [res["text"] for res in ranked_results]
            self.logger.info(
                f"[RagService] Selected {len(context_docs)} high-quality docs after rerank."
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
                        filename=meta.get("filename", "unknown"),
                        page_number=int(meta.get("page", 1)),
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
