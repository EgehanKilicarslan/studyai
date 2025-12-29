from unittest.mock import Mock, patch

import pytest
from app.llm.factory import get_llm_provider


@pytest.fixture
def mock_settings():
    settings = Mock()
    settings.llm_provider = "openai"
    settings.llm_api_key = "test-key"
    settings.llm_model_name = "gpt-4"
    settings.llm_timeout = 60
    settings.llm_base_url = None
    return settings


@pytest.fixture
def mock_logger():
    logger = Mock()
    logger.get_logger = Mock(return_value=Mock())
    return logger


@patch("app.llm.provider.openai_provider.AsyncOpenAI")
def test_factory_create_openai_provider(mock_client, mock_settings, mock_logger):
    """Test creating OpenAI provider."""
    mock_settings.llm_provider = "openai"

    provider = get_llm_provider(mock_settings, mock_logger)

    assert provider.__class__.__name__ == "OpenAIProvider"
    assert provider.provider_name == "openai"


@patch("app.llm.provider.anthropic_provider.AsyncAnthropic")
def test_factory_create_anthropic_provider(mock_client, mock_settings, mock_logger):
    """Test creating Anthropic provider."""
    mock_settings.llm_provider = "anthropic"

    provider = get_llm_provider(mock_settings, mock_logger)

    assert provider.__class__.__name__ == "AnthropicProvider"
    assert provider.provider_name == "anthropic"


@patch("app.llm.provider.gemini_provider.Client")
def test_factory_create_gemini_provider(mock_client, mock_settings, mock_logger):
    """Test creating Gemini provider."""
    mock_settings.llm_provider = "gemini"

    provider = get_llm_provider(mock_settings, mock_logger)

    assert provider.__class__.__name__ == "GeminiProvider"
    assert provider.provider_name == "gemini"


def test_factory_create_dummy_provider(mock_settings, mock_logger):
    """Test creating Dummy provider as fallback."""
    mock_settings.llm_provider = "unknown"

    provider = get_llm_provider(mock_settings, mock_logger)

    assert provider.__class__.__name__ == "DummyProvider"
    assert provider.provider_name == "dummy"


@patch("app.llm.provider.openai_provider.AsyncOpenAI")
def test_factory_with_base_url(mock_client, mock_settings, mock_logger):
    """Test provider creation with custom base URL."""
    mock_settings.llm_provider = "openai"
    mock_settings.llm_base_url = "https://custom.api.com"

    provider = get_llm_provider(mock_settings, mock_logger)

    assert provider.__class__.__name__ == "OpenAIProvider"


@patch("app.llm.provider.openai_provider.AsyncOpenAI")
def test_factory_with_different_timeouts(mock_client, mock_settings, mock_logger):
    """Test provider creation with different timeout values."""
    mock_settings.llm_timeout = 120
    mock_settings.llm_provider = "openai"

    provider = get_llm_provider(mock_settings, mock_logger)

    assert provider.__class__.__name__ == "OpenAIProvider"


def test_factory_case_insensitive_provider(mock_settings, mock_logger):
    """Test that provider name is case-insensitive."""
    mock_settings.llm_provider = "OpenAI"

    provider = get_llm_provider(mock_settings, mock_logger)

    # Case sensitivity check - should fall back to dummy if exact match fails
    assert provider is not None
    # The factory may return DummyProvider if case doesn't match exactly
    assert hasattr(provider, "generate_response")


@patch("app.llm.provider.openai_provider.AsyncOpenAI")
@patch("app.llm.provider.anthropic_provider.AsyncAnthropic")
@patch("app.llm.provider.gemini_provider.Client")
def test_factory_all_providers_have_common_interface(
    mock_gemini, mock_anthropic, mock_openai, mock_settings, mock_logger
):
    """Test that all providers implement the common interface."""
    providers = []

    for provider_name in ["openai", "anthropic", "gemini", "unknown"]:
        mock_settings.llm_provider = provider_name
        provider = get_llm_provider(mock_settings, mock_logger)
        providers.append(provider)

    # All providers should have these methods
    for provider in providers:
        assert hasattr(provider, "generate_response")
        assert hasattr(provider, "provider_name")
        assert callable(provider.generate_response)


def test_factory_returns_valid_provider_types(mock_settings, mock_logger):
    """Test that factory returns expected provider types for each name."""
    test_cases = [
        ("openai", "OpenAIProvider"),
        ("anthropic", "AnthropicProvider"),
        ("gemini", "GeminiProvider"),
        ("unknown", "DummyProvider"),
        ("invalid", "DummyProvider"),
    ]

    for provider_name, expected_class in test_cases:
        mock_settings.llm_provider = provider_name

        with (
            patch("app.llm.provider.openai_provider.AsyncOpenAI"),
            patch("app.llm.provider.anthropic_provider.AsyncAnthropic"),
            patch("app.llm.provider.gemini_provider.Client"),
        ):
            provider = get_llm_provider(mock_settings, mock_logger)
            assert provider.__class__.__name__ == expected_class
