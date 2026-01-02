import uuid

from database import Base
from sqlalchemy import Column, DateTime, Integer, String, Text
from sqlalchemy.sql import func


class DocumentChunk(Base):
    """
    Document chunk model for storing parsed text content.

    Note: The document_id references a document managed by Go.
    There is no foreign key constraint since Go's database is the source of truth.
    """

    __tablename__ = "document_chunks"

    id = Column(String, primary_key=True, default=lambda: str(uuid.uuid4()))
    document_id = Column(String, nullable=False, index=True)  # References Go's document ID
    chunk_index = Column(Integer, nullable=False)  # Order of chunk in document
    content = Column(Text, nullable=False)  # The actual text content
    page_number = Column(Integer, nullable=True)  # Page number (for PDFs)
    created_at = Column(DateTime(timezone=True), server_default=func.now())
