import contextlib
from typing import AsyncGenerator

from config import Settings
from logger import AppLogger
from sqlalchemy.exc import SQLAlchemyError
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)


class Database:
    def __init__(self, settings: Settings, logger: AppLogger):
        self.db_url = settings.database_url
        self.echo = settings.log_level == "DEBUG"
        self.engine: AsyncEngine | None = None
        self.session_factory: async_sessionmaker[AsyncSession] | None = None
        self.logger = logger.get_logger(__name__)

    async def connect(self) -> None:
        """Creates the database engine and session factory."""
        if self.engine is not None:
            self.logger.info("Database engine already initialized.")
            return

        self.logger.info("Creating database engine and session factory...")
        self.engine = create_async_engine(
            self.db_url,
            echo=self.echo,
            pool_pre_ping=True,
            pool_recycle=3600,
        )
        self.session_factory = async_sessionmaker(
            self.engine,
            expire_on_commit=False,
            class_=AsyncSession,
        )
        self.logger.info("Database connection pool and session factory initialized.")

    async def disconnect(self) -> None:
        """Disposes of the database engine."""
        if self.engine:
            self.logger.info("Closing database connections...")
            await self.engine.dispose()
            self.engine = None
            self.session_factory = None
            self.logger.info("Database connections closed.")

    @contextlib.asynccontextmanager
    async def get_session(self) -> AsyncGenerator[AsyncSession, None]:
        """Provides a database session within a context manager."""
        if self.session_factory is None:
            raise RuntimeError("Database is not connected. Call connect() first.")

        session: AsyncSession = self.session_factory()
        try:
            yield session
            await session.commit()
        except SQLAlchemyError:
            self.logger.exception("Database error occurred, rolling back session.")
            await session.rollback()
            raise
        finally:
            await session.close()
