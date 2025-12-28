from typing import Optional

from pydantic import Field, model_validator
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    python_port: int = Field(default=50051)

    llm_provider: str = Field(default="dummy", pattern="^(openai|gemini|anthropic|dummy)$")
    llm_base_url: Optional[str] = Field(default=None)
    llm_api_key: Optional[str] = Field(default=None)

    model_name: str = Field(default="local")

    llm_timeout: float = Field(default=60.0)

    qdrant_host: str = Field(default="localhost")
    qdrant_port: int = Field(default=6333)
    qdrant_collection: str = Field(default="school_docs")

    embedding_vector_size: int = Field(default=384)
    embedding_chunk_size: int = Field(default=500)
    embedding_chunk_overlap: int = Field(default=50)

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


settings = Settings()
