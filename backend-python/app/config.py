from typing import Optional

from pydantic import Field, model_validator
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    python_port: int = Field(default=50051)

    llm_provider: str = Field(default="dummy")

    model_name: str = Field(default="local")

    llm_timeout: float = Field(default=60.0)

    openai_api_key: Optional[str] = Field(default=None)
    gemini_api_key: Optional[str] = Field(default=None)

    local_llm_url: str = Field(default="http://localhost:8080/v1/chat/completions")

    qdrant_host: str = Field(default="localhost")
    qdrant_port: int = Field(default=6333)

    embedding_vector_size: int = Field(default=384)

    @model_validator(mode="after")
    def validate_provider(self) -> "Settings":
        """Validate and normalize the LLM provider"""
        v = self.llm_provider.lower()
        valid_providers = ["openai", "gemini", "local", "dummy"]

        if v not in valid_providers:
            raise ValueError(
                f"Invalid LLM provider. Valid options are: {', '.join(valid_providers)}"
            )

        # API key checks
        if v == "openai" and not self.openai_api_key:
            raise ValueError("OpenAI selected but OPENAI_API_KEY is missing")
        if v == "gemini" and not self.gemini_api_key:
            raise ValueError("Gemini selected but GEMINI_API_KEY is missing")

        self.llm_provider = v
        return self

    class ConfigDict:
        env_file = ".env"
        env_file_encoding = "utf-8"


settings = Settings()
