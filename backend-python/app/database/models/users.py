from database import Base
from database.models.relations import user_documents
from sqlalchemy import BigInteger, Column, String
from sqlalchemy.orm import relationship


class User(Base):
    __tablename__ = "users"
    __tableargs__ = {"extend_existing": True}

    id = Column(BigInteger, primary_key=True)
    email = Column(String, unique=True)

    # Relationship
    documents = relationship("Document", secondary=user_documents, back_populates="users")
