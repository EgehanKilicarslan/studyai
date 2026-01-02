"""
Document processing Celery tasks.

This module contains async tasks for processing documents:
- Parsing files
- Generating embeddings
- Upserting vectors to Qdrant
- Updating document status in PostgreSQL

Note: These tasks run synchronously in Celery workers, outside of the
async gRPC server context.
"""

import logging
import uuid
from enum import Enum
from pathlib import Path
from typing import Optional

from celery_app import celery_app
from config import settings
from qdrant_client import QdrantClient, models
from sqlalchemy import text
from sqlalchemy.orm import Session


class DocumentStatus(str, Enum):
    """Document status enum matching Go's database schema."""

    PENDING = "PENDING"
    PROCESSING = "PROCESSING"
    COMPLETED = "COMPLETED"
    ERROR = "ERROR"


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


def update_document_status(
    document_id: str,
    status: DocumentStatus,
    chunks_count: int = 0,
    error_message: Optional[str] = None,
) -> None:
    """
    Update the document status in PostgreSQL.

    Args:
        document_id: UUID of the document (from Go)
        status: New status (PROCESSING, COMPLETED, ERROR)
        chunks_count: Number of chunks created (for COMPLETED status)
        error_message: Error message (for ERROR status)
    """
    session = get_sync_db_session()
    try:
        # Use raw SQL to update the documents table managed by Go
        if error_message:
            query = text("""
                UPDATE documents 
                SET status = :status, 
                    chunks_count = :chunks_count,
                    error_message = :error_message,
                    updated_at = NOW()
                WHERE id = :document_id
            """)
            session.execute(
                query,
                {
                    "status": status.value,
                    "chunks_count": chunks_count,
                    "error_message": error_message,
                    "document_id": document_id,
                },
            )
        else:
            query = text("""
                UPDATE documents 
                SET status = :status, 
                    chunks_count = :chunks_count,
                    error_message = NULL,
                    updated_at = NOW()
                WHERE id = :document_id
            """)
            session.execute(
                query,
                {
                    "status": status.value,
                    "chunks_count": chunks_count,
                    "document_id": document_id,
                },
            )
        session.commit()
    except Exception as e:
        session.rollback()
        raise e
    finally:
        session.close()


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

    try:
        # 1) Update status to PROCESSING
        update_document_status(document_id, DocumentStatus.PROCESSING)

        # 2) Verify file exists
        file_path_obj = Path(file_path)
        if not file_path_obj.exists():
            error_msg = f"File not found: {file_path}"
            logger.error(f"[DocumentTask] ❌ {error_msg}")
            update_document_status(document_id, DocumentStatus.ERROR, error_message=error_msg)
            return {"status": "error", "message": error_msg, "document_id": document_id}

        # 3) Check file size
        file_size = file_path_obj.stat().st_size
        max_file_size = settings.maximum_file_size
        if file_size > max_file_size:
            error_msg = f"File size ({file_size}) exceeds maximum ({max_file_size})"
            logger.error(f"[DocumentTask] ❌ {error_msg}")
            update_document_status(document_id, DocumentStatus.ERROR, error_message=error_msg)
            return {"status": "error", "message": error_msg, "document_id": document_id}

        # 4) Parse the document
        logger.info(f"[DocumentTask] Parsing document: {filename}...")
        parser = get_document_parser()
        try:
            text_chunks, metadatas = parser.parse_file(file_path, filename)
        except ValueError as ve:
            error_msg = str(ve)
            logger.warning(f"[DocumentTask] ⚠️ Validation Error: {error_msg}")
            update_document_status(document_id, DocumentStatus.ERROR, error_message=error_msg)
            return {"status": "error", "message": error_msg, "document_id": document_id}

        if not text_chunks:
            error_msg = "No text extracted from document"
            logger.warning(f"[DocumentTask] ⚠️ {error_msg}")
            update_document_status(
                document_id, DocumentStatus.COMPLETED, chunks_count=0, error_message=error_msg
            )
            return {
                "status": "warning",
                "message": error_msg,
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
            collection_name=settings.qdrant_collection_name,
            points=points,
        )
        qdrant_client.close()

        # 8) Update document status to COMPLETED
        chunks_count = len(chunk_ids)
        update_document_status(document_id, DocumentStatus.COMPLETED, chunks_count=chunks_count)

        logger.info(
            f"[DocumentTask] ✅ Document processed successfully: "
            f"doc_id={document_id}, chunks={chunks_count}"
        )

        return {
            "status": "success",
            "message": f"Successfully processed and indexed {chunks_count} chunks",
            "document_id": document_id,
            "chunks_count": chunks_count,
        }

    except Exception as e:
        error_msg = str(e)
        logger.error(f"[DocumentTask] ❌ Critical Error: {error_msg}", exc_info=True)

        # Update status to ERROR
        try:
            update_document_status(document_id, DocumentStatus.ERROR, error_message=error_msg)
        except Exception as status_error:
            logger.error(f"[DocumentTask] Failed to update status: {status_error}")

        # Re-raise to trigger Celery retry mechanism
        raise self.retry(exc=e)
