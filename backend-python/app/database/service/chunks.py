"""
Chunk Service for managing document chunks in PostgreSQL.

Note: Go is the source of truth for documents (metadata, permissions, storage).
Python only manages chunks for RAG (Retrieval Augmented Generation).
"""

from typing import Dict, List

from database.models import DocumentChunk
from database.repositories import DocumentChunkRepository
from logger import AppLogger


class ChunkService:
    """
    Service for managing document chunks.

    Chunks are stored in PostgreSQL and referenced by their IDs in Qdrant.
    The actual document metadata is managed by Go.
    """

    def __init__(
        self,
        chunk_repo: DocumentChunkRepository,
        logger: AppLogger,
    ):
        self.chunk_repo = chunk_repo
        self.logger = logger.get_logger(__name__)

    async def store_chunks(
        self,
        document_id: str,
        text_chunks: List[str],
        metadatas: List[Dict],
    ) -> List[DocumentChunk]:
        """
        Store document chunks in the database.

        Args:
            document_id: The document ID (UUID from Go)
            text_chunks: List of text content for each chunk
            metadatas: List of metadata dicts (e.g., {"page": 1})

        Returns:
            List of stored DocumentChunk objects with generated IDs
        """
        chunks = []
        for idx, (content, meta) in enumerate(zip(text_chunks, metadatas)):
            chunk = DocumentChunk(
                document_id=document_id,
                chunk_index=idx,
                content=content,
                page_number=meta.get("page"),
            )
            chunks.append(chunk)

        stored_chunks = await self.chunk_repo.create_many(chunks)
        self.logger.info(f"📦 Stored {len(stored_chunks)} chunks for document {document_id}")
        return stored_chunks

    async def get_chunks_by_ids(self, chunk_ids: List[str]) -> List[DocumentChunk]:
        """
        Get chunks by their IDs.

        Used by ChatService to retrieve chunk content after vector search.
        """
        return await self.chunk_repo.get_by_ids(chunk_ids)

    async def get_chunks_by_document_id(self, document_id: str) -> List[DocumentChunk]:
        """Get all chunks for a document, ordered by chunk_index."""
        return await self.chunk_repo.get_by_document_id(document_id)

    async def delete_chunks_by_document_id(self, document_id: str) -> int:
        """
        Delete all chunks for a document.

        Called when Go deletes a document.
        Returns the number of deleted chunks.
        """
        count = await self.chunk_repo.delete_by_document_id(document_id)
        self.logger.info(f"🗑️ Deleted {count} chunks for document {document_id}")
        return count
