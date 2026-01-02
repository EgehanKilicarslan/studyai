from unittest.mock import AsyncMock, Mock, patch

import pytest
from app.services.vector_store import VectorStore


@pytest.fixture
def mock_settings():
    """Fixture providing mock settings for VectorStore."""
    settings = Mock()
    settings.qdrant_host = "vector_db"
    settings.qdrant_port = 6333
    settings.qdrant_collection_name = "test_collection"
    return settings


@pytest.fixture
def mock_logger():
    """Fixture providing mock logger."""
    logger = Mock()
    logger.get_logger.return_value = Mock()
    return logger


@pytest.fixture
def mock_embedding_generator():
    """Fixture providing mock embedding generator with vector size."""
    gen = Mock()
    gen.vector_size = 768
    return gen


@pytest.mark.asyncio
async def test_initialization_creates_collection(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test that VectorStore creates a new collection during initialization when it doesn't exist."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient"),
        patch("app.services.vector_store.QdrantClient") as MockSyncClient,
    ):
        sync_instance = MockSyncClient.return_value
        sync_instance.collection_exists.return_value = False

        VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        sync_instance.create_collection.assert_called_once()
        assert sync_instance.create_collection.call_args[1]["collection_name"] == "test_collection"


@pytest.mark.asyncio
async def test_initialization_skips_collection_creation_if_exists(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test that VectorStore does not create a collection if it already exists."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient"),
        patch("app.services.vector_store.QdrantClient") as MockSyncClient,
    ):
        sync_instance = MockSyncClient.return_value
        sync_instance.collection_exists.return_value = True

        VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        sync_instance.create_collection.assert_not_called()


@pytest.mark.asyncio
async def test_upsert_vectors_with_chunk_ids(mock_settings, mock_logger, mock_embedding_generator):
    """Test that vectors are correctly upserted with chunk IDs and metadata."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        async_client_instance.upsert = AsyncMock()

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        vectors = [[0.1, 0.2]]
        chunk_ids = ["chunk-1"]
        document_id = "doc-123"
        filename = "test.txt"

        count = await store.upsert_vectors_with_chunk_ids(vectors, chunk_ids, document_id, filename)

        assert count == 1
        async_client_instance.upsert.assert_called_once()

        call_args = async_client_instance.upsert.call_args
        points = call_args[1]["points"]
        assert points[0].payload["chunk_id"] == "chunk-1"
        assert points[0].payload["document_id"] == "doc-123"
        assert points[0].payload["filename"] == "test.txt"


@pytest.mark.asyncio
async def test_upsert_multiple_vectors_with_chunk_ids(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test that multiple vectors can be upserted in a single operation."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        async_client_instance.upsert = AsyncMock()

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        vectors = [[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]
        chunk_ids = ["chunk-1", "chunk-2", "chunk-3"]
        document_id = "doc-123"
        filename = "test.txt"

        count = await store.upsert_vectors_with_chunk_ids(vectors, chunk_ids, document_id, filename)

        assert count == 3
        async_client_instance.upsert.assert_called_once()


@pytest.mark.asyncio
async def test_upsert_vectors_empty_list(mock_settings, mock_logger, mock_embedding_generator):
    """Test that upserting an empty list returns zero count."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        async_client_instance.upsert = AsyncMock()

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        count = await store.upsert_vectors_with_chunk_ids([], [], "doc-123", "test.txt")

        assert count == 0
        async_client_instance.upsert.assert_not_called()


@pytest.mark.asyncio
async def test_search_vectors(mock_settings, mock_logger, mock_embedding_generator):
    """Test that vector search returns correct results with specified limit."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        mock_result = Mock()
        mock_result.points = ["hit1", "hit2"]
        async_client_instance.query_points = AsyncMock(return_value=mock_result)

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        results = await store.search([0.1, 0.2], limit=5)

        assert len(results) == 2
        async_client_instance.query_points.assert_called_once()


@pytest.mark.asyncio
async def test_search_vectors_with_default_limit(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test that vector search uses default limit when not specified."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        mock_result = Mock()
        mock_result.points = []
        async_client_instance.query_points = AsyncMock(return_value=mock_result)

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        results = await store.search([0.1, 0.2])

        assert len(results) == 0
        async_client_instance.query_points.assert_called_once()


@pytest.mark.asyncio
async def test_search_vectors_no_results(mock_settings, mock_logger, mock_embedding_generator):
    """Test that search returns empty list when no vectors match the query."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        mock_result = Mock()
        mock_result.points = []
        async_client_instance.query_points = AsyncMock(return_value=mock_result)

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        results = await store.search([0.1, 0.2], limit=10)

        assert len(results) == 0
        assert results == []


@pytest.mark.asyncio
async def test_delete_by_document_id(mock_settings, mock_logger, mock_embedding_generator):
    """Test that vectors can be deleted by document ID."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        async_client_instance.delete = AsyncMock()

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        await store.delete_by_document_id("doc-123")

        async_client_instance.delete.assert_called_once()
        call_args = async_client_instance.delete.call_args
        # Collection name comes from mock_settings.qdrant_collection_name
        assert call_args[1]["collection_name"] == mock_settings.qdrant_collection_name


@pytest.mark.asyncio
async def test_upsert_vectors_with_metadata(mock_settings, mock_logger, mock_embedding_generator):
    """Test that vectors are correctly upserted with multi-tenant metadata."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        async_client_instance.upsert = AsyncMock()

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        vectors = [[0.1, 0.2], [0.3, 0.4]]
        chunk_ids = ["chunk-1", "chunk-2"]
        document_id = "doc-123"
        filename = "test.pdf"
        organization_id = 1
        group_id = 10
        owner_id = 100

        count = await store.upsert_vectors_with_metadata(
            vectors=vectors,
            chunk_ids=chunk_ids,
            document_id=document_id,
            filename=filename,
            organization_id=organization_id,
            group_id=group_id,
            owner_id=owner_id,
        )

        assert count == 2
        async_client_instance.upsert.assert_called_once()

        call_args = async_client_instance.upsert.call_args
        points = call_args[1]["points"]

        # Verify first point has correct metadata
        assert points[0].payload["chunk_id"] == "chunk-1"
        assert points[0].payload["document_id"] == "doc-123"
        assert points[0].payload["filename"] == "test.pdf"
        assert points[0].payload["organization_id"] == 1
        assert points[0].payload["group_id"] == 10
        assert points[0].payload["owner_id"] == 100

        # Verify second point
        assert points[1].payload["chunk_id"] == "chunk-2"
        assert points[1].payload["organization_id"] == 1


@pytest.mark.asyncio
async def test_upsert_vectors_with_metadata_org_wide(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test upserting vectors for org-wide document (group_id is None)."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        async_client_instance.upsert = AsyncMock()

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        vectors = [[0.1, 0.2]]
        chunk_ids = ["chunk-1"]

        count = await store.upsert_vectors_with_metadata(
            vectors=vectors,
            chunk_ids=chunk_ids,
            document_id="doc-org-wide",
            filename="org-doc.pdf",
            organization_id=1,
            group_id=None,  # Org-wide document
            owner_id=100,
        )

        assert count == 1
        call_args = async_client_instance.upsert.call_args
        points = call_args[1]["points"]

        # Verify group_id is stored as None for org-wide documents
        assert points[0].payload["group_id"] is None
        assert points[0].payload["organization_id"] == 1


@pytest.mark.asyncio
async def test_upsert_vectors_with_metadata_empty_list(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test that upserting an empty list returns zero count."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        async_client_instance.upsert = AsyncMock()

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        count = await store.upsert_vectors_with_metadata(
            vectors=[],
            chunk_ids=[],
            document_id="doc-123",
            filename="test.pdf",
            organization_id=1,
            group_id=None,
            owner_id=100,
        )

        assert count == 0
        async_client_instance.upsert.assert_not_called()


# ==================== Tenant-Scoped Search Tests ====================


@pytest.mark.asyncio
async def test_search_with_tenant_filter_org_and_groups(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test tenant-scoped search with organization ID and group IDs."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        mock_result = Mock()
        mock_result.points = ["hit1", "hit2"]
        async_client_instance.query_points = AsyncMock(return_value=mock_result)

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        results = await store.search_with_tenant_filter(
            query_vector=[0.1, 0.2],
            organization_id=1,
            group_ids=[10, 20, 30],
            limit=5,
        )

        assert len(results) == 2
        async_client_instance.query_points.assert_called_once()

        # Verify the filter was passed
        call_kwargs = async_client_instance.query_points.call_args[1]
        assert "query_filter" in call_kwargs
        assert call_kwargs["limit"] == 5


@pytest.mark.asyncio
async def test_search_with_tenant_filter_org_only_no_groups(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test tenant-scoped search with organization ID but no group membership."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        mock_result = Mock()
        mock_result.points = ["org-wide-hit"]
        async_client_instance.query_points = AsyncMock(return_value=mock_result)

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        # User has no group memberships, should only see org-wide documents
        results = await store.search_with_tenant_filter(
            query_vector=[0.1, 0.2],
            organization_id=1,
            group_ids=None,  # No groups
            limit=10,
        )

        assert len(results) == 1
        async_client_instance.query_points.assert_called_once()

        # Verify filter includes organization and null group filter
        call_kwargs = async_client_instance.query_points.call_args[1]
        assert "query_filter" in call_kwargs


@pytest.mark.asyncio
async def test_search_with_tenant_filter_empty_groups(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test tenant-scoped search with empty group list (same as no groups)."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        mock_result = Mock()
        mock_result.points = []
        async_client_instance.query_points = AsyncMock(return_value=mock_result)

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        results = await store.search_with_tenant_filter(
            query_vector=[0.1, 0.2],
            organization_id=1,
            group_ids=[],  # Empty list, treated same as None
            limit=25,
        )

        assert len(results) == 0
        async_client_instance.query_points.assert_called_once()


@pytest.mark.asyncio
async def test_search_with_tenant_filter_default_limit(
    mock_settings, mock_logger, mock_embedding_generator
):
    """Test tenant-scoped search uses default limit of 25."""
    with (
        patch("app.services.vector_store.AsyncQdrantClient") as MockAsyncClient,
        patch("app.services.vector_store.QdrantClient"),
    ):
        async_client_instance = MockAsyncClient.return_value
        mock_result = Mock()
        mock_result.points = []
        async_client_instance.query_points = AsyncMock(return_value=mock_result)

        store = VectorStore(mock_settings, mock_logger, mock_embedding_generator)

        await store.search_with_tenant_filter(
            query_vector=[0.1, 0.2],
            organization_id=1,
            group_ids=[10],
        )

        call_kwargs = async_client_instance.query_points.call_args[1]
        assert call_kwargs["limit"] == 25
