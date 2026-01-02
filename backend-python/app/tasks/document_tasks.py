"""
Document processing Celery tasks.

This module contains async tasks for processing documents:
- Parsing files
- Generating embeddings
- Upserting vectors to Qdrant
- Notifying Go of status changes via gRPC (no direct DB writes)

Note: These tasks run synchronously in Celery workers, outside of the
async gRPC server context.

IMPORTANT: Python does NOT write directly to the `documents` table.
All status updates are sent to Go via gRPC, which handles:
- Updating document status in the database
- Refunding storage quota on ERROR
"""

import logging
import uuid
from pathlib import Path
from typing import Optional

from celery_app import celery_app
from config import settings
from pb import rag_service_pb2 as rs
from qdrant_client import QdrantClient, models
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.services.grpc.api_grpc_client import update_document_status_via_grpc


def get_sync_db_session() -> Session:
    """Create a synchronous SQLAlchemy session for Celery tasks."""
    from sqlalchemy import create_engine
    from sqlalchemy.orm import sessionmaker

    engine = create_engine(
        settings.sync_database_url,
        pool_pre_ping=True,
        pool_recycle=3600,
    )
    SessionLocal = sessionmaker(bind=engine)
    return SessionLocal()


def notify_status_update(
    document_id: str,
    status: int,
    chunks_count: int = 0,
    error_message: str = "",
) -> bool:
    """
    Notify Go of a document status update via gRPC.

    This function replaces direct SQL updates to the documents table.
    Go will handle:
    - Updating the document status in the database
    - Refunding storage quota on ERROR status

    Args:
        document_id: UUID of the document (from Go)
        status: New status (PROCESSING, COMPLETED, ERROR) - use rs.DOCUMENT_STATUS_* constants
        chunks_count: Number of chunks created (for COMPLETED status)
        error_message: Error message (for ERROR status)

    Returns:
        True if the update was successful, False otherwise
    """
    logger = logging.getLogger(__name__)
    logger.info(
        f"[notify_status_update] Notifying Go: doc_id={document_id}, "
        f"status={rs.DocumentProcessingStatus.Name(status)}, chunks={chunks_count}"  # type: ignore[arg-type]
    )

    return update_document_status_via_grpc(
        document_id=document_id,
        status=status,
        chunks_count=chunks_count,
        error_message=error_message,
    )


def store_chunks_sync(
    document_id: str,
    text_chunks: list[str],
    metadatas: list[dict],
) -> list[str]:
    """
    Store document chunks in PostgreSQL synchronously.

    Args:
        document_id: The document ID (UUID from Go)
        text_chunks: List of text content for each chunk
        metadatas: List of metadata dicts (e.g., {"page": 1})

    Returns:
        List of generated chunk IDs (UUIDs)
    """
    session = get_sync_db_session()
    chunk_ids = []

    try:
        for idx, (content, meta) in enumerate(zip(text_chunks, metadatas)):
            chunk_id = str(uuid.uuid4())
            chunk_ids.append(chunk_id)

            query = text("""
                INSERT INTO document_chunks (id, document_id, chunk_index, content, page_number, created_at)
                VALUES (:id, :document_id, :chunk_index, :content, :page_number, NOW())
            """)
            session.execute(
                query,
                {
                    "id": chunk_id,
                    "document_id": document_id,
                    "chunk_index": idx,
                    "content": content,
                    "page_number": meta.get("page"),
                },
            )

        session.commit()
        return chunk_ids
    except Exception as e:
        session.rollback()
        raise e
    finally:
        session.close()


def get_document_parser():
    """Create a document parser instance for sync Celery tasks."""
    from logger import AppLogger
    from services import DocumentParser

    logger = AppLogger(settings)
    return DocumentParser(settings=settings, logger=logger)


def get_embedding_generator():
    """Create an embedding generator instance for sync Celery tasks."""
    from logger import AppLogger
    from services import EmbeddingGenerator

    logger = AppLogger(settings)
    return EmbeddingGenerator(settings=settings, logger=logger)


def get_vector_store_sync():
    """Create a synchronous Qdrant client for Celery tasks."""
    return QdrantClient(host=settings.qdrant_host, port=settings.qdrant_port)


@celery_app.task(
    bind=True,
    name="tasks.document_tasks.process_document_task",
    max_retries=3,
    default_retry_delay=60,
    autoretry_for=(Exception,),
    retry_backoff=True,
)
def process_document_task(
    self,
    document_id: str,
    file_path: str,
    organization_id: int,
    group_id: Optional[int],
    owner_id: int,
    filename: str,
) -> dict:
    """
    Celery task to process a document asynchronously.

    This task:
    1. Updates document status to PROCESSING
    2. Parses the file from disk
    3. Generates embeddings for text chunks
    4. Stores chunks in PostgreSQL
    5. Upserts vectors to Qdrant with org/group metadata
    6. Updates document status to COMPLETED (or ERROR on failure)

    Args:
        document_id: UUID of the document (from Go)
        file_path: Path to the file on disk
        organization_id: Organization ID for multi-tenancy
        group_id: Group ID (None if org-wide)
        owner_id: Owner user ID
        filename: Original filename

    Returns:
        dict with status information
    """
    logger = logging.getLogger(__name__)
    logger.info(
        f"[DocumentTask] Starting processing: doc_id={document_id}, file={filename}, "
        f"org={organization_id}, group={group_id}"
    )

    # Track final status for finally block
    final_status = rs.DocumentProcessingStatus.DOCUMENT_STATUS_ERROR
    final_chunks_count = 0
    final_error_message = ""

    try:
        # 1) Update status to PROCESSING via gRPC
        notify_status_update(document_id, rs.DocumentProcessingStatus.DOCUMENT_STATUS_PROCESSING)

        # 2) Verify file exists
        file_path_obj = Path(file_path)
        if not file_path_obj.exists():
            final_error_message = f"File not found: {file_path}"
            logger.error(f"[DocumentTask] ❌ {final_error_message}")
            return {"status": "error", "message": final_error_message, "document_id": document_id}

        # 3) Check file size
        file_size = file_path_obj.stat().st_size
        max_file_size = settings.maximum_file_size
        if file_size > max_file_size:
            final_error_message = f"File size ({file_size}) exceeds maximum ({max_file_size})"
            logger.error(f"[DocumentTask] ❌ {final_error_message}")
            return {"status": "error", "message": final_error_message, "document_id": document_id}

        # 4) Parse the document
        logger.info(f"[DocumentTask] Parsing document: {filename}...")
        parser = get_document_parser()
        try:
            text_chunks, metadatas = parser.parse_file(file_path, filename)
        except ValueError as ve:
            final_error_message = str(ve)
            logger.warning(f"[DocumentTask] ⚠️ Validation Error: {final_error_message}")
            return {"status": "error", "message": final_error_message, "document_id": document_id}

        if not text_chunks:
            # No text extracted - still mark as COMPLETED but with warning
            final_status = rs.DocumentProcessingStatus.DOCUMENT_STATUS_COMPLETED
            final_chunks_count = 0
            final_error_message = "No text extracted from document"
            logger.warning(f"[DocumentTask] ⚠️ {final_error_message}")
            return {
                "status": "warning",
                "message": final_error_message,
                "document_id": document_id,
                "chunks_count": 0,
            }

        # 5) Store chunks in PostgreSQL
        logger.info(f"[DocumentTask] Storing {len(text_chunks)} chunks in database...")
        chunk_ids = store_chunks_sync(
            document_id=document_id,
            text_chunks=text_chunks,
            metadatas=metadatas,
        )

        # 6) Generate embeddings
        logger.info(f"[DocumentTask] Generating embeddings for {len(text_chunks)} chunks...")
        embedder = get_embedding_generator()
        vectors = embedder.generate_sync(text_chunks)

        # 7) Upsert vectors into Qdrant with multi-tenant metadata
        # CRITICAL: Include organization_id and group_id in payload for filtering
        logger.info("[DocumentTask] Upserting vectors into Qdrant...")
        qdrant_client = get_vector_store_sync()

        points = [
            models.PointStruct(
                id=chunk_id,
                vector=vec,
                payload={
                    "chunk_id": chunk_id,
                    "document_id": document_id,
                    "filename": filename,
                    "organization_id": organization_id,
                    "group_id": group_id,  # None for org-wide documents
                    "owner_id": owner_id,
                },
            )
            for vec, chunk_id in zip(vectors, chunk_ids)
        ]

        qdrant_client.upsert(
            collection_name=settings.qdrant_docs_collection_name,
            points=points,
        )
        qdrant_client.close()

        # 8) Mark processing as successful
        final_status = rs.DocumentProcessingStatus.DOCUMENT_STATUS_COMPLETED
        final_chunks_count = len(chunk_ids)
        final_error_message = ""

        logger.info(
            f"[DocumentTask] ✅ Document processed successfully: "
            f"doc_id={document_id}, chunks={final_chunks_count}"
        )

        return {
            "status": "success",
            "message": f"Successfully processed and indexed {final_chunks_count} chunks",
            "document_id": document_id,
            "chunks_count": final_chunks_count,
        }

    except Exception as e:
        # Capture error for finally block
        final_error_message = str(e)
        final_status = rs.DocumentProcessingStatus.DOCUMENT_STATUS_ERROR
        logger.error(f"[DocumentTask] ❌ Critical Error: {final_error_message}", exc_info=True)

        # Re-raise to trigger Celery retry mechanism
        raise self.retry(exc=e)

    finally:
        # ALWAYS notify Go of the final status via gRPC
        # This ensures quota is refunded on ERROR even if processing crashes
        try:
            notify_status_update(
                document_id=document_id,
                status=final_status,
                chunks_count=final_chunks_count,
                error_message=final_error_message,
            )
        except Exception as status_error:
            logger.error(f"[DocumentTask] Failed to notify Go of status: {status_error}")

        # ALWAYS clean up the uploaded file to prevent disk exhaustion
        # This runs whether processing succeeds or fails
        try:
            file_path_to_delete = Path(file_path)
            if file_path_to_delete.exists():
                file_path_to_delete.unlink()
                logger.info(f"[DocumentTask] 🗑️ Cleaned up file: {file_path}")
            else:
                logger.debug(f"[DocumentTask] File already removed or not found: {file_path}")
        except Exception as cleanup_error:
            logger.error(f"[DocumentTask] Failed to clean up file {file_path}: {cleanup_error}")
