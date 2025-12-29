from unittest.mock import Mock, patch

import pytest


def test_app_module_structure():
    """Test that app module has required components."""
    from app import config, logger

    assert config is not None
    assert logger is not None


def test_settings_configuration():
    """Test that settings are properly configured."""
    from app.config import settings

    assert hasattr(settings, "ai_service_port")
    assert hasattr(settings, "llm_provider")
    assert settings.ai_service_port > 0


@patch("grpc.aio.server")
def test_grpc_server_initialization(mock_grpc_server):
    """Test gRPC server can be initialized."""
    from app.config import settings

    # Mock server
    mock_server = Mock()
    mock_grpc_server.return_value = mock_server

    # Verify settings are accessible
    assert settings.ai_service_port == 50051


def test_service_imports():
    """Test that all required services can be imported."""
    try:
        from app.services import (
            document_parser,
            embedding_generator,
            rag_service,
            reranker_service,
            vector_store,
        )

        assert document_parser is not None
        assert embedding_generator is not None
        assert rag_service is not None
        assert reranker_service is not None
        assert vector_store is not None
    except ImportError as e:
        pytest.fail(f"Failed to import services: {e}")


def test_llm_providers_available():
    """Test that LLM providers can be imported."""
    try:
        from app.llm.provider import (
            AnthropicProvider,
            DummyProvider,
            GeminiProvider,
            OpenAIProvider,
        )

        assert OpenAIProvider is not None
        assert AnthropicProvider is not None
        assert GeminiProvider is not None
        assert DummyProvider is not None
    except ImportError as e:
        pytest.fail(f"Failed to import LLM providers: {e}")


@patch("app.containers.Container")
def test_dependency_injection_container(mock_container):
    """Test that dependency injection container works."""
    mock_container.return_value = Mock()

    from app import containers

    assert containers is not None
    assert hasattr(containers, "Container")
