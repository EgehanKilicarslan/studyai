import uuid

from database import Base
from database.models.relations import user_documents
from sqlalchemy import Column, DateTime, Integer, String
from sqlalchemy.orm import relationship
from sqlalchemy.sql import func


class Document(Base):
    __tablename__ = "documents"

    id = Column(String, primary_key=True, default=lambda: str(uuid.uuid4()))
    file_hash = Column(String, unique=True, index=True, nullable=False)
    filename = Column(String, nullable=False)
    content_type = Column(String, nullable=True, default="application/octet-stream")
    created_at = Column(DateTime(timezone=True), server_default=func.now())
    chunk_count = Column(Integer, default=0)

    # Relationships
    users = relationship("User", secondary=user_documents, back_populates="documents")
    chunks = relationship("DocumentChunk", back_populates="document", cascade="all, delete-orphan")
