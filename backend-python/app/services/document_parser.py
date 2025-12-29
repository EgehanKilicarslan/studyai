import re
from pathlib import Path
from typing import Dict, List, Tuple

import fitz
from config import Settings
from langchain_text_splitters import RecursiveCharacterTextSplitter
from logger import AppLogger


class DocumentParser:
    def __init__(self, settings: Settings, logger: AppLogger) -> None:
        self.logger = logger.get_logger(__name__)
        self.text_splitter = RecursiveCharacterTextSplitter(
            chunk_size=settings.embedding_chunk_size,
            chunk_overlap=settings.embedding_chunk_overlap,
            separators=["\n\n", "\n", " ", ""],
        )

    def parse_file(self, file_path: str, filename: str) -> Tuple[List[str], List[Dict]]:
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
