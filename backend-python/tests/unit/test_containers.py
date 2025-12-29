from unittest.mock import patch

from app.containers import Container


def test_container_initialization():
    """Test container can be initialized."""
    container = Container()
    assert container is not None


def test_container_has_config():
    """Test container has config provider."""
    container = Container()
    assert hasattr(container, "config")


def test_container_has_logger():
    """Test container has logger provider."""
    container = Container()
    assert hasattr(container, "app_logger")


def test_container_has_services():
    """Test container has all required service providers."""
    container = Container()
    assert hasattr(container, "llm_client")
    assert hasattr(container, "document_parser")
    assert hasattr(container, "embedding_generator")
    assert hasattr(container, "reranker_service")
    assert hasattr(container, "vector_store")
    assert hasattr(container, "rag_service")


def test_container_logger_initialization():
    """Test logger provider creates valid logger."""
    container = Container()
    logger = container.app_logger()
    assert logger is not None


@patch("app.llm.provider.openai_provider.AsyncOpenAI")
def test_container_llm_provider(mock_openai):
    """Test LLM provider is accessible."""
    container = Container()
    llm = container.llm_client()
    assert llm is not None


def test_container_vector_store_provider():
    """Test vector store provider is defined and is a Factory."""
    from dependency_injector import providers

    container = Container()

    # Verify the provider exists and is the correct type
    assert hasattr(container, "vector_store")
    assert isinstance(container.vector_store, providers.Factory)


def test_container_rag_service_provider():
    """Test RAG service provider is defined and is a Factory."""
    from dependency_injector import providers

    container = Container()

    # Verify the provider exists and is the correct type
    assert hasattr(container, "rag_service")
    assert isinstance(container.rag_service, providers.Factory)
