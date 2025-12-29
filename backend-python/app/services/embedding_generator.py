import asyncio
from typing import List

from config import Settings
from fastembed import TextEmbedding
from logger import AppLogger


class EmbeddingGenerator:
    def __init__(self, settings: Settings, logger: AppLogger) -> None:
        self.logger = logger.get_logger(__name__)

        self.logger.info(f"📦 [EmbeddingGenerator] Loading model: {settings.embedding_model_name}")
        self.model = TextEmbedding(
            model_name=settings.embedding_model_name, cache_dir="/home/appuser/.cache/models"
        )

        dummy_vec = list(self.model.embed(["test"]))[0]
        self._vector_size = len(dummy_vec)
        self.logger.info(
            f"✅ [EmbeddingGenerator] Model loaded with vector size: {self._vector_size}"
        )

    @property
    def vector_size(self) -> int:
        return self._vector_size

    def generate_sync(self, documents: List[str]) -> List[List[float]]:
        self.logger.info(
            f"🔍 [EmbeddingGenerator] Generating embeddings for {len(documents)} documents (sync)"
        )
        return [e.tolist() for e in self.model.embed(documents)]

    async def generate(self, documents: List[str]) -> List[List[float]]:
        return await asyncio.to_thread(self.generate_sync, documents)
