from unittest.mock import Mock, patch

import numpy as np
import pytest
from app.services.embedding_generator import EmbeddingGenerator


@pytest.fixture
def mock_settings():
    settings = Mock()
    settings.embedding_model_name = "BAAI/bge-small-en-v1.5"
    return settings


@pytest.fixture
def mock_logger():
    logger = Mock()
    logger.get_logger.return_value = Mock()
    return logger


@pytest.fixture
def mock_fastembed():
    with patch("app.services.embedding_generator.TextEmbedding") as MockClass:
        mock_instance = MockClass.return_value

        def mock_embed(documents):
            for _ in documents:
                yield np.array([0.1, 0.2, 0.3])

        mock_instance.embed.side_effect = mock_embed
        yield MockClass


def test_initialization(mock_settings, mock_logger, mock_fastembed):
    """Test that EmbeddingGenerator initializes correctly with proper model loading and vector size detection."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    mock_fastembed.assert_called_once()
    assert generator.vector_size == 3
    assert mock_fastembed.return_value.embed.called


def test_generate_sync(mock_settings, mock_logger, mock_fastembed):
    """Test synchronous embedding generation returns correct number and format of embeddings."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    docs = ["text1", "text2"]
    embeddings = generator.generate_sync(docs)

    assert len(embeddings) == 2
    assert len(embeddings[0]) == 3
    assert isinstance(embeddings[0], list)


@pytest.mark.asyncio
async def test_generate_async(mock_settings, mock_logger, mock_fastembed):
    """Test asynchronous embedding generation returns correct number and format of embeddings."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    docs = ["text1"]
    embeddings = await generator.generate(docs)

    assert len(embeddings) == 1
    assert len(embeddings[0]) == 3


def test_generate_sync_empty_input(mock_settings, mock_logger, mock_fastembed):
    """Test that synchronous generation handles empty document list correctly."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    docs = []
    embeddings = generator.generate_sync(docs)

    assert len(embeddings) == 0
    assert isinstance(embeddings, list)


@pytest.mark.asyncio
async def test_generate_async_empty_input(mock_settings, mock_logger, mock_fastembed):
    """Test that asynchronous generation handles empty document list correctly."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    docs = []
    embeddings = await generator.generate(docs)

    assert len(embeddings) == 0
    assert isinstance(embeddings, list)


def test_generate_sync_single_document(mock_settings, mock_logger, mock_fastembed):
    """Test synchronous embedding generation with a single document."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    docs = ["single document"]
    embeddings = generator.generate_sync(docs)

    assert len(embeddings) == 1
    assert len(embeddings[0]) == 3
    assert all(isinstance(val, float) for val in embeddings[0])


@pytest.mark.asyncio
async def test_generate_async_multiple_documents(mock_settings, mock_logger, mock_fastembed):
    """Test asynchronous embedding generation with multiple documents."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    docs = ["doc1", "doc2", "doc3"]
    embeddings = await generator.generate(docs)

    assert len(embeddings) == 3
    assert all(len(emb) == 3 for emb in embeddings)


def test_vector_size_determination(mock_settings, mock_logger, mock_fastembed):
    """Test that vector size is correctly determined from the first generated embedding."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    assert generator.vector_size == 3


def test_embedding_values_are_numeric(mock_settings, mock_logger, mock_fastembed):
    """Test that generated embeddings contain numeric values."""
    generator = EmbeddingGenerator(mock_settings, mock_logger)

    docs = ["test document"]
    embeddings = generator.generate_sync(docs)

    assert all(isinstance(val, (int, float)) for val in embeddings[0])
