import time

import fitz
import grpc
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from app.services import EmbeddingService

from ..llm import LLMProvider


class RagService(rs_grpc.RagServiceServicer):
    def __init__(self, llm_provider: LLMProvider, embedding_service: EmbeddingService):
        self.llm: LLMProvider = llm_provider
        self.embedding_service: EmbeddingService = embedding_service

    async def UploadDocument(
        self, request: rs.UploadRequest, context: grpc.aio.ServicerContext
    ) -> rs.UploadResponse:
        print(
            f"[RagService] UploadDocument called with filename: {request.filename} ({len(request.file_content)} bytes)"
        )

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
    ) -> rs.ChatResponse:
        start_time = time.time()
        print(f"[RagService] Question received: {request.query} | Session ID: {request.session_id}")

        try:
            search_results = self.embedding_service.search(request.query, limit=3)

            context_docs = [hit["content"] for hit in search_results]

            print(f"[RagService] Retrieved {len(context_docs)} context documents from vector DB.")

            answer = await self.llm.generate_response(
                query=request.query, context_docs=context_docs, history=[]
            )

            source_documents = []
            for hit in search_results:
                meta = hit["metadata"]

                doc = rs.Source(
                    filename=meta.get("filename", "Bilinmeyen Dosya"),
                    page_number=int(meta.get("page", 1)),
                    # Truncate snippet to first 200 characters for brevity
                    snippet=hit["content"][:200].replace("\n", " ") + "...",
                    score=hit["score"],
                )
                source_documents.append(doc)

                processing_time = (time.time() - start_time) * 1000

            return rs.ChatResponse(
                answer=answer, source_documents=source_documents, processing_time_ms=processing_time
            )

        except Exception as e:
            print(f"Error: {e}")
            return rs.ChatResponse(
                answer="Sorry, an error occurred while generating the response.",
                source_documents=[],
                processing_time_ms=0.0,
            )
