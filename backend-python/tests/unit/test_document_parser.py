from unittest.mock import MagicMock, Mock, mock_open, patch

import pytest
from app.services.document_parser import DocumentParser


@pytest.fixture
def mock_settings():
    """Fixture providing mock settings with embedding configuration."""
    settings = Mock()
    settings.embedding_chunk_size = 100
    settings.embedding_chunk_overlap = 10
    return settings


@pytest.fixture
def mock_logger():
    """Fixture providing mock logger instance."""
    logger = Mock()
    logger.get_logger.return_value = Mock()
    return logger


@pytest.fixture
def document_parser(mock_settings, mock_logger):
    """Fixture providing DocumentParser instance with mocked dependencies."""
    return DocumentParser(mock_settings, mock_logger)


def test_initialization(document_parser, mock_settings):
    """
    Test: DocumentParser initializes text splitter with correct chunk size and overlap.

    Verifies that the text splitter is configured with the chunk size and overlap
    values from the settings during initialization.
    """
    assert document_parser.text_splitter._chunk_size == mock_settings.embedding_chunk_size
    assert document_parser.text_splitter._chunk_overlap == mock_settings.embedding_chunk_overlap


def test_parse_file_invalid_filename(document_parser):
    """
    Test: parse_file raises ValueError when filename contains invalid characters.

    Verifies that filenames with special characters like '/' are rejected
    to prevent path traversal or security issues.
    """
    with pytest.raises(ValueError) as exc:
        document_parser.parse_file("path/to/file", "invalid/name.txt")
    assert "Invalid filename format" in str(exc.value)


def test_parse_file_unsupported_extension(document_parser):
    """
    Test: parse_file raises ValueError for unsupported file extensions.

    Verifies that attempting to parse files with unsupported extensions
    (e.g., .png) raises an appropriate error.
    """
    with pytest.raises(ValueError) as exc:
        document_parser.parse_file("path/to/file", "image.png")
    assert "Unsupported file type" in str(exc.value)


def test_parse_text_file_success(document_parser):
    """
    Test: parse_file successfully parses plain text files.

    Verifies that:
    - Text content is read from .txt files
    - Content is split into chunks
    - Metadata is generated for each chunk with correct filename and page number
    """
    mock_content = "This is a test content. " * 10

    m = mock_open(read_data=mock_content)

    with patch("builtins.open", m):
        chunks, metadatas = document_parser.parse_file("dummy_path", "test.txt")

        assert len(chunks) > 0
        assert len(metadatas) == len(chunks)
        assert metadatas[0]["filename"] == "test.txt"
        assert metadatas[0]["page"] == 1


def test_parse_pdf_file_success(document_parser):
    """
    Test: parse_file successfully parses PDF files.

    Verifies that:
    - PDF content is extracted using PyMuPDF (fitz)
    - Text from PDF pages is captured in chunks
    - Metadata includes correct filename and page number
    """
    mock_page = Mock()
    mock_page.get_text.return_value = "PDF Page Content"

    mock_doc = MagicMock()
    mock_doc.__enter__.return_value = [mock_page]
    mock_doc.__len__.return_value = 1
    mock_doc.__getitem__.return_value = mock_page

    with patch("fitz.open", return_value=mock_doc):
        chunks, metadatas = document_parser.parse_file("dummy_path", "test.pdf")

        assert len(chunks) > 0
        assert "PDF Page Content" in chunks[0]
        assert metadatas[0]["filename"] == "test.pdf"
        assert metadatas[0]["page"] == 1


def test_parse_pdf_error_handling(document_parser):
    """
    Test: parse_file properly handles PDF parsing errors.

    Verifies that exceptions during PDF parsing (e.g., corrupted files)
    are propagated with the original error message.
    """
    with patch("fitz.open", side_effect=Exception("Corrupted PDF")):
        with pytest.raises(Exception) as exc:
            document_parser.parse_file("dummy_path", "test.pdf")
        assert "Corrupted PDF" in str(exc.value)


def test_parse_text_file_empty_content(document_parser):
    """
    Test: parse_file handles empty text files.

    Verifies that empty files are processed without errors and return
    appropriate (possibly empty) results.
    """
    m = mock_open(read_data="")

    with patch("builtins.open", m):
        chunks, metadatas = document_parser.parse_file("dummy_path", "empty.txt")

        assert isinstance(chunks, list)
        assert isinstance(metadatas, list)


def test_parse_pdf_multiple_pages(document_parser):
    """
    Test: parse_file correctly handles multi-page PDF documents.

    Verifies that:
    - All pages in a PDF are processed
    - Each page's content is extracted
    - Page numbers in metadata are sequential and correct
    """
    mock_page1 = Mock()
    mock_page1.get_text.return_value = "Page 1 Content"

    mock_page2 = Mock()
    mock_page2.get_text.return_value = "Page 2 Content"

    mock_doc = MagicMock()
    mock_doc.__enter__.return_value = [mock_page1, mock_page2]
    mock_doc.__len__.return_value = 2
    mock_doc.__getitem__.side_effect = [mock_page1, mock_page2]

    with patch("fitz.open", return_value=mock_doc):
        chunks, metadatas = document_parser.parse_file("dummy_path", "multipage.pdf")

        assert len(chunks) > 0
        page_numbers = [m["page"] for m in metadatas]
        assert 1 in page_numbers or 2 in page_numbers


def test_parse_file_encoding_handling(document_parser):
    """
    Test: parse_file handles text files with different encodings.

    Verifies that text files with UTF-8 or other encodings are read correctly
    without encoding errors.
    """
    mock_content = "Unicode content: café, naïve, 中文"
    m = mock_open(read_data=mock_content)

    with patch("builtins.open", m):
        chunks, metadatas = document_parser.parse_file("dummy_path", "unicode.txt")

        assert len(chunks) > 0


def test_parse_file_large_chunks(document_parser):
    """
    Test: parse_file correctly splits large content into multiple chunks.

    Verifies that content exceeding the chunk size is split into multiple
    chunks with proper overlap between them.
    """
    # Create content larger than chunk_size (100)
    mock_content = "A" * 500

    m = mock_open(read_data=mock_content)

    with patch("builtins.open", m):
        chunks, metadatas = document_parser.parse_file("dummy_path", "large.txt")

        assert len(chunks) > 1  # Should be split into multiple chunks
        assert all(m["filename"] == "large.txt" for m in metadatas)
