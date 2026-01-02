import tempfile
from pathlib import Path
from unittest.mock import AsyncMock, Mock, patch

import pytest
from app.services.grpc.knowledge_base_service import KnowledgeBaseService
from pb import rag_service_pb2 as rs


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
    store.upsert_vectors_with_metadata = AsyncMock(return_value=5)
    store.delete_by_document_id = AsyncMock(return_value=None)
    return store


@pytest.fixture
def mock_embedder():
    embedder = Mock()
    embedder.generate = AsyncMock(return_value=[[0.1, 0.2, 0.3], [0.4, 0.5, 0.6]])
    return embedder


@pytest.fixture
def mock_parser():
    parser = Mock()
    parser.parse_file = Mock(return_value=(["chunk1", "chunk2"], [{"page": 1}, {"page": 2}]))
    return parser


@pytest.fixture
def mock_chunk_service():
    chunk_service = Mock()
    # Mock stored chunks
    mock_chunk1 = Mock()
    mock_chunk1.id = "chunk-uuid-1"
    mock_chunk2 = Mock()
    mock_chunk2.id = "chunk-uuid-2"
    chunk_service.store_chunks = AsyncMock(return_value=[mock_chunk1, mock_chunk2])
    return chunk_service


@pytest.fixture
def mock_context():
    """Create a mock gRPC context."""
    context = Mock()
    return context


@pytest.fixture
def mock_celery_task():
    """Create a mock Celery task result."""
    task = Mock()
    task.id = "celery-task-uuid-123"
    return task


@pytest.fixture
def knowledge_base_service(
    mock_settings,
    mock_logger,
    mock_vector_store,
    mock_embedder,
    mock_parser,
    mock_chunk_service,
):
    return KnowledgeBaseService(
        settings=mock_settings,
        logger=mock_logger,
        vector_store=mock_vector_store,
        embedder=mock_embedder,
        parser=mock_parser,
        chunk_service=mock_chunk_service,
    )


class TestProcessDocument:
    """Tests for the ProcessDocument gRPC method."""

    @pytest.mark.asyncio
    async def test_process_document_file_not_found(self, knowledge_base_service, mock_context):
        """Test ProcessDocument returns error when file doesn't exist."""
        request = rs.ProcessDocumentRequest(
            document_id="doc-123",
            file_path="/nonexistent/path/test.pdf",
            filename="test.pdf",
            content_type="application/pdf",
            organization_id=1,
            group_id=0,
            owner_id=100,
        )

        response = await knowledge_base_service.ProcessDocument(request, mock_context)

        assert response.status == "error"
        assert "File not found" in response.message
        assert response.document_id == "doc-123"
        assert response.chunks_count == 0

    @pytest.mark.asyncio
    async def test_process_document_file_too_large(
        self,
        mock_logger,
        mock_vector_store,
        mock_embedder,
        mock_parser,
        mock_chunk_service,
        mock_context,
    ):
        """Test ProcessDocument returns error when file exceeds max size."""
        # Create settings with small file size limit
        small_settings = Mock()
        small_settings.maximum_file_size = 100  # 100 bytes

        service = KnowledgeBaseService(
            settings=small_settings,
            logger=mock_logger,
            vector_store=mock_vector_store,
            embedder=mock_embedder,
            parser=mock_parser,
            chunk_service=mock_chunk_service,
        )

        # Create a temp file that exceeds the limit
        with tempfile.NamedTemporaryFile(mode="wb", suffix=".pdf", delete=False) as f:
            f.write(b"x" * 200)  # 200 bytes
            temp_path = f.name

        try:
            request = rs.ProcessDocumentRequest(
                document_id="doc-123",
                file_path=temp_path,
                filename="large.pdf",
                content_type="application/pdf",
                organization_id=1,
                group_id=0,
                owner_id=100,
            )

            response = await service.ProcessDocument(request, mock_context)

            assert response.status == "error"
            assert "exceeds maximum" in response.message
        finally:
            Path(temp_path).unlink()

    @pytest.mark.asyncio
    async def test_process_document_success(
        self,
        knowledge_base_service,
        mock_celery_task,
        mock_context,
    ):
        """Test successful document processing queues Celery task."""
        # Create a temp file to process
        with tempfile.NamedTemporaryFile(mode="wb", suffix=".pdf", delete=False) as f:
            f.write(b"PDF content here")
            temp_path = f.name

        try:
            request = rs.ProcessDocumentRequest(
                document_id="doc-123",
                file_path=temp_path,
                filename="test.pdf",
                content_type="application/pdf",
                organization_id=1,
                group_id=10,
                owner_id=100,
            )

            with patch(
                "app.services.grpc.knowledge_base_service.process_document_task"
            ) as mock_task:
                mock_task.delay.return_value = mock_celery_task

                response = await knowledge_base_service.ProcessDocument(request, mock_context)

                assert response.status == "success"
                assert response.document_id == "doc-123"
                # chunks_count is 0 because actual processing happens in Celery
                assert response.chunks_count == 0
                assert "celery-task-uuid-123" in response.message

                # Verify Celery task was queued with correct arguments
                mock_task.delay.assert_called_once_with(
                    document_id="doc-123",
                    file_path=temp_path,
                    organization_id=1,
                    group_id=10,
                    owner_id=100,
                    filename="test.pdf",
                )
        finally:
            Path(temp_path).unlink()

    @pytest.mark.asyncio
    async def test_process_document_org_wide(
        self,
        knowledge_base_service,
        mock_celery_task,
        mock_context,
    ):
        """Test processing org-wide document (group_id=0) passes None to Celery."""
        with tempfile.NamedTemporaryFile(mode="wb", suffix=".pdf", delete=False) as f:
            f.write(b"PDF content")
            temp_path = f.name

        try:
            request = rs.ProcessDocumentRequest(
                document_id="doc-org-wide",
                file_path=temp_path,
                filename="org-doc.pdf",
                content_type="application/pdf",
                organization_id=1,
                group_id=0,  # Org-wide, no specific group
                owner_id=100,
            )

            with patch(
                "app.services.grpc.knowledge_base_service.process_document_task"
            ) as mock_task:
                mock_task.delay.return_value = mock_celery_task

                response = await knowledge_base_service.ProcessDocument(request, mock_context)

                assert response.status == "success"
                # Verify group_id is passed as None for org-wide documents
                call_args = mock_task.delay.call_args
                assert call_args.kwargs["group_id"] is None
        finally:
            Path(temp_path).unlink()

    @pytest.mark.asyncio
    async def test_process_document_celery_error(self, knowledge_base_service, mock_context):
        """Test ProcessDocument handles Celery errors gracefully."""
        with tempfile.NamedTemporaryFile(mode="wb", suffix=".pdf", delete=False) as f:
            f.write(b"content")
            temp_path = f.name

        try:
            request = rs.ProcessDocumentRequest(
                document_id="doc-123",
                file_path=temp_path,
                filename="test.pdf",
                content_type="application/pdf",
                organization_id=1,
                group_id=0,
                owner_id=100,
            )

            with patch(
                "app.services.grpc.knowledge_base_service.process_document_task"
            ) as mock_task:
                mock_task.delay.side_effect = Exception("Celery broker connection error")

                response = await knowledge_base_service.ProcessDocument(request, mock_context)

                assert response.status == "error"
                assert "Celery broker connection error" in response.message
        finally:
            Path(temp_path).unlink()


class TestDeleteDocument:
    """Tests for the DeleteDocument gRPC method."""

    @pytest.mark.asyncio
    async def test_delete_success(self, knowledge_base_service, mock_vector_store, mock_context):
        """Test successful document deletion."""
        request = rs.DeleteDocumentRequest(document_id="doc-123")

        response = await knowledge_base_service.DeleteDocument(request, mock_context)

        assert response.status == "success"
        mock_vector_store.delete_by_document_id.assert_called_once_with("doc-123")

    @pytest.mark.asyncio
    async def test_delete_error_handling(
        self, knowledge_base_service, mock_vector_store, mock_context
    ):
        """Test DeleteDocument handles errors gracefully."""
        mock_vector_store.delete_by_document_id = AsyncMock(side_effect=Exception("Database error"))
        request = rs.DeleteDocumentRequest(document_id="doc-123")

        response = await knowledge_base_service.DeleteDocument(request, mock_context)

        assert response.status == "error"
        assert "Database error" in response.message
