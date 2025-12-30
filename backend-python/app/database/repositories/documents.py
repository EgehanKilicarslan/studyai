from typing import List, Optional

from database import Database
from database.models import Document, User
from database.models.relations import user_documents
from sqlalchemy import delete, func, select


class DocumentRepository:
    def __init__(self, db: Database):
        self.db = db

    async def create(self, document: Document) -> Document:
        async with self.db.get_session() as session:
            session.add(document)
            await session.commit()
            await session.refresh(document)
            return document

    async def get_by_id(self, document_id: str) -> Optional[Document]:
        async with self.db.get_session() as session:
            result = await session.execute(select(Document).where(Document.id == document_id))
            return result.scalars().first()

    async def get_by_hash(self, file_hash: str) -> Optional[Document]:
        async with self.db.get_session() as session:
            result = await session.execute(select(Document).where(Document.file_hash == file_hash))
            return result.scalars().first()

    async def link_user_to_document(self, user_id: int, document_id: str) -> None:
        async with self.db.get_session() as session:
            stmt = user_documents.insert().values(user_id=user_id, document_id=document_id)
            try:
                await session.execute(stmt)
                await session.commit()
            except Exception:
                await session.rollback()

    async def unlink_user_from_document(self, user_id: int, document_id: str) -> None:
        async with self.db.get_session() as session:
            stmt = (
                user_documents.delete()
                .where(user_documents.c.user_id == user_id)
                .where(user_documents.c.document_id == document_id)
            )
            await session.execute(stmt)
            await session.commit()

    async def get_linked_user_count(self, document_id: str) -> int:
        async with self.db.get_session() as session:
            stmt = (
                select(func.count())
                .select_from(user_documents)
                .where(user_documents.c.document_id == document_id)
            )
            result = await session.execute(stmt)
            return result.scalar() or 0

    async def list_by_user_id(self, user_id: int) -> List[Document]:
        async with self.db.get_session() as session:
            stmt = (
                select(Document)
                .join(Document.users)
                .where(User.id == user_id)
                .order_by(Document.created_at.desc())
            )
            result = await session.execute(stmt)
            return list(result.scalars().all())

    async def delete(self, document_id: str) -> None:
        async with self.db.get_session() as session:
            await session.execute(delete(Document).where(Document.id == document_id))
            await session.commit()
