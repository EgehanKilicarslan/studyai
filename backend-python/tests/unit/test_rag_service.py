from unittest.mock import AsyncMock, MagicMock, Mock, PropertyMock, patch

import pytest
from app.services.rag_service import RagService
from pb import rag_service_pb2 as rs


async def async_iter(items):
    for item in items:
        yield item


@pytest.fixture
def mock_settings():
    """Common settings mock for all tests."""
    settings = Mock()
    settings.maximum_file_size = 1024 * 1024
    settings.embedding_chunk_size = 500
    settings.embedding_chunk_overlap = 50
    return settings


@pytest.fixture
def mock_llm():
    """Common LLM mock for all tests."""
    llm = Mock()
    llm.generate_response = MagicMock(return_value=async_iter(["Answer"]))
    type(llm).provider_name = PropertyMock(return_value="dummy")
    return llm


@pytest.fixture
def mock_embedding_service():
    """Common embedding service mock for all tests."""
    service = Mock()
    service.search = AsyncMock(return_value=[])
    service.add_documents = AsyncMock(return_value=5)
    return service


@pytest.fixture
def rag_service(mock_settings, mock_llm, mock_embedding_service):
    """RAG service instance with mocked dependencies."""
    return RagService(mock_settings, mock_llm, mock_embedding_service)


@pytest.mark.asyncio
async def test_chat_success_scenario(rag_service, mock_llm, mock_embedding_service):
    """
    Scenario: The user asks a question, and a streaming response is returned.
    """
    # 1. ARRANGE
    mock_llm.generate_response = MagicMock(return_value=async_iter(["Hello ", "from ", "Python!"]))
    mock_embedding_service.search = AsyncMock(
        return_value=[
            {
                "content": "Context info",
                "metadata": {"filename": "doc.pdf", "page": 1},
                "score": 0.9,
            }
        ]
    )

    mock_request = rs.ChatRequest(query="Test Question", session_id="123")
    mock_context = Mock()

    # 2. ACT
    responses = [res async for res in rag_service.Chat(request=mock_request, context=mock_context)]

    # 3. ASSERT
    # LLM parts (3 parts) + Source info (1 part) = Total 4 responses expected
    assert len(responses) == 4

    # Is the combined answer correct?
    full_answer = "".join([r.answer for r in responses])
    assert "Hello from Python!" in full_answer

    # Does the last message contain sources?
    last_response = responses[-1]
    assert len(last_response.source_documents) == 1
    assert last_response.source_documents[0].filename == "doc.pdf"


@pytest.mark.asyncio
async def test_upload_document_success(mock_settings, mock_embedding_service):
    """
    Scenario: A valid text file is uploaded.
    """
    # 1. ARRANGE
    service = RagService(mock_settings, Mock(), mock_embedding_service)

    # Create upload stream with metadata and chunks
    async def mock_request_iterator():
        # First yield metadata
        yield rs.UploadRequest(
            metadata=rs.UploadMetadata(filename="test_notes.txt", content_type="text/plain")
        )
        # Then yield file content as chunks
        content = b"This is a test content. " * 50
        yield rs.UploadRequest(chunk=content)

    # 2. ACT
    # Mock the _parse_document_sync to return expected chunks
    with patch("asyncio.to_thread", new_callable=AsyncMock) as mock_to_thread:
        # First call is for file write (we don't care about return)
        # Subsequent calls are for parse_document_sync
        mock_to_thread.side_effect = [
            None,  # temp_file.write
            (  # _parse_document_sync
                ["chunk1", "chunk2", "chunk3", "chunk4", "chunk5"],
                [{"filename": "test_notes.txt", "page": 1}] * 5,
            ),
            None,  # Path.unlink
        ]

        response = await service.UploadDocument(
            request_iterator=mock_request_iterator(), context=Mock()
        )

    # 3. ASSERT
    assert response.status == "success"
    assert response.chunks_count == 5
    mock_embedding_service.add_documents.assert_called_once()


@pytest.mark.asyncio
async def test_upload_document_validation_error(mock_settings):
    """
    Scenario: Unsupported file format.
    """
    service = RagService(mock_settings, Mock(), Mock())

    async def mock_request_iterator():
        yield rs.UploadRequest(
            metadata=rs.UploadMetadata(
                filename="virus.exe", content_type="application/octet-stream"
            )
        )
        yield rs.UploadRequest(chunk=b"binary data")

    response = await service.UploadDocument(
        request_iterator=mock_request_iterator(), context=Mock()
    )

    assert response.status == "error"
    assert "Unsupported file type" in response.message


@pytest.mark.asyncio
async def test_chat_with_empty_query(rag_service, mock_llm):
    """Test handling of empty query."""
    mock_llm.generate_response = MagicMock(return_value=async_iter(["No query provided"]))
    mock_request = rs.ChatRequest(query="", session_id="123")

    responses = [res async for res in rag_service.Chat(request=mock_request, context=Mock())]

    assert len(responses) > 0


@pytest.mark.asyncio
async def test_chat_with_long_query(rag_service, mock_llm):
    """Test handling of very long queries."""
    mock_llm.generate_response = MagicMock(return_value=async_iter(["Response to long query"]))
    mock_request = rs.ChatRequest(query="x" * 10000, session_id="123")

    responses = [res async for res in rag_service.Chat(request=mock_request, context=Mock())]

    assert len(responses) > 0


@pytest.mark.asyncio
async def test_chat_error_handling(rag_service, mock_llm, mock_embedding_service):
    """Test error handling during chat."""
    mock_llm.generate_response = MagicMock(
        return_value=async_iter(["Error generating response: Test error"])
    )
    mock_embedding_service.search = AsyncMock(side_effect=Exception("DB Error"))
    mock_request = rs.ChatRequest(query="test", session_id="123")

    responses = [res async for res in rag_service.Chat(request=mock_request, context=Mock())]

    assert len(responses) > 0
    assert any("error" in r.answer.lower() for r in responses)


@pytest.mark.asyncio
async def test_chat_returns_processing_time(rag_service):
    """Test that processing time is included in response."""
    mock_request = rs.ChatRequest(query="test", session_id="123")

    responses = [res async for res in rag_service.Chat(request=mock_request, context=Mock())]

    assert responses[-1].processing_time_ms >= 0


@pytest.mark.asyncio
async def test_chat_passes_context_docs_to_llm(rag_service, mock_llm, mock_embedding_service):
    """Test that context documents are passed to LLM."""
    mock_embedding_service.search = AsyncMock(
        return_value=[{"content": "Doc1", "metadata": {}, "score": 0.9}]
    )
    mock_request = rs.ChatRequest(query="test", session_id="123")

    await rag_service.Chat(request=mock_request, context=Mock()).__anext__()

    mock_llm.generate_response.assert_called_once()
    call_args = mock_llm.generate_response.call_args
    assert len(call_args[1]["context_docs"]) == 1


@pytest.mark.asyncio
async def test_chat_passes_empty_history(rag_service, mock_llm):
    """Test that empty history is passed to LLM."""
    mock_request = rs.ChatRequest(query="test", session_id="123")

    await rag_service.Chat(request=mock_request, context=Mock()).__anext__()

    mock_llm.generate_response.assert_called_once()
    call_args = mock_llm.generate_response.call_args
    assert call_args[1]["history"] == []
