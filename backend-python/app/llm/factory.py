from app.config import Settings

from .base import LLMProvider
from .provider import (
    DummyProvider,
    GeminiProvider,
    LocalProvider,
    OpenAIProvider,
)


def get_llm_provider(settings: Settings) -> LLMProvider:
    provider = settings.llm_provider
    print(f"🧠 [Factory] Selected LLM Provider: {provider}")

    if provider == "openai":
        assert settings.openai_api_key is not None
        return OpenAIProvider(
            api_key=settings.openai_api_key, model=settings.model_name, timeout=settings.llm_timeout
        )

    elif provider == "gemini":
        assert settings.gemini_api_key is not None
        return GeminiProvider(
            api_key=settings.gemini_api_key, model=settings.model_name, timeout=settings.llm_timeout
        )
    elif provider == "local":
        return LocalProvider(
            base_url=settings.local_llm_url, model=settings.model_name, timeout=settings.llm_timeout
        )

    else:
        return DummyProvider()
