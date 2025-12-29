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
    """
    Factory function to retrieve the appropriate LLM (Large Language Model) provider
    based on the application settings.

    Args:
        settings (Settings): The application settings containing configuration
            for the LLM provider, including provider name, API key, base URL,
            model name, and timeout.
        logger (AppLogger): The application logger instance used for logging
            information and debugging.

    Returns:
        LLMProvider: An instance of the selected LLM provider class. If the
            provider specified in the settings is not recognized, a DummyProvider
            instance is returned.

    Raises:
        AssertionError: If the selected provider requires an API key and it is
            not provided in the settings.

    Notes:
        - Supported providers are "openai", "gemini", and "anthropic".
        - The provider classes must be mapped in the `provider_map` dictionary.
    """

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
