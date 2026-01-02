from typing import Optional

from pydantic import Field, model_validator
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application configuration settings."""

    app_env: str = Field(default="development", pattern="^(development|production)$")
    log_level: str = Field(default="INFO", pattern="^(DEBUG|INFO|WARNING|ERROR|CRITICAL)$")

    ai_service_port: int = Field(default=50051)

    postgresql_host: str = Field(default="db")
    postgresql_port: int = Field(default=5432)
    postgresql_user: str = Field(default="studyai_user")
    postgresql_password: str = Field(default="studyai_password")
    postgresql_database: str = Field(default="studyai_db")

    redis_host: str = Field(default="redis")
    redis_port: int = Field(default=6379)
    redis_database: int = Field(default=0)

    llm_provider: str = Field(default="dummy", pattern="^(openai|gemini|anthropic|dummy)$")
    llm_base_url: Optional[str] = Field(default=None)
    llm_api_key: Optional[str] = Field(default=None)
    llm_model_name: str = Field(default="local")
    llm_timeout: float = Field(default=60.0)

    qdrant_host: str = Field(default="vector-db")
    qdrant_port: int = Field(default=6333)
    qdrant_collection_name: str = Field(default="school_docs")

    embedding_model_name: str = Field(default="BAAI/bge-small-en-v1.5")
    embedding_chunk_size: int = Field(default=1000)
    embedding_chunk_overlap: int = Field(default=200)

    reranker_model_name: str = Field(default="ms-marco-MiniLM-L-12-v2")

    maximum_file_size: int = Field(default=50 * 1024 * 1024)  # 50 MB

    @model_validator(mode="after")
    def validate_provider(self) -> "Settings":
        """Validate and normalize the LLM provider"""
        v = self.llm_provider.lower()

        # API key checks
        if v in {"openai", "gemini", "anthropic"} and not self.llm_api_key:
            raise ValueError(f"{v.capitalize()} selected but LLM_API_KEY is missing")

        self.llm_provider = v
        return self

    class ConfigDict:
        env_file = ".env"
        env_file_encoding = "utf-8"

    @property
    def database_url(self) -> str:
        """Construct the async database URL from components."""
        return (
            f"postgresql+asyncpg://{self.postgresql_user}:"
            f"{self.postgresql_password}@"
            f"{self.postgresql_host}:"
            f"{self.postgresql_port}/"
            f"{self.postgresql_database}"
        )

    @property
    def sync_database_url(self) -> str:
        """Construct the sync database URL for Celery tasks."""
        return (
            f"postgresql+psycopg2://{self.postgresql_user}:"
            f"{self.postgresql_password}@"
            f"{self.postgresql_host}:"
            f"{self.postgresql_port}/"
            f"{self.postgresql_database}"
        )

    @property
    def celery_broker_url(self) -> str:
        """Construct the Celery broker URL (Redis)."""
        return f"redis://{self.redis_host}:{self.redis_port}/{self.redis_database}"

    @property
    def celery_result_backend(self) -> str:
        """Construct the Celery result backend URL (Redis)."""
        return f"redis://{self.redis_host}:{self.redis_port}/{self.redis_database}"


settings = Settings()
