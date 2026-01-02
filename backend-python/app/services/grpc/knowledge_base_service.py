from pathlib import Path

import grpc
from config import Settings
from database.service import ChunkService
from logger import AppLogger
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc

from ..document_parser import DocumentParser
from ..embedding_generator import EmbeddingGenerator
from ..vector_store import VectorStore


class KnowledgeBaseService(rs_grpc.KnowledgeBaseServiceServicer):
    """
    Knowledge Base Service for document processing.

    Go is the source of truth for documents (metadata, permissions, storage).
    Python handles:
    - Receiving document processing requests from Go
    - Triggering async processing via Celery
    - Returning immediate acknowledgment to Go

    Document processing happens asynchronously in Celery workers.
    """

    def __init__(
        self,
        settings: Settings,
        logger: AppLogger,
        vector_store: VectorStore,
        embedder: EmbeddingGenerator,
        parser: DocumentParser,
        chunk_service: ChunkService,
    ):
        self.logger = logger.get_logger(__name__)
        self.vector_store: VectorStore = vector_store
        self.embedding_service: EmbeddingGenerator = embedder
        self.document_parser: DocumentParser = parser
        self.chunk_service: ChunkService = chunk_service
        self.max_file_size = settings.maximum_file_size

    async def ProcessDocument(
        self,
        request: rs.ProcessDocumentRequest,
        context: grpc.aio.ServicerContext,
    ) -> rs.ProcessDocumentResponse:
        """
        Receive a document processing request from Go and trigger async processing.

        Go provides:
        - document_id: UUID assigned by Go (source of truth)
        - file_path: Path to the file on disk
        - filename: Original filename
        - content_type: MIME type
        - organization_id: For multi-tenancy
        - group_id: For group-level access (0 if org-wide)
        - owner_id: Document owner

        Python does:
        - Validates the request
        - Triggers Celery task for async processing
        - Returns immediate acknowledgment to Go

        The Celery worker will:
        - Parse the document from file_path
        - Generate embeddings
        - Store vectors in Qdrant with org/group metadata for filtering
        - Update document status in PostgreSQL (COMPLETED or ERROR)
        """
        document_id = request.document_id
        file_path = request.file_path
        filename = request.filename
        organization_id = request.organization_id
        group_id = request.group_id if request.group_id > 0 else None
        owner_id = request.owner_id

        self.logger.info(
            f"[KnowledgeBaseService] ProcessDocument request received: "
            f"doc_id={document_id}, file={filename}, org={organization_id}, group={group_id}"
        )

        try:
            # 1) Basic validation - verify file exists before queueing
            file_path_obj = Path(file_path)
            if not file_path_obj.exists():
                self.logger.error(f"[KnowledgeBaseService] ❌ File not found: {file_path}")
                return rs.ProcessDocumentResponse(
                    document_id=document_id,
                    status="error",
                    chunks_count=0,
                    message=f"File not found: {file_path}",
                )

            # 2) Check file size before queueing
            file_size = file_path_obj.stat().st_size
            if file_size > self.max_file_size:
                self.logger.error(
                    f"[KnowledgeBaseService] ❌ File too large: {file_size} > {self.max_file_size}"
                )
                return rs.ProcessDocumentResponse(
                    document_id=document_id,
                    status="error",
                    chunks_count=0,
                    message=f"File size ({file_size}) exceeds maximum ({self.max_file_size})",
                )

            # 3) Trigger async processing via Celery
            self.logger.info(
                f"[KnowledgeBaseService] Queueing document for async processing: {document_id}"
            )

            # Lazy import to avoid circular dependency with tasks.document_tasks
            from tasks.document_tasks import process_document_task

            # Send task to Celery worker
            task = process_document_task.delay(
                document_id=document_id,
                file_path=file_path,
                organization_id=organization_id,
                group_id=group_id,
                owner_id=owner_id,
                filename=filename,
            )

            self.logger.info(
                f"[KnowledgeBaseService] ✅ Document queued for processing: "
                f"doc_id={document_id}, task_id={task.id}"
            )

            # Return success - the request was accepted and queued
            # Go will track actual processing status via the document's status field in DB
            return rs.ProcessDocumentResponse(
                document_id=document_id,
                status="success",
                chunks_count=0,
                message=f"Document accepted for processing. Task ID: {task.id}",
            )

        except Exception as e:
            self.logger.error(f"❌ ProcessDocument Error: {e}")
            return rs.ProcessDocumentResponse(
                document_id=document_id,
                status="error",
                chunks_count=0,
                message=str(e),
            )

    async def DeleteDocument(
        self,
        request: rs.DeleteDocumentRequest,
        context: grpc.aio.ServicerContext,
    ) -> rs.DeleteDocumentResponse:
        """
        Delete a document's vectors and chunks from the knowledge base.

        Note: Go handles permission checks before calling this.
        Python just deletes the vectors and chunks for the given document_id.
        """
        document_id = request.document_id
        self.logger.info(
            f"[KnowledgeBaseService] DeleteDocument request for document {document_id}"
        )

        try:
            # 1) Delete vectors from Qdrant
            await self.vector_store.delete_by_document_id(document_id)
            self.logger.info(
                f"[KnowledgeBaseService] ✅ Deleted vectors for document {document_id}"
            )

            # 2) Delete chunks from PostgreSQL (optional - Go may handle this)
            # For now, we keep chunks in case Go needs them for other purposes
            # await self.document_service.delete_chunks_by_document_id(document_id)

            return rs.DeleteDocumentResponse(
                status="success",
                message=f"Document {document_id} vectors deleted from vector store.",
            )

        except Exception as e:
            self.logger.error(f"❌ Delete Document Error: {e}")
            return rs.DeleteDocumentResponse(status="error", message=str(e))
