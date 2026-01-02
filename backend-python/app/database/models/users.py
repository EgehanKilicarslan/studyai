from database import Base
from sqlalchemy import BigInteger, Column, String


class User(Base):
    """
    User model for Python backend.

    Note: Document relationships are now managed by Go.
    This model only stores basic user info for reference.
    """

    __tablename__ = "users"
    __tableargs__ = {"extend_existing": True}

    id = Column(BigInteger, primary_key=True)
    email = Column(String, unique=True)
