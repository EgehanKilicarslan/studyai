from typing import Dict, List, Tuple

from database.models import Document, DocumentChunk
from database.repositories import DocumentChunkRepository, DocumentRepository
from logger import AppLogger


class DocumentService:
    def __init__(
        self,
        repo: DocumentRepository,
        chunk_repo: DocumentChunkRepository,
        logger: AppLogger,
    ):
        self.repo = repo
        self.chunk_repo = chunk_repo
        self.logger = logger.get_logger(__name__)

    async def get_or_register_document(
        self,
        user_id: int,
        filename: str,
        file_hash: str,
        chunk_count: int,
        content_type: str,
    ) -> Tuple[Document, bool]:
        """
        Get or register a document. Returns (document, is_new).
        is_new is True if this is a new document, False if it already existed.
        """
        # 1) Check if document with the same hash exists
        existing_doc = await self.repo.get_by_hash(file_hash)

        if existing_doc:
            self.logger.info(
                f"📄 Document found in Database: {filename} ({file_hash[:8]}...). Linking to user {user_id}."
            )
            # 2) Link user to document
            await self.repo.link_user_to_document(user_id, str(existing_doc.id))
            return existing_doc, False

        self.logger.info(f"🆕 Creating new document: {filename}")
        new_doc = Document(
            filename=filename,
            file_hash=file_hash,
            chunk_count=chunk_count,
            content_type=content_type,
        )
        doc = await self.repo.create(new_doc)

        # 2) Link user to document
        await self.repo.link_user_to_document(user_id, str(doc.id))

        return doc, True

    async def store_chunks(
        self,
        document_id: str,
        text_chunks: List[str],
        metadatas: List[Dict],
    ) -> List[DocumentChunk]:
        """Store document chunks in the database."""
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
        """Get chunks by their IDs."""
        return await self.chunk_repo.get_by_ids(chunk_ids)

    async def list_user_documents(self, user_id: int) -> list[Document]:
        return await self.repo.list_by_user_id(user_id)

    async def delete_document_for_user(self, user_id: int, document_id: str) -> bool:
        """
        Delete document for a user. If no other users are linked,
        fully delete the document and its chunks.
        Returns True if document was fully deleted, False if only unlinked.
        """
        # 1) Unlink user from document
        self.logger.info(f"🔗 Unlinking document {document_id} from user {user_id}.")
        await self.repo.unlink_user_from_document(user_id, document_id)

        # 2) Check if document is linked to any other users
        remaining_users = await self.repo.get_linked_user_count(document_id)

        if remaining_users == 0:
            self.logger.info(
                f"🗑️ No remaining users linked to document {document_id}. Deleting document and chunks."
            )
            # Chunks will be deleted via CASCADE
            await self.repo.delete(document_id)
            return True

        self.logger.info(
            f"📄 Document {document_id} still linked to {remaining_users} users. Not deleting."
        )
        return False
