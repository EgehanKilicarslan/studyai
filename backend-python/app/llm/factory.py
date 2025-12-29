from config import Settings
from llm.base import LLMProvider
from llm.provider import (
    AnthropicProvider,
    DummyProvider,
    GeminiProvider,
    OpenAIProvider,
)
from logger import AppLogger


def get_llm_provider(settings: Settings, logger: AppLogger) -> LLMProvider:
    _logger = logger.get_logger(__name__)

    provider = settings.llm_provider
    _logger.info(f"🧠 [Factory] Selected LLM Provider: {provider}")

    # Map provider names to their classes
    provider_map = {
        "openai": OpenAIProvider,
        "gemini": GeminiProvider,
        "anthropic": AnthropicProvider,
    }

    if provider in provider_map:
        assert settings.llm_api_key is not None

        return provider_map[provider](
            base_url=settings.llm_base_url,
            api_key=settings.llm_api_key,
            model=settings.llm_model_name,
            timeout=settings.llm_timeout,
        )

    return DummyProvider()
