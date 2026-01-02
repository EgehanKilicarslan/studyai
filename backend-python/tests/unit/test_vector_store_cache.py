"""Unit tests for the VectorStore semantic cache methods."""

import uuid
from unittest.mock import AsyncMock, Mock, patch

import pytest
from app.services.vector_store import CacheHit, VectorStore
from qdrant_client import models

# Default test cache collection name
TEST_CACHE_COLLECTION = "semantic_cache"


@pytest.fixture
def mock_settings():
    """Create mock settings."""
    settings = Mock()
    settings.qdrant_host = "vector_db"
    settings.qdrant_port = 6333
    settings.qdrant_docs_collection_name = "test_collection"
    settings.qdrant_cache_collection_name = TEST_CACHE_COLLECTION
    return settings


@pytest.fixture
def mock_logger():
    """Create a mock logger."""
    logger = Mock()
    logger.get_logger.return_value = Mock()
    return logger


@pytest.fixture
def mock_embedding_generator():
    """Create a mock embedding generator."""
    gen = Mock()
    gen.vector_size = 768
    return gen


@pytest.fixture
def vector_store(mock_settings, mock_logger, mock_embedding_generator):
    """Create a VectorStore instance with mocked Qdrant clients."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient"),
        patch("app.services.vector_store.QdrantClient") as MockSyncClient,
    ):
        # Setup sync client for collection creation
        sync_instance = MockSyncClient.return_value
        sync_instance.collection_exists.return_value = True

        # Create VectorStore
        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        # Replace async client with a fresh mock for testing
        store.client = AsyncMock()

        yield store


class TestSearchCache:
    """Tests for the search_cache method."""

    @pytest.mark.asyncio
    async def test_search_cache_hit_with_user_id(self, vector_store):
        """Test that search_cache returns a CacheHit when a similar query is found for user."""
        query_vector = [0.1, 0.2, 0.3]
        user_id = 123

        # Mock a cache hit
        mock_point = Mock()
        mock_point.id = "cache-123"
        mock_point.score = 0.98
        mock_point.payload = {
            "user_id": user_id,
            "response_text": "This is a cached response.",
        }

        mock_result = Mock()
        mock_result.points = [mock_point]
        vector_store.client.query_points.return_value = mock_result

        result = await vector_store.search_cache(query_vector, user_id=user_id)

        assert result is not None
        assert isinstance(result, CacheHit)
        assert result.response_text == "This is a cached response."
        assert result.score == 0.98
        assert result.cache_id == "cache-123"

        # Verify correct filter was applied
        vector_store.client.query_points.assert_called_once()
        call_kwargs = vector_store.client.query_points.call_args.kwargs
        assert call_kwargs["collection_name"] == TEST_CACHE_COLLECTION
        assert call_kwargs["query"] == query_vector
        assert call_kwargs["score_threshold"] == 0.95

    @pytest.mark.asyncio
    async def test_search_cache_hit_with_group_ids(self, vector_store):
        """Test that search_cache returns a CacheHit when a similar query is found for groups."""
        query_vector = [0.1, 0.2, 0.3]
        group_ids = [1, 2, 3]

        mock_point = Mock()
        mock_point.id = "cache-456"
        mock_point.score = 0.97
        mock_point.payload = {
            "group_ids": group_ids,
            "response_text": "Group cached response.",
        }

        mock_result = Mock()
        mock_result.points = [mock_point]
        vector_store.client.query_points.return_value = mock_result

        result = await vector_store.search_cache(query_vector, group_ids=group_ids)

        assert result is not None
        assert result.response_text == "Group cached response."

    @pytest.mark.asyncio
    async def test_search_cache_miss(self, vector_store):
        """Test that search_cache returns None when no similar query is found."""
        query_vector = [0.1, 0.2, 0.3]
        user_id = 123

        # Mock a cache miss
        mock_result = Mock()
        mock_result.points = []
        vector_store.client.query_points.return_value = mock_result

        result = await vector_store.search_cache(query_vector, user_id=user_id)

        assert result is None

    @pytest.mark.asyncio
    async def test_search_cache_no_context_returns_none(self, vector_store):
        """Test that search_cache returns None when no user_id or group_ids provided."""
        query_vector = [0.1, 0.2, 0.3]

        result = await vector_store.search_cache(query_vector)

        assert result is None
        # Should not call query_points at all
        vector_store.client.query_points.assert_not_called()

    @pytest.mark.asyncio
    async def test_search_cache_custom_threshold(self, vector_store):
        """Test that search_cache respects custom threshold parameter."""
        query_vector = [0.1, 0.2, 0.3]
        user_id = 123

        mock_result = Mock()
        mock_result.points = []
        vector_store.client.query_points.return_value = mock_result

        await vector_store.search_cache(query_vector, user_id=user_id, threshold=0.90)

        call_kwargs = vector_store.client.query_points.call_args.kwargs
        assert call_kwargs["score_threshold"] == 0.90

    @pytest.mark.asyncio
    async def test_search_cache_applies_user_filter(self, vector_store):
        """Test that search_cache applies user filter for isolation."""
        query_vector = [0.1, 0.2, 0.3]
        user_id = 456

        mock_result = Mock()
        mock_result.points = []
        vector_store.client.query_points.return_value = mock_result

        await vector_store.search_cache(query_vector, user_id=user_id)

        call_kwargs = vector_store.client.query_points.call_args.kwargs
        query_filter = call_kwargs["query_filter"]

        # Verify filter structure
        assert isinstance(query_filter, models.Filter)
        assert query_filter.must is not None
        assert isinstance(query_filter.must, list)
        assert len(query_filter.must) == 1
        field_condition = query_filter.must[0]
        assert isinstance(field_condition, models.FieldCondition)
        assert field_condition.key == "user_id"
        assert isinstance(field_condition.match, models.MatchValue)
        assert field_condition.match.value == user_id

    @pytest.mark.asyncio
    async def test_search_cache_handles_error(self, vector_store):
        """Test that search_cache returns None on error (graceful degradation)."""
        query_vector = [0.1, 0.2, 0.3]
        user_id = 123

        # Mock an error
        vector_store.client.query_points.side_effect = Exception("Connection error")

        result = await vector_store.search_cache(query_vector, user_id=user_id)

        # Should return None instead of raising
        assert result is None


class TestSaveToCache:
    """Tests for the save_to_cache method."""

    @pytest.mark.asyncio
    async def test_save_to_cache_with_user_id(self, vector_store):
        """Test that save_to_cache successfully stores a response for a user."""
        query_vector = [0.1, 0.2, 0.3]
        response_text = "This is the answer to your question."
        user_id = 123

        vector_store.client.upsert.return_value = None

        with patch("app.services.vector_store.uuid.uuid4") as mock_uuid:
            mock_uuid.return_value = uuid.UUID("12345678-1234-5678-1234-567812345678")
            cache_id = await vector_store.save_to_cache(
                query_vector, response_text, user_id=user_id
            )

        assert cache_id == "12345678-1234-5678-1234-567812345678"

        # Verify upsert was called correctly
        vector_store.client.upsert.assert_called_once()
        call_kwargs = vector_store.client.upsert.call_args.kwargs
        assert call_kwargs["collection_name"] == TEST_CACHE_COLLECTION

        points = call_kwargs["points"]
        assert len(points) == 1
        point = points[0]
        assert point.vector == query_vector
        assert point.payload["user_id"] == user_id
        assert point.payload["response_text"] == response_text

    @pytest.mark.asyncio
    async def test_save_to_cache_with_group_ids(self, vector_store):
        """Test that save_to_cache successfully stores a response for groups."""
        query_vector = [0.1, 0.2, 0.3]
        response_text = "Group answer."
        group_ids = [1, 2, 3]

        vector_store.client.upsert.return_value = None

        with patch("app.services.vector_store.uuid.uuid4") as mock_uuid:
            mock_uuid.return_value = uuid.UUID("12345678-1234-5678-1234-567812345678")
            cache_id = await vector_store.save_to_cache(
                query_vector, response_text, group_ids=group_ids
            )

        assert cache_id is not None

        call_kwargs = vector_store.client.upsert.call_args.kwargs
        points = call_kwargs["points"]
        point = points[0]
        assert point.payload["group_ids"] == group_ids
        assert point.payload["response_text"] == response_text

    @pytest.mark.asyncio
    async def test_save_to_cache_handles_error(self, vector_store):
        """Test that save_to_cache returns None on error (graceful degradation)."""
        query_vector = [0.1, 0.2, 0.3]
        response_text = "Answer"
        user_id = 123

        # Mock an error
        vector_store.client.upsert.side_effect = Exception("Storage error")

        cache_id = await vector_store.save_to_cache(query_vector, response_text, user_id=user_id)

        # Should return None instead of raising
        assert cache_id is None


class TestTenantIsolation:
    """Tests to verify tenant isolation in cache operations."""

    @pytest.mark.asyncio
    async def test_different_users_get_different_cache_filters(self, vector_store):
        """Test that queries from different users use different filters."""
        mock_result = Mock()
        mock_result.points = []
        vector_store.client.query_points.return_value = mock_result

        query_vector = [0.1, 0.2, 0.3]

        # Query from User 1
        await vector_store.search_cache(query_vector, user_id=1)
        user1_filter = vector_store.client.query_points.call_args.kwargs["query_filter"]

        # Query from User 2
        await vector_store.search_cache(query_vector, user_id=2)
        user2_filter = vector_store.client.query_points.call_args.kwargs["query_filter"]

        # Verify filters target different users
        assert isinstance(user1_filter.must, list)
        assert isinstance(user2_filter.must, list)
        user1_condition = user1_filter.must[0]
        user2_condition = user2_filter.must[0]
        assert isinstance(user1_condition, models.FieldCondition)
        assert isinstance(user2_condition, models.FieldCondition)
        assert isinstance(user1_condition.match, models.MatchValue)
        assert isinstance(user2_condition.match, models.MatchValue)
        assert user1_condition.match.value == 1
        assert user2_condition.match.value == 2
