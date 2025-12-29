import uuid
from typing import Dict, List

from config import Settings
from logger import AppLogger
from qdrant_client import AsyncQdrantClient, QdrantClient, models

from .embedding_generator import EmbeddingGenerator


class VectorStore:
    def __init__(
        self, settings: Settings, logger: AppLogger, embedding_generator: EmbeddingGenerator
    ) -> None:
        self.logger = logger.get_logger(__name__)
        self.client = AsyncQdrantClient(host=settings.qdrant_host, port=settings.qdrant_port)
        self.collection_name = settings.qdrant_collection_name
        self.vector_size = embedding_generator.vector_size

        self._ensure_collection(settings)

    def _ensure_collection(self, settings: Settings) -> None:
        sync_client = QdrantClient(host=settings.qdrant_host, port=settings.qdrant_port)

        try:
            if not sync_client.collection_exists(self.collection_name):
                sync_client.create_collection(
                    collection_name=self.collection_name,
                    vectors_config=models.VectorParams(
                        size=self.vector_size,
                        distance=models.Distance.COSINE,  # Use cosine similarity for semantic search
                    ),
                )
                self.logger.info("✅ [VectorStore] Collection created (Startup check).")
        finally:
            sync_client.close()

    async def upsert_vectors(
        self, vectors: List[List[float]], contents: List[str], metadatas: List[Dict]
    ) -> int:
        if not vectors:
            return 0

        points = [
            models.PointStruct(
                id=uuid.uuid4().hex,
                vector=vec,
                payload={"content": content, **meta},
            )
            for vec, content, meta in zip(vectors, contents, metadatas)
        ]

        await self.client.upsert(collection_name=self.collection_name, points=points)
        return len(points)

    async def search(self, query_vector: List[float], limit: int = 25) -> List[models.ScoredPoint]:
        res = await self.client.query_points(
            collection_name=self.collection_name,
            query=query_vector,
            limit=limit,
            with_payload=True,
        )
        return res.points
