import re
import time
from pathlib import Path
from typing import AsyncGenerator

import fitz
import grpc
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from app.config import Settings
from app.services import EmbeddingService

from ..llm import LLMProvider


class RagService(rs_grpc.RagServiceServicer):
    def __init__(
        self, settings: Settings, llm_provider: LLMProvider, embedding_service: EmbeddingService
    ):
        self.llm: LLMProvider = llm_provider
        self.embedding_service: EmbeddingService = embedding_service
        self.max_file_size = settings.maximum_file_size
        self.allowed_file_types = {".pdf", ".txt", ".md"}

    def _validate_upload(self, request: rs.UploadRequest) -> tuple[bool, str]:
        if len(request.file_content) > self.max_file_size:
            return False, f"File size exceeds the maximum limit of {self.max_file_size} bytes."

        file_ext = Path(request.filename).suffix.lower()
        if file_ext not in self.allowed_file_types:
            return False, f"File type '{file_ext}' is not supported."

        if not re.match(r"^[\w\-. ]+$", request.filename):
            return False, "Invalid filename characters"

        return True, ""

    async def UploadDocument(
        self, request: rs.UploadRequest, context: grpc.aio.ServicerContext
    ) -> rs.UploadResponse:
        print(
            f"[RagService] UploadDocument called with filename: {request.filename} ({len(request.file_content)} bytes)"
        )

        is_valid, error_msg = self._validate_upload(request)
        if not is_valid:
            return rs.UploadResponse(status="error", chunks_count=0, message=error_msg)

        try:
            text_chunks = []
            metadatas = []

            if request.filename.lower().endswith(".pdf"):
                with fitz.open(stream=request.file_content, filetype="pdf") as doc:
                    for i in range(len(doc)):
                        page = doc[i]
                        text = page.get_text()

                        if isinstance(text, str) and text.strip():
                            text_chunks.append(text)
                            metadatas.append({"filename": request.filename, "page": i + 1})

                print(f"[RagService] Extracted {len(text_chunks)} text chunks from PDF.")
            else:
                text = request.file_content.decode("utf-8")
                text_chunks.append(text)
                metadatas.append({"filename": request.filename, "page": 1})
                print("[RagService] Extracted text from non-PDF document.")

            if not text_chunks:
                return rs.UploadResponse(
                    status="warning",
                    chunks_count=0,
                    message="No text extracted from the document.",
                )

            count = self.embedding_service.add_documents(documents=text_chunks, metadatas=metadatas)

            return rs.UploadResponse(
                status="success",
                chunks_count=count,
                message=f"Successfully indexed {count} chunks using PyMuPDF.",
            )
        except Exception as e:
            print(f"❌ Upload Error: {e}")

            import traceback

            traceback.print_exc()

            return rs.UploadResponse(status="error", chunks_count=0, message=str(e))

    async def Chat(
        self, request: rs.ChatRequest, context: grpc.aio.ServicerContext
    ) -> AsyncGenerator[rs.ChatResponse, None]:
        start_time = time.time()
        print(f"[RagService] Question received: {request.query} | Session ID: {request.session_id}")

        try:
            search_results = self.embedding_service.search(request.query, limit=3)

            context_docs = [hit["content"] for hit in search_results]

            print(f"[RagService] Retrieved {len(context_docs)} context documents from vector DB.")

            llm_error = False
            async for chunk in self.llm.generate_response(
                query=request.query, context_docs=context_docs, history=[]
            ):
                # Check if chunk is an error message
                if chunk.startswith("Error generating response"):
                    llm_error = True

                yield rs.ChatResponse(
                    answer=chunk,
                    source_documents=[],
                    processing_time_ms=0.0,
                )

            # Only send sources if LLM didn't error
            if not llm_error:
                processing_time = (time.time() - start_time) * 1000

                source_documents = []
                for hit in search_results:
                    meta = hit["metadata"]

                    doc = rs.Source(
                        filename=meta.get("filename", "Unknown file"),
                        page_number=int(meta.get("page", 1)),
                        # Truncate snippet to first 100 characters for brevity
                        snippet=hit["content"][:100].replace("\n", " ") + "...",
                        score=hit["score"],
                    )
                    source_documents.append(doc)

                yield rs.ChatResponse(
                    answer="", source_documents=source_documents, processing_time_ms=processing_time
                )

        except Exception as e:
            print(f"Error: {e}")
            yield rs.ChatResponse(
                answer="Sorry, an error occurred while generating the response.",
                source_documents=[],
                processing_time_ms=0.0,
            )
