from database import Base
from sqlalchemy import Column, ForeignKey, Integer, String, Table

# Many-to-Many: Users <-> Documents
user_documents = Table(
    "user_documents",
    Base.metadata,
    Column("user_id", Integer, ForeignKey("users.id"), primary_key=True),
    Column("document_id", String, ForeignKey("documents.id"), primary_key=True),
)
