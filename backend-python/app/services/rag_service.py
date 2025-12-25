import asyncio
import re
import tempfile
import time
from pathlib import Path
from typing import AsyncGenerator, Dict, List, Tuple

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

    def _parse_document_sync(self, file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
        """
        🛑 THIS METHOD CONTAINS CPU-INTENSIVE OPERATIONS (Runs synchronously).
        This method should be called within 'asyncio.to_thread'.
        """
        print(f"[Worker Thread] Parsing file: {filename} from {file_path}")
        text_chunks = []
        metadatas = []

        try:
            # A) PDF Processing
            if filename.lower().endswith(".pdf"):
                with fitz.open(file_path) as doc:
                    for i in range(len(doc)):
                        page = doc[i]
                        text = page.get_text()

                        if isinstance(text, str) and text.strip():
                            page_chunks = self.text_splitter.split_text(text)
                            for chunk in page_chunks:
                                text_chunks.append(chunk)
                                metadatas.append({"filename": filename, "page": i + 1})

                print(f"[Worker Thread] Extracted {len(text_chunks)} chunks from PDF.")

            # B) Text/MD Processing
            else:
                with open(file_path, "r", encoding="utf-8") as f:
                    text = f.read()
                file_chunks = self.text_splitter.split_text(text)
                for chunk in file_chunks:
                    text_chunks.append(chunk)
                    metadatas.append({"filename": filename, "page": 1})
                print("[Worker Thread] Extracted chunks from text file.")

            return text_chunks, metadatas

        except Exception as e:
            print(f"❌ Parsing Error: {e}")
            raise e

    async def UploadDocument(
        self,
        request_iterator: AsyncGenerator[rs.UploadRequest, None],
        context: grpc.aio.ServicerContext,
    ) -> rs.UploadResponse:
        filename = "unknown"
        current_size = 0
        temp_file = None
        temp_file_path = None

        print("[RagService] UploadDocument stream started...")

        try:
            # Create temporary file
            temp_file = tempfile.NamedTemporaryFile(mode="wb", delete=False, suffix=".tmp")
            temp_file_path = temp_file.name

            # 1. Stream Loop (Non-blocking I/O) - Write to temp file
            async for request in request_iterator:
                # Is Metadata present?
                if request.HasField("metadata"):
                    filename = request.metadata.filename
                    # Validation
                    is_valid, err_msg = self._validate_filename(filename)
                    if not is_valid:
                        return rs.UploadResponse(status="error", message=err_msg)

                # Is Chunk present?
                elif request.HasField("chunk"):
                    chunk_len = len(request.chunk)

                    if current_size + chunk_len > self.max_file_size:
                        msg = f"Limit exceeded ({self.max_file_size} bytes)."
                        return rs.UploadResponse(status="error", message=msg)

                    # Write chunk to temp file asynchronously
                    await asyncio.to_thread(temp_file.write, request.chunk)
                    current_size += chunk_len

            # Close the temp file
            temp_file.close()

            if current_size == 0:
                return rs.UploadResponse(status="warning", message="Received empty file.")

            # 3. Send CPU-Intensive Task to Thread (Parsing from temp file)
            text_chunks, metadatas = await asyncio.to_thread(
                self._parse_document_sync, temp_file_path, filename
            )

            if not text_chunks:
                return rs.UploadResponse(
                    status="warning",
                    chunks_count=0,
                    message="No text extracted from the document.",
                )

            # 4. Send to Embedding Service (Async IO)
            count = await self.embedding_service.add_documents(
                documents=text_chunks, metadatas=metadatas
            )

            return rs.UploadResponse(
                status="success",
                chunks_count=count,
                message=f"Successfully processed and indexed {count} chunks.",
            )

        except Exception as e:
            print(f"❌ Upload Error: {e}")
            return rs.UploadResponse(status="error", chunks_count=0, message=str(e))

        finally:
            # Cleanup: Close and delete temp file
            if temp_file and not temp_file.closed:
                temp_file.close()
            if temp_file_path and Path(temp_file_path).exists():
                await asyncio.to_thread(Path(temp_file_path).unlink)
                print(f"[RagService] Cleaned up temp file: {temp_file_path}")

    async def Chat(
        self, request: rs.ChatRequest, context: grpc.aio.ServicerContext
    ) -> AsyncGenerator[rs.ChatResponse, None]:
        start_time = time.time()
        print(f"[RagService] Question received: {request.query} | Session ID: {request.session_id}")

        try:
            search_results = await self.embedding_service.search(request.query, limit=3)

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
