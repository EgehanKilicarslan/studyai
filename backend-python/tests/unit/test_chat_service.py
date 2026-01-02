from unittest.mock import AsyncMock, MagicMock, Mock

import pytest
from app.services.grpc.chat_service import (
    ChatService,
    get_tenant_context_from_metadata,
    get_user_id_from_context,
)
from pb import rag_service_pb2 as rs


async def async_iter(items):
    """Helper to create an async iterator from a list."""
    for item in items:
        yield item


@pytest.fixture
def mock_logger():
    logger = Mock()
    logger.get_logger.return_value = Mock()
    return logger


@pytest.fixture
def mock_llm():
    llm = Mock()
    llm.generate_response = MagicMock(return_value=async_iter(["Answer"]))
    return llm


@pytest.fixture
def mock_vector_store():
    store = Mock()
    store.search = AsyncMock(return_value=[])
    store.search_with_tenant_filter = AsyncMock(return_value=[])
    return store


@pytest.fixture
def mock_embedder():
    embedder = Mock()
    embedder.generate = AsyncMock(return_value=[[0.1, 0.2]])
    return embedder


@pytest.fixture
def mock_reranker():
    reranker = Mock()
    reranker.rerank = Mock(return_value=[])
    return reranker


@pytest.fixture
def mock_chunk_service():
    chunk_service = Mock()
    chunk_service.get_chunks_by_ids = AsyncMock(return_value=[])
    return chunk_service


@pytest.fixture
def mock_context():
    """Create a mock gRPC context with user ID and tenant metadata."""
    context = Mock()
    context.invocation_metadata.return_value = [
        ("x-user-id", "123"),
        ("x-organization-id", "1"),
        ("x-group-ids", "1,2,3"),
    ]
    return context


@pytest.fixture
def mock_context_no_user():
    """Create a mock gRPC context without user ID."""
    context = Mock()
    context.invocation_metadata.return_value = []
    return context


@pytest.fixture
def mock_context_no_org():
    """Create a mock gRPC context with user ID but no organization."""
    context = Mock()
    context.invocation_metadata.return_value = [("x-user-id", "123")]
    return context


@pytest.fixture
def chat_service(
    mock_logger,
    mock_llm,
    mock_vector_store,
    mock_embedder,
    mock_reranker,
    mock_chunk_service,
):
    return ChatService(
        logger=mock_logger,
        llm_provider=mock_llm,
        vector_store=mock_vector_store,
        embedder=mock_embedder,
        reranker=mock_reranker,
        chunk_service=mock_chunk_service,
    )


class TestGetUserIdFromContext:
    """Tests for the get_user_id_from_context helper function."""

    def test_valid_user_id(self, mock_context):
        """Test extracting valid user ID from metadata."""
        user_id = get_user_id_from_context(mock_context)
        assert user_id == 123

    def test_missing_user_id(self, mock_context_no_user):
        """Test when user ID is not present in metadata."""
        user_id = get_user_id_from_context(mock_context_no_user)
        assert user_id is None

    def test_invalid_user_id(self):
        """Test when user ID is not a valid integer."""
        context = Mock()
        context.invocation_metadata.return_value = [("x-user-id", "invalid")]
        user_id = get_user_id_from_context(context)
        assert user_id is None

    def test_none_metadata(self):
        """Test when invocation_metadata returns None."""
        context = Mock()
        context.invocation_metadata.return_value = None
        user_id = get_user_id_from_context(context)
        assert user_id is None


class TestGetTenantContextFromMetadata:
    """Tests for the get_tenant_context_from_metadata helper function."""

    def test_valid_tenant_context(self, mock_context):
        """Test extracting valid tenant context from metadata."""
        org_id, group_ids = get_tenant_context_from_metadata(mock_context)
        assert org_id == 1
        assert group_ids == [1, 2, 3]

    def test_missing_tenant_context(self, mock_context_no_user):
        """Test when tenant context is not present in metadata."""
        org_id, group_ids = get_tenant_context_from_metadata(mock_context_no_user)
        assert org_id is None
        assert group_ids is None

    def test_org_only_no_groups(self):
        """Test when organization is present but groups are missing."""
        context = Mock()
        context.invocation_metadata.return_value = [
            ("x-user-id", "123"),
            ("x-organization-id", "5"),
        ]
        org_id, group_ids = get_tenant_context_from_metadata(context)
        assert org_id == 5
        assert group_ids is None

    def test_invalid_org_id(self):
        """Test when organization ID is not a valid integer."""
        context = Mock()
        context.invocation_metadata.return_value = [
            ("x-organization-id", "invalid"),
            ("x-group-ids", "1,2"),
        ]
        org_id, group_ids = get_tenant_context_from_metadata(context)
        assert org_id is None
        assert group_ids == [1, 2]


class TestChatService:
    """Tests for the ChatService gRPC service."""

    @pytest.mark.asyncio
    async def test_chat_unauthorized(self, chat_service, mock_context_no_user):
        """Test Chat returns error when user ID is not provided."""
        request = rs.ChatRequest(query="test query", session_id="session-1")

        responses = []
        async for response in chat_service.Chat(request, mock_context_no_user):
            responses.append(response)

        assert len(responses) == 1
        assert "Unauthorized" in responses[0].answer

    @pytest.mark.asyncio
    async def test_chat_missing_organization(self, chat_service, mock_context_no_org):
        """Test Chat returns error when organization context is not provided."""
        request = rs.ChatRequest(query="test query", session_id="session-1")

        responses = []
        async for response in chat_service.Chat(request, mock_context_no_org):
            responses.append(response)

        assert len(responses) == 1
        assert "Organization context not provided" in responses[0].answer

    @pytest.mark.asyncio
    async def test_chat_no_documents_found(self, chat_service, mock_vector_store, mock_context):
        """Test Chat returns appropriate message when no documents are found."""
        mock_vector_store.search_with_tenant_filter = AsyncMock(return_value=[])
        request = rs.ChatRequest(query="test query", session_id="session-1")

        responses = []
        async for response in chat_service.Chat(request, mock_context):
            responses.append(response)

        assert len(responses) == 1
        assert "couldn't find any relevant documents" in responses[0].answer

    @pytest.mark.asyncio
    async def test_chat_success_with_documents(
        self,
        chat_service,
        mock_vector_store,
        mock_embedder,
        mock_reranker,
        mock_llm,
        mock_chunk_service,
        mock_context,
    ):
        """Test successful chat flow with document retrieval and LLM response."""
        # Setup mock hits from vector store
        mock_hit = Mock()
        mock_hit.id = "chunk-1"
        mock_hit.payload = {"filename": "test.pdf"}
        mock_vector_store.search_with_tenant_filter = AsyncMock(return_value=[mock_hit])

        # Setup mock chunk from database
        mock_chunk = Mock()
        mock_chunk.id = "chunk-1"
        mock_chunk.content = "This is the document content."
        mock_chunk.document_id = "doc-1"
        mock_chunk.page_number = 1
        mock_chunk_service.get_chunks_by_ids = AsyncMock(return_value=[mock_chunk])

        # Setup reranker to return the passage
        mock_reranker.rerank = Mock(
            return_value=[
                {
                    "text": "This is the document content.",
                    "meta": {
                        "chunk_id": "chunk-1",
                        "document_id": "doc-1",
                        "filename": "test.pdf",
                        "page": 1,
                    },
                }
            ]
        )

        # Setup LLM response
        mock_llm.generate_response = MagicMock(return_value=async_iter(["The answer ", "is 42."]))

        request = rs.ChatRequest(query="What is the answer?", session_id="session-1")

        responses = []
        async for response in chat_service.Chat(request, mock_context):
            responses.append(response)

        # Verify we got streaming responses
        assert len(responses) >= 2
        # Verify embedder was called with query
        mock_embedder.generate.assert_called_once_with(["What is the answer?"])
        # Verify vector store search was called with tenant filter
        mock_vector_store.search_with_tenant_filter.assert_called_once()
        # Verify reranker was called
        mock_reranker.rerank.assert_called_once()

    @pytest.mark.asyncio
    async def test_chat_no_chunks_in_database(
        self,
        chat_service,
        mock_vector_store,
        mock_chunk_service,
        mock_context,
    ):
        """Test Chat returns error when vector hits don't match database chunks."""
        # Vector store returns hits
        mock_hit = Mock()
        mock_hit.id = "chunk-1"
        mock_hit.payload = {"filename": "test.pdf"}
        mock_vector_store.search_with_tenant_filter = AsyncMock(return_value=[mock_hit])

        # But database has no matching chunks
        mock_chunk_service.get_chunks_by_ids = AsyncMock(return_value=[])

        request = rs.ChatRequest(query="test query", session_id="session-1")

        responses = []
        async for response in chat_service.Chat(request, mock_context):
            responses.append(response)

        assert len(responses) == 1
        assert "couldn't find the document content" in responses[0].answer

    @pytest.mark.asyncio
    async def test_chat_llm_error_handling(
        self,
        chat_service,
        mock_vector_store,
        mock_chunk_service,
        mock_reranker,
        mock_llm,
        mock_context,
    ):
        """Test Chat handles LLM errors gracefully."""
        # Setup successful document retrieval
        mock_hit = Mock()
        mock_hit.id = "chunk-1"
        mock_hit.payload = {"filename": "test.pdf"}
        mock_vector_store.search_with_tenant_filter = AsyncMock(return_value=[mock_hit])

        mock_chunk = Mock()
        mock_chunk.id = "chunk-1"
        mock_chunk.content = "Content"
        mock_chunk.document_id = "doc-1"
        mock_chunk.page_number = 1
        mock_chunk_service.get_chunks_by_ids = AsyncMock(return_value=[mock_chunk])

        mock_reranker.rerank = Mock(
            return_value=[
                {
                    "text": "Content",
                    "meta": {"document_id": "doc-1", "filename": "test.pdf", "page": 1},
                }
            ]
        )

        # LLM returns an error message
        mock_llm.generate_response = MagicMock(
            return_value=async_iter(["Error: API rate limit exceeded"])
        )

        request = rs.ChatRequest(query="test query", session_id="session-1")

        responses = []
        async for response in chat_service.Chat(request, mock_context):
            responses.append(response)

        # Should still return the error message
        assert any("Error" in r.answer for r in responses)
