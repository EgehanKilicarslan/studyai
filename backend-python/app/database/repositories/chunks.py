from typing import List, Optional

from database import Database
from database.models import DocumentChunk
from sqlalchemy import select


class DocumentChunkRepository:
    def __init__(self, db: Database):
        self.db = db

    async def create_many(self, chunks: List[DocumentChunk]) -> List[DocumentChunk]:
        """Create multiple chunks in a single transaction."""
        async with self.db.get_session() as session:
            session.add_all(chunks)
            await session.commit()
            for chunk in chunks:
                await session.refresh(chunk)
            return chunks

    async def get_by_id(self, chunk_id: str) -> Optional[DocumentChunk]:
        """Get a single chunk by ID."""
        async with self.db.get_session() as session:
            result = await session.execute(
                select(DocumentChunk).where(DocumentChunk.id == chunk_id)
            )
            return result.scalars().first()

    async def get_by_ids(self, chunk_ids: List[str]) -> List[DocumentChunk]:
        """Get multiple chunks by their IDs."""
        async with self.db.get_session() as session:
            result = await session.execute(
                select(DocumentChunk).where(DocumentChunk.id.in_(chunk_ids))
            )
            return list(result.scalars().all())

    async def get_by_document_id(self, document_id: str) -> List[DocumentChunk]:
        """Get all chunks for a document."""
        async with self.db.get_session() as session:
            result = await session.execute(
                select(DocumentChunk)
                .where(DocumentChunk.document_id == document_id)
                .order_by(DocumentChunk.chunk_index)
            )
            return list(result.scalars().all())

    async def delete_by_document_id(self, document_id: str) -> int:
        """Delete all chunks for a document. Returns number of deleted chunks."""
        async with self.db.get_session() as session:
            result = await session.execute(
                select(DocumentChunk).where(DocumentChunk.document_id == document_id)
            )
            chunks = result.scalars().all()
            count = len(list(chunks))
            for chunk in chunks:
                await session.delete(chunk)
            await session.commit()
            return count
