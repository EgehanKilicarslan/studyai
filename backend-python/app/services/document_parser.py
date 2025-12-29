import re
from pathlib import Path
from typing import Dict, List, Tuple

import fitz
from config import Settings
from langchain_text_splitters import RecursiveCharacterTextSplitter
from logger import AppLogger


class DocumentParser:
    """
    DocumentParser is a service class responsible for parsing document files such as PDFs, text files,
    and markdown files. It extracts text content from these files and splits the text into smaller
    semantic chunks for further processing.

    Attributes:
        logger (Logger): A logger instance for logging messages and errors.
        text_splitter (RecursiveCharacterTextSplitter): A utility for splitting text into smaller chunks
            based on specified chunk size, overlap, and separators.

    Methods:
        __init__(settings: Settings, logger: AppLogger) -> None:
            Initializes the DocumentParser with the provided settings and logger.

        parse_file(file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
            Parses the given file based on its extension and returns text chunks along with metadata.

        _parse_pdf(file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
            Parses a PDF file, extracting text from each page and splitting it into chunks.

        _parse_text(file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
            Parses a text or markdown file, reading it in chunks to avoid memory issues, and splitting
            the content into smaller semantic chunks.

    Raises:
        ValueError: If the filename format is invalid or the file type is unsupported.
        Exception: If an error occurs during file parsing.
    """

    def __init__(self, settings: Settings, logger: AppLogger) -> None:
        """
        Initializes the DocumentParser service.

        Args:
            settings (Settings): Configuration settings for the application, including
                parameters for text splitting such as chunk size and overlap.
            logger (AppLogger): Application logger instance used for logging within the service.

        Attributes:
            logger (logging.Logger): Logger instance for the current module.
            text_splitter (RecursiveCharacterTextSplitter): Utility for splitting text into
                chunks based on specified chunk size, overlap, and separators.
        """

        self.logger = logger.get_logger(__name__)
        self.text_splitter = RecursiveCharacterTextSplitter(
            chunk_size=settings.embedding_chunk_size,
            chunk_overlap=settings.embedding_chunk_overlap,
            separators=["\n\n", "\n", " ", ""],
        )

    def parse_file(self, file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
        """
        Parses a file and extracts its content based on the file type.

        Args:
            file_path (str): The full path to the file to be parsed.
            filename (str): The name of the file to be parsed.

        Returns:
            Tuple[List[str], List[Dict]]: A tuple containing:
                - A list of strings representing the parsed content.
                - A list of dictionaries containing metadata or additional information.

        Raises:
            ValueError: If the filename format is invalid or the file type is unsupported.
        """

        self.logger.info(f"[Parser] Processing: {filename}")

        if not re.match(r"^[\w\-. ]+$", filename):
            raise ValueError(f"Invalid filename format: {filename}")

        file_ext = Path(filename).suffix.lower()

        if file_ext == ".pdf":
            return self._parse_pdf(file_path, filename)
        elif file_ext in (".txt", ".md"):
            return self._parse_text(file_path, filename)
        else:
            raise ValueError(f"Unsupported file type: {filename}")

    def _parse_pdf(self, file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
        """
        Parses a PDF file and extracts text chunks along with their metadata.

        Args:
            file_path (str): The file path to the PDF document to be parsed.
            filename (str): The name of the file, used for metadata.

        Returns:
            Tuple[List[str], List[Dict]]: A tuple containing:
                - A list of text chunks extracted from the PDF.
                - A list of metadata dictionaries, each containing:
                    - "filename" (str): The name of the file.
                    - "page" (int): The page number where the text chunk was found.

        Raises:
            Exception: If an error occurs during PDF parsing, it logs the error and re-raises the exception.
        """

        text_chunks = []
        metadatas = []

        try:
            with fitz.open(file_path) as doc:
                for i in range(len(doc)):
                    page = doc[i]
                    text = page.get_text()

                    if isinstance(text, str) and text.strip():
                        page_chunks = self.text_splitter.split_text(text)
                        for chunk in page_chunks:
                            text_chunks.append(chunk)
                            metadatas.append({"filename": filename, "page": i + 1})

            return text_chunks, metadatas
        except Exception as e:
            self.logger.error(f"❌ PDF Parsing Error: {e}")
            raise

    def _parse_text(self, file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
        """
        Parses the text content of a file into semantic chunks and generates metadata for each chunk.

        Args:
            file_path (str): The path to the file to be parsed.
            filename (str): The name of the file being processed.

        Returns:
            Tuple[List[str], List[Dict]]: A tuple containing:
                - A list of text chunks (List[str]).
                - A list of metadata dictionaries (List[Dict]) corresponding to each chunk.
                  Each metadata dictionary contains:
                    - "filename" (str): The name of the file.
                    - "page" (int): The page number (currently hardcoded as 1).

        Raises:
            Exception: If an error occurs during file reading or processing, the exception is logged
                       and re-raised.

        Notes:
            - The file is read in chunks to avoid loading the entire file into memory.
            - Text is split into semantic chunks using the `text_splitter` instance.
            - Overlap is maintained between chunks to avoid splitting words or sentences at boundaries.
            - Metadata for each chunk includes the filename and a hardcoded page number.
        """

        CHUNK_SIZE = 1024 * 1024  # Read 1MB at a time

        text_chunks = []
        metadatas = []
        text_buffer = ""

        try:
            with open(file_path, "r", encoding="utf-8") as f:
                while True:
                    # Read file in chunks to avoid loading entire file into memory
                    chunk = f.read(CHUNK_SIZE)
                    if not chunk:
                        break

                    text_buffer += chunk

                    # Process buffer when it's large enough or at end of file
                    # Keep some overlap to avoid splitting words/sentences at chunk boundaries
                    if len(text_buffer) >= CHUNK_SIZE * 2 or not chunk:
                        # Split text into semantic chunks
                        file_chunks = self.text_splitter.split_text(text_buffer)

                        # Process all but the last chunk (keep last for overlap)
                        chunks_to_process = (
                            file_chunks[:-1] if len(file_chunks) > 1 else file_chunks
                        )

                        for text_chunk in chunks_to_process:
                            text_chunks.append(text_chunk)
                            metadatas.append({"filename": filename, "page": 1})

                        # Keep the last chunk as buffer for next iteration (for overlap)
                        text_buffer = file_chunks[-1] if len(file_chunks) > 1 else ""

            # Process any remaining text in buffer
            if text_buffer.strip():
                final_chunks = self.text_splitter.split_text(text_buffer)
                for text_chunk in final_chunks:
                    text_chunks.append(text_chunk)
                    metadatas.append({"filename": filename, "page": 1})

            return text_chunks, metadatas
        except Exception as e:
            self.logger.error(f"❌ Text Parsing Error: {e}")
            raise
