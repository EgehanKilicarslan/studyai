import asyncio
import hashlib
import tempfile
from pathlib import Path
from typing import AsyncGenerator

import grpc
from config import Settings
from database.service import DocumentService
from logger import AppLogger
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from ..document_parser import DocumentParser
from ..embedding_generator import EmbeddingGenerator
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


class KnowledgeBaseService(rs_grpc.KnowledgeBaseServiceServicer):
    def __init__(
        self,
        settings: Settings,
        logger: AppLogger,
        vector_store: VectorStore,
        embedder: EmbeddingGenerator,
        parser: DocumentParser,
        document_service: DocumentService,
    ):
        self.logger = logger.get_logger(__name__)
        self.vector_store: VectorStore = vector_store
        self.embedding_service: EmbeddingGenerator = embedder
        self.document_parser: DocumentParser = parser
        self.document_service: DocumentService = document_service
        self.max_file_size = settings.maximum_file_size

    async def UploadDocument(
        self,
        request_iterator: AsyncGenerator[rs.UploadRequest, None],
        context: grpc.aio.ServicerContext,
    ) -> rs.UploadResponse:
        # Extract user_id from gRPC metadata headers
        user_id = get_user_id_from_context(context)
        if user_id is None:
            self.logger.error("[KnowledgeBaseService] ❌ User ID not found in gRPC metadata")
            return rs.UploadResponse(status="error", message="Unauthorized: User ID not provided.")

        filename = None
        content_type = None
        current_size = 0
        temp_file = None
        temp_file_path = None
        metadata_received = False
        file_hash = hashlib.sha256()

        self.logger.info(
            f"[KnowledgeBaseService] UploadDocument stream started for user {user_id}..."
        )

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
                    content_type = request.metadata.content_type
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
                    file_hash.update(request.chunk)
                    current_size += chunk_len

            # EOF reached, finalize the file
            temp_file.close()
            computed_hash = file_hash.hexdigest()

            # 3) Validate received data
            if not metadata_received or not filename:
                return rs.UploadResponse(status="error", message="No metadata received.")

            if current_size == 0:
                return rs.UploadResponse(status="warning", message="Received empty file.")

            # 4) Parse the document
            try:
                self.logger.info(f"[KnowledgeBaseService] Parsing document: {filename}...")
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

            # 5) Register document in database and link to user
            self.logger.info(f"[KnowledgeBaseService] Registering document for user {user_id}...")
            document, is_new = await self.document_service.get_or_register_document(
                user_id=user_id,
                filename=filename,
                file_hash=computed_hash,
                chunk_count=len(text_chunks),
                content_type=content_type or "application/octet-stream",
            )

            # If document already exists (same hash), skip storing chunks and vectors
            if not is_new:
                self.logger.info(
                    f"[KnowledgeBaseService] Document already exists, linked to user {user_id}"
                )
                return rs.UploadResponse(
                    document_id=str(document.id),
                    status="success",
                    chunks_count=int(document.chunk_count or 0),  # type: ignore[arg-type]
                    message="Document already exists. Linked to your account.",
                )

            # 6) Store chunks in PostgreSQL database
            self.logger.info(
                f"[KnowledgeBaseService] Storing {len(text_chunks)} chunks in database..."
            )
            stored_chunks = await self.document_service.store_chunks(
                document_id=str(document.id),
                text_chunks=text_chunks,
                metadatas=metadatas,
            )

            # 7) Generate embeddings
            self.logger.info(
                f"[KnowledgeBaseService] Generating embeddings for {len(text_chunks)} chunks..."
            )
            vectors = await self.embedding_service.generate(text_chunks)

            # 8) Upsert vectors into Qdrant (only vectors + chunk_id references)
            self.logger.info("[KnowledgeBaseService] Upserting vectors into vector store...")
            chunk_ids = [str(chunk.id) for chunk in stored_chunks]
            count = await self.vector_store.upsert_vectors_with_chunk_ids(
                vectors=vectors,
                chunk_ids=chunk_ids,
                document_id=str(document.id),
                filename=filename,
            )

            return rs.UploadResponse(
                document_id=str(document.id),
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
                self.logger.info(f"[KnowledgeBaseService] Cleaned up temp file: {temp_file_path}")

    async def DeleteDocument(
        self,
        request: rs.DeleteDocumentRequest,
        context: grpc.aio.ServicerContext,
    ) -> rs.DeleteDocumentResponse:
        """Delete a document from the knowledge base."""
        # Extract user_id from gRPC metadata headers
        user_id = get_user_id_from_context(context)
        if user_id is None:
            self.logger.error("[KnowledgeBaseService] ❌ User ID not found in gRPC metadata")
            return rs.DeleteDocumentResponse(
                status="error", message="Unauthorized: User ID not provided."
            )

        document_id = request.document_id
        self.logger.info(
            f"[KnowledgeBaseService] DeleteDocument request for document {document_id} by user {user_id}"
        )

        try:
            # Delete document from database (this handles user unlinking and shared document logic)
            should_delete_vectors = await self.document_service.delete_document_for_user(
                user_id, document_id
            )

            # If no other users are linked to this document, delete vectors from vector store
            if should_delete_vectors:
                await self.vector_store.delete_by_document_id(document_id)
                self.logger.info(
                    f"[KnowledgeBaseService] ✅ Document {document_id} fully deleted (including vectors)"
                )
            else:
                self.logger.info(
                    f"[KnowledgeBaseService] ✅ Document {document_id} unlinked from user {user_id}"
                )

            return rs.DeleteDocumentResponse(
                status="success",
                message=f"Document {document_id} successfully deleted.",
            )

        except Exception as e:
            self.logger.error(f"❌ Delete Document Error: {e}")
            return rs.DeleteDocumentResponse(status="error", message=str(e))

    async def ListDocuments(
        self,
        request: rs.ListDocumentsRequest,
        context: grpc.aio.ServicerContext,
    ) -> rs.ListDocumentsResponse:
        """List all documents for the authenticated user."""
        # Extract user_id from gRPC metadata headers
        user_id = get_user_id_from_context(context)
        if user_id is None:
            self.logger.error("[KnowledgeBaseService] ❌ User ID not found in gRPC metadata")
            return rs.ListDocumentsResponse(documents=[])

        self.logger.info(f"[KnowledgeBaseService] ListDocuments request for user {user_id}")

        try:
            # Get documents for the user from the database
            documents = await self.document_service.list_user_documents(user_id)

            # Convert to protobuf response
            document_infos = []
            for doc in documents:
                created_ts = 0
                if doc.created_at is not None:
                    created_ts = int(doc.created_at.timestamp())
                document_infos.append(
                    rs.DocumentInfo(
                        document_id=str(doc.id),
                        filename=str(doc.filename),
                        upload_timestamp=created_ts,
                        chunks_count=int(doc.chunk_count or 0),  # type: ignore[arg-type]
                    )
                )

            self.logger.info(
                f"[KnowledgeBaseService] ✅ Found {len(document_infos)} documents for user {user_id}"
            )

            return rs.ListDocumentsResponse(documents=document_infos)

        except Exception as e:
            self.logger.error(f"❌ List Documents Error: {e}")
            return rs.ListDocumentsResponse(documents=[])
