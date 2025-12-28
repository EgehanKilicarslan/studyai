from app.config import Settings

from .base import LLMProvider
from .provider import (
    DummyProvider,
    GeminiProvider,
    OpenAIProvider,
)


def get_llm_provider(settings: Settings) -> LLMProvider:
    provider = settings.llm_provider
    print(f"🧠 [Factory] Selected LLM Provider: {provider}")

    if provider in ["openai", "gemini"]:
        assert settings.llm_api_key is not None
        provider_class = OpenAIProvider if provider == "openai" else GeminiProvider
        return provider_class(
            base_url=settings.llm_base_url,
            api_key=settings.llm_api_key,
            model=settings.model_name,
            timeout=settings.llm_timeout,
        )

    else:
        return DummyProvider()
