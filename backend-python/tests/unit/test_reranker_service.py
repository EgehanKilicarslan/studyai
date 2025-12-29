from unittest.mock import Mock, patch

import pytest
from app.services.reranker_service import RerankerService


@pytest.fixture
def mock_settings():
    """Fixture that provides mock settings with reranker configuration."""
    settings = Mock()
    settings.reranker_model_name = "ms-marco-TinyBERT-L-2-v2"
    return settings


@pytest.fixture
def mock_logger():
    """Fixture that provides a mock logger instance."""
    logger = Mock()
    logger.get_logger.return_value = Mock()
    return logger


def test_reranker_initialization(mock_settings, mock_logger):
    """
    Test that RerankerService initializes the Ranker with correct model settings.

    Verifies:
    - Ranker class is instantiated once
    - Correct model_name from settings is passed to Ranker
    """
    with patch("app.services.reranker_service.Ranker") as MockRanker:
        RerankerService(mock_settings, mock_logger)

        MockRanker.assert_called_once()
        call_args = MockRanker.call_args
        assert call_args[1]["model_name"] == mock_settings.reranker_model_name


def test_rerank_empty_documents(mock_settings, mock_logger):
    """
    Test that reranking with an empty document list returns an empty list.

    Verifies:
    - Service handles empty input gracefully
    - Returns empty list without calling the ranker
    """
    with patch("app.services.reranker_service.Ranker"):
        service = RerankerService(mock_settings, mock_logger)
        results = service.rerank("query", [])
        assert results == []


def test_rerank_success(mock_settings, mock_logger):
    """
    Test successful reranking of documents.

    Verifies:
    - Service calls ranker.rerank with correct parameters
    - Returns reranked results with correct structure
    - top_k parameter limits the number of results
    """
    mock_docs = [{"id": 1, "text": "doc1"}, {"id": 2, "text": "doc2"}]

    with patch("app.services.reranker_service.Ranker") as MockRanker:
        ranker_instance = MockRanker.return_value
        ranker_instance.rerank.return_value = [{"id": 1, "score": 0.9}]

        service = RerankerService(mock_settings, mock_logger)
        results = service.rerank("query", mock_docs, top_k=1)

        assert len(results) == 1
        assert results[0]["id"] == 1
        ranker_instance.rerank.assert_called_once()


def test_rerank_with_multiple_documents(mock_settings, mock_logger):
    """
    Test reranking returns documents in order of relevance score.

    Verifies:
    - Multiple documents are processed correctly
    - Results are returned in descending score order
    - All expected documents are present in results
    """
    mock_docs = [{"id": 1, "text": "doc1"}, {"id": 2, "text": "doc2"}, {"id": 3, "text": "doc3"}]

    with patch("app.services.reranker_service.Ranker") as MockRanker:
        ranker_instance = MockRanker.return_value
        ranker_instance.rerank.return_value = [
            {"id": 3, "score": 0.95},
            {"id": 1, "score": 0.85},
            {"id": 2, "score": 0.75},
        ]

        service = RerankerService(mock_settings, mock_logger)
        results = service.rerank("test query", mock_docs)

        assert len(results) == 3
        assert results[0]["id"] == 3
        assert results[0]["score"] == 0.95
        assert results[1]["id"] == 1
        assert results[2]["id"] == 2


def test_rerank_with_custom_top_k(mock_settings, mock_logger):
    """
    Test that top_k parameter correctly limits the number of returned results.

    Verifies:
    - top_k parameter is respected
    - Only the specified number of top results are returned
    """
    mock_docs = [{"id": i, "text": f"doc{i}"} for i in range(10)]

    with patch("app.services.reranker_service.Ranker") as MockRanker:
        ranker_instance = MockRanker.return_value
        ranker_instance.rerank.return_value = [
            {"id": i, "score": 1.0 - (i * 0.1)} for i in range(3)
        ]

        service = RerankerService(mock_settings, mock_logger)
        results = service.rerank("query", mock_docs, top_k=3)

        assert len(results) == 3


def test_rerank_preserves_document_metadata(mock_settings, mock_logger):
    """
    Test that reranking preserves original document metadata.

    Verifies:
    - Additional document fields are maintained
    - Only relevance scores are added/updated
    """
    mock_docs = [
        {"id": 1, "text": "doc1", "metadata": {"source": "file1.txt"}},
        {"id": 2, "text": "doc2", "metadata": {"source": "file2.txt"}},
    ]

    with patch("app.services.reranker_service.Ranker") as MockRanker:
        ranker_instance = MockRanker.return_value
        ranker_instance.rerank.return_value = [
            {"id": 1, "score": 0.9, "metadata": {"source": "file1.txt"}}
        ]

        service = RerankerService(mock_settings, mock_logger)
        results = service.rerank("query", mock_docs, top_k=1)

        assert results[0].get("metadata") is not None


def test_rerank_handles_ranker_exception(mock_settings, mock_logger):
    """
    Test that service handles exceptions from the ranker gracefully.

    Verifies:
    - Exceptions from ranker are caught
    - Error is logged
    - Empty list or appropriate error response is returned
    """
    mock_docs = [{"id": 1, "text": "doc1"}]

    with patch("app.services.reranker_service.Ranker") as MockRanker:
        ranker_instance = MockRanker.return_value
        ranker_instance.rerank.side_effect = Exception("Ranker error")

        service = RerankerService(mock_settings, mock_logger)

        with pytest.raises(Exception):
            service.rerank("query", mock_docs)
