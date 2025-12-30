from unittest.mock import AsyncMock, Mock

import pytest
from app.services.grpc.knowledge_base_service import (
    KnowledgeBaseService,
    get_user_id_from_context,
)
from pb import rag_service_pb2 as rs


async def async_iter(items):
    """Helper to create an async iterator from a list."""
    for item in items:
        yield item


@pytest.fixture
def mock_settings():
    settings = Mock()
    settings.maximum_file_size = 1024 * 1024  # 1MB
    return settings


@pytest.fixture
def mock_logger():
    logger = Mock()
    logger.get_logger.return_value = Mock()
    return logger


@pytest.fixture
def mock_vector_store():
    store = Mock()
    store.upsert_vectors_with_chunk_ids = AsyncMock(return_value=5)
    store.delete_by_document_id = AsyncMock(return_value=None)
    return store


@pytest.fixture
def mock_embedder():
    embedder = Mock()
    embedder.generate = AsyncMock(return_value=[[0.1, 0.2, 0.3]])
    return embedder


@pytest.fixture
def mock_parser():
    parser = Mock()
    parser.parse_file = Mock(return_value=(["chunk1", "chunk2"], [{"page": 1}, {"page": 2}]))
    return parser


@pytest.fixture
def mock_document_service():
    doc_service = Mock()
    # Mock document for get_or_register_document
    mock_doc = Mock()
    mock_doc.id = "doc-123"
    mock_doc.chunk_count = 2
    doc_service.get_or_register_document = AsyncMock(return_value=(mock_doc, True))

    # Mock stored chunks
    mock_chunk1 = Mock()
    mock_chunk1.id = "chunk-1"
    mock_chunk2 = Mock()
    mock_chunk2.id = "chunk-2"
    doc_service.store_chunks = AsyncMock(return_value=[mock_chunk1, mock_chunk2])

    # Mock list documents
    doc_service.list_user_documents = AsyncMock(return_value=[])

    # Mock delete
    doc_service.delete_document_for_user = AsyncMock(return_value=True)

    return doc_service


@pytest.fixture
def mock_context():
    """Create a mock gRPC context with user ID in metadata."""
    context = Mock()
    context.invocation_metadata.return_value = [("x-user-id", "123")]
    return context


@pytest.fixture
def mock_context_no_user():
    """Create a mock gRPC context without user ID."""
    context = Mock()
    context.invocation_metadata.return_value = []
    return context


@pytest.fixture
def knowledge_base_service(
    mock_settings,
    mock_logger,
    mock_vector_store,
    mock_embedder,
    mock_parser,
    mock_document_service,
):
    return KnowledgeBaseService(
        settings=mock_settings,
        logger=mock_logger,
        vector_store=mock_vector_store,
        embedder=mock_embedder,
        parser=mock_parser,
        document_service=mock_document_service,
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


class TestUploadDocument:
    """Tests for the UploadDocument gRPC method."""

    @pytest.mark.asyncio
    async def test_upload_unauthorized(self, knowledge_base_service, mock_context_no_user):
        """Test UploadDocument returns error when user ID is not provided."""

        async def request_iterator():
            yield rs.UploadRequest(
                metadata=rs.UploadMetadata(filename="test.pdf", content_type="application/pdf")
            )

        response = await knowledge_base_service.UploadDocument(
            request_iterator(), mock_context_no_user
        )

        assert response.status == "error"
        assert "Unauthorized" in response.message

    @pytest.mark.asyncio
    async def test_upload_no_metadata(self, knowledge_base_service, mock_context):
        """Test UploadDocument returns error when no metadata is sent."""

        async def request_iterator():
            yield rs.UploadRequest(chunk=b"some data")

        response = await knowledge_base_service.UploadDocument(request_iterator(), mock_context)

        assert response.status == "error"
        assert "Metadata must be sent" in response.message

    @pytest.mark.asyncio
    async def test_upload_empty_file(self, knowledge_base_service, mock_context):
        """Test UploadDocument returns warning for empty file."""

        async def request_iterator():
            yield rs.UploadRequest(
                metadata=rs.UploadMetadata(filename="test.pdf", content_type="application/pdf")
            )
            # No chunks sent

        response = await knowledge_base_service.UploadDocument(request_iterator(), mock_context)

        assert response.status == "warning"
        assert "empty file" in response.message.lower()

    @pytest.mark.asyncio
    async def test_upload_file_too_large(
        self,
        mock_logger,
        mock_vector_store,
        mock_embedder,
        mock_parser,
        mock_document_service,
        mock_context,
    ):
        """Test UploadDocument returns error when file exceeds max size."""
        # Create settings with small file size limit
        small_settings = Mock()
        small_settings.maximum_file_size = 100  # 100 bytes

        service = KnowledgeBaseService(
            settings=small_settings,
            logger=mock_logger,
            vector_store=mock_vector_store,
            embedder=mock_embedder,
            parser=mock_parser,
            document_service=mock_document_service,
        )

        async def request_iterator():
            yield rs.UploadRequest(
                metadata=rs.UploadMetadata(filename="test.pdf", content_type="application/pdf")
            )
            yield rs.UploadRequest(chunk=b"x" * 200)  # Exceeds limit

        response = await service.UploadDocument(request_iterator(), mock_context)

        assert response.status == "error"
        assert "exceeds the maximum limit" in response.message

    @pytest.mark.asyncio
    async def test_upload_success_new_document(
        self,
        knowledge_base_service,
        mock_document_service,
        mock_parser,
        mock_embedder,
        mock_vector_store,
        mock_context,
    ):
        """Test successful upload of a new document."""

        async def request_iterator():
            yield rs.UploadRequest(
                metadata=rs.UploadMetadata(filename="test.pdf", content_type="application/pdf")
            )
            yield rs.UploadRequest(chunk=b"PDF content here")

        response = await knowledge_base_service.UploadDocument(request_iterator(), mock_context)

        assert response.status == "success"
        assert response.document_id == "doc-123"
        # chunks_count comes from vector_store.upsert_vectors_with_chunk_ids (returns 5 in mock)
        assert response.chunks_count == 5
        mock_document_service.get_or_register_document.assert_called_once()
        mock_document_service.store_chunks.assert_called_once()
        mock_embedder.generate.assert_called_once()
        mock_vector_store.upsert_vectors_with_chunk_ids.assert_called_once()

    @pytest.mark.asyncio
    async def test_upload_existing_document(
        self, knowledge_base_service, mock_document_service, mock_context
    ):
        """Test uploading an existing document links it to user."""
        # Document already exists (is_new = False)
        mock_doc = Mock()
        mock_doc.id = "existing-doc"
        mock_doc.chunk_count = 5
        mock_document_service.get_or_register_document = AsyncMock(return_value=(mock_doc, False))

        async def request_iterator():
            yield rs.UploadRequest(
                metadata=rs.UploadMetadata(filename="test.pdf", content_type="application/pdf")
            )
            yield rs.UploadRequest(chunk=b"PDF content")

        response = await knowledge_base_service.UploadDocument(request_iterator(), mock_context)

        assert response.status == "success"
        assert "already exists" in response.message
        # store_chunks should NOT be called for existing documents
        mock_document_service.store_chunks.assert_not_called()

    @pytest.mark.asyncio
    async def test_upload_parser_error(self, knowledge_base_service, mock_parser, mock_context):
        """Test UploadDocument handles parser errors."""
        mock_parser.parse_file = Mock(side_effect=ValueError("Unsupported file type"))

        async def request_iterator():
            yield rs.UploadRequest(
                metadata=rs.UploadMetadata(filename="test.xyz", content_type="application/xyz")
            )
            yield rs.UploadRequest(chunk=b"content")

        response = await knowledge_base_service.UploadDocument(request_iterator(), mock_context)

        assert response.status == "error"
        assert "Unsupported file type" in response.message

    @pytest.mark.asyncio
    async def test_upload_no_text_extracted(
        self, knowledge_base_service, mock_parser, mock_context
    ):
        """Test UploadDocument returns warning when no text is extracted."""
        mock_parser.parse_file = Mock(return_value=([], []))

        async def request_iterator():
            yield rs.UploadRequest(
                metadata=rs.UploadMetadata(filename="empty.pdf", content_type="application/pdf")
            )
            yield rs.UploadRequest(chunk=b"content")

        response = await knowledge_base_service.UploadDocument(request_iterator(), mock_context)

        assert response.status == "warning"
        assert "No text extracted" in response.message


class TestDeleteDocument:
    """Tests for the DeleteDocument gRPC method."""

    @pytest.mark.asyncio
    async def test_delete_unauthorized(self, knowledge_base_service, mock_context_no_user):
        """Test DeleteDocument returns error when user ID is not provided."""
        request = rs.DeleteDocumentRequest(document_id="doc-123")

        response = await knowledge_base_service.DeleteDocument(request, mock_context_no_user)

        assert response.status == "error"
        assert "Unauthorized" in response.message

    @pytest.mark.asyncio
    async def test_delete_success_full_delete(
        self, knowledge_base_service, mock_document_service, mock_vector_store, mock_context
    ):
        """Test successful deletion when no other users are linked."""
        mock_document_service.delete_document_for_user = AsyncMock(return_value=True)
        request = rs.DeleteDocumentRequest(document_id="doc-123")

        response = await knowledge_base_service.DeleteDocument(request, mock_context)

        assert response.status == "success"
        mock_document_service.delete_document_for_user.assert_called_once_with(123, "doc-123")
        mock_vector_store.delete_by_document_id.assert_called_once_with("doc-123")

    @pytest.mark.asyncio
    async def test_delete_success_unlink_only(
        self, knowledge_base_service, mock_document_service, mock_vector_store, mock_context
    ):
        """Test successful unlink when other users are still linked."""
        mock_document_service.delete_document_for_user = AsyncMock(return_value=False)
        request = rs.DeleteDocumentRequest(document_id="doc-123")

        response = await knowledge_base_service.DeleteDocument(request, mock_context)

        assert response.status == "success"
        mock_document_service.delete_document_for_user.assert_called_once()
        # Vectors should NOT be deleted when other users are linked
        mock_vector_store.delete_by_document_id.assert_not_called()

    @pytest.mark.asyncio
    async def test_delete_error_handling(
        self, knowledge_base_service, mock_document_service, mock_context
    ):
        """Test DeleteDocument handles errors gracefully."""
        mock_document_service.delete_document_for_user = AsyncMock(
            side_effect=Exception("Database error")
        )
        request = rs.DeleteDocumentRequest(document_id="doc-123")

        response = await knowledge_base_service.DeleteDocument(request, mock_context)

        assert response.status == "error"
        assert "Database error" in response.message


class TestListDocuments:
    """Tests for the ListDocuments gRPC method."""

    @pytest.mark.asyncio
    async def test_list_unauthorized(self, knowledge_base_service, mock_context_no_user):
        """Test ListDocuments returns empty list when user ID is not provided."""
        request = rs.ListDocumentsRequest()

        response = await knowledge_base_service.ListDocuments(request, mock_context_no_user)

        assert response.documents == []

    @pytest.mark.asyncio
    async def test_list_empty(self, knowledge_base_service, mock_document_service, mock_context):
        """Test ListDocuments returns empty list when user has no documents."""
        mock_document_service.list_user_documents = AsyncMock(return_value=[])
        request = rs.ListDocumentsRequest()

        response = await knowledge_base_service.ListDocuments(request, mock_context)

        assert len(response.documents) == 0
        mock_document_service.list_user_documents.assert_called_once_with(123)

    @pytest.mark.asyncio
    async def test_list_success_with_documents(
        self, knowledge_base_service, mock_document_service, mock_context
    ):
        """Test ListDocuments returns user's documents."""
        from datetime import datetime, timezone

        mock_doc1 = Mock()
        mock_doc1.id = "doc-1"
        mock_doc1.filename = "document1.pdf"
        mock_doc1.created_at = datetime(2024, 1, 1, 12, 0, 0, tzinfo=timezone.utc)
        mock_doc1.chunk_count = 10

        mock_doc2 = Mock()
        mock_doc2.id = "doc-2"
        mock_doc2.filename = "document2.txt"
        mock_doc2.created_at = datetime(2024, 1, 2, 12, 0, 0, tzinfo=timezone.utc)
        mock_doc2.chunk_count = 5

        mock_document_service.list_user_documents = AsyncMock(return_value=[mock_doc1, mock_doc2])
        request = rs.ListDocumentsRequest()

        response = await knowledge_base_service.ListDocuments(request, mock_context)

        assert len(response.documents) == 2
        assert response.documents[0].document_id == "doc-1"
        assert response.documents[0].filename == "document1.pdf"
        assert response.documents[0].chunks_count == 10
        assert response.documents[1].document_id == "doc-2"
        assert response.documents[1].filename == "document2.txt"
        assert response.documents[1].chunks_count == 5

    @pytest.mark.asyncio
    async def test_list_document_with_none_created_at(
        self, knowledge_base_service, mock_document_service, mock_context
    ):
        """Test ListDocuments handles documents with None created_at."""
        mock_doc = Mock()
        mock_doc.id = "doc-1"
        mock_doc.filename = "test.pdf"
        mock_doc.created_at = None
        mock_doc.chunk_count = 3

        mock_document_service.list_user_documents = AsyncMock(return_value=[mock_doc])
        request = rs.ListDocumentsRequest()

        response = await knowledge_base_service.ListDocuments(request, mock_context)

        assert len(response.documents) == 1
        assert response.documents[0].upload_timestamp == 0

    @pytest.mark.asyncio
    async def test_list_error_handling(
        self, knowledge_base_service, mock_document_service, mock_context
    ):
        """Test ListDocuments handles errors gracefully."""
        mock_document_service.list_user_documents = AsyncMock(
            side_effect=Exception("Database error")
        )
        request = rs.ListDocumentsRequest()

        response = await knowledge_base_service.ListDocuments(request, mock_context)

        # Should return empty list on error
        assert response.documents == []
