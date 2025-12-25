import re
import time
from pathlib import Path
from typing import AsyncGenerator

import fitz
import grpc
from langchain_text_splitters import RecursiveCharacterTextSplitter
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from app.config import Settings
from app.services import EmbeddingService

from ..llm import LLMProvider


class RagService(rs_grpc.RagServiceServicer):
    def __init__(
        self,
        settings: Settings,
        llm_provider: LLMProvider,
        embedding_service: EmbeddingService,
    ):
        self.llm: LLMProvider = llm_provider
        self.embedding_service: EmbeddingService = embedding_service
        self.max_file_size = settings.maximum_file_size
        self.allowed_file_types = {".pdf", ".txt", ".md"}
        self.text_splitter = RecursiveCharacterTextSplitter(
            chunk_size=settings.embedding_chunk_size,
            chunk_overlap=settings.embedding_chunk_overlap,
            separators=["\n\n", "\n", " ", ""],
        )

    def _validate_filename(self, filename: str) -> tuple[bool, str]:
        file_ext = Path(filename).suffix.lower()
        if file_ext not in self.allowed_file_types:
            return (
                False,
                f"Unsupported file type: {file_ext}. Allowed types are: {', '.join(self.allowed_file_types)}",
            )

        if not re.match(r"^[\w\-. ]+$", filename):
            return False, "Invalid filename characters"

        return True, ""

    async def UploadDocument(
        self,
        request_iterator: AsyncGenerator[rs.UploadRequest, None],
        context: grpc.aio.ServicerContext,
    ) -> rs.UploadResponse:
        file_content = bytearray()
        filename = "unknown"
        content_type = ""
        current_size = 0

        print("[RagService] UploadDocument stream started...")

        try:
            # 1. Stream loop
            async for request in request_iterator:
                # A) Is metadata present?
                if request.HasField("metadata"):
                    filename = request.metadata.filename
                    content_type = request.metadata.content_type
                    print(f"[RagService] Receiving file: {filename} (Type: {content_type})")

                    # Validate filename and type
                    is_valid, err_msg = self._validate_filename(filename)
                    if not is_valid:
                        return rs.UploadResponse(status="error", message=err_msg)

                # B) Is chunk present?
                elif request.HasField("chunk"):
                    chunk_data = request.chunk
                    chunk_len = len(chunk_data)

                    # File size check
                    if current_size + chunk_len > self.max_file_size:
                        msg = f"File size limit exceeded ({self.max_file_size} bytes)."
                        print(f"❌ {msg}")
                        return rs.UploadResponse(status="error", message=msg)

                    # Accumulate chunk
                    file_content.extend(chunk_data)
                    current_size += chunk_len

            # 2. Stream ended, file ready
            print(f"[RagService] Upload complete. Total size: {current_size} bytes.")

            if current_size == 0:
                return rs.UploadResponse(status="warning", message="Received empty file.")

            # 3. Convert to bytes
            final_file_bytes = bytes(file_content)

            text_chunks = []
            metadatas = []

            # 4. PDF Processing
            if filename.lower().endswith(".pdf"):
                with fitz.open(stream=final_file_bytes, filetype="pdf") as doc:
                    for i in range(len(doc)):
                        page = doc[i]
                        text = page.get_text()

                        if isinstance(text, str) and text.strip():
                            page_chunks = self.text_splitter.split_text(text)
                            for chunk in page_chunks:
                                text_chunks.append(chunk)
                                metadatas.append({"filename": filename, "page": i + 1})

                print(f"[RagService] Extracted {len(text_chunks)} text chunks from PDF.")

            # 4. Text/MD Processing
            else:
                text = final_file_bytes.decode("utf-8")
                file_chunks = self.text_splitter.split_text(text)
                for chunk in file_chunks:
                    text_chunks.append(chunk)
                    metadatas.append({"filename": filename, "page": 1})
                print("[RagService] Extracted text from non-PDF document.")

            if not text_chunks:
                return rs.UploadResponse(
                    status="warning",
                    chunks_count=0,
                    message="No text extracted from the document.",
                )

            # 5. Add to Embedding Service
            count = self.embedding_service.add_documents(documents=text_chunks, metadatas=metadatas)

            # 6. Return response
            return rs.UploadResponse(
                status="success",
                chunks_count=count,
                message=f"Successfully processed and indexed {count} chunks.",
            )

        except Exception as e:
            print(f"❌ Upload Processing Error: {e}")
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
