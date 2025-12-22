import uuid
from typing import Dict, List

from fastembed import TextEmbedding
from qdrant_client import QdrantClient, models

from app.config import Settings


class EmbeddingService:
    def __init__(self, settings: Settings):
        host = settings.qdrant_host
        port = settings.qdrant_port

        print(f"📦 [EmbeddingService] Connecting to Qdrant at {host}:{port}")
        self.client = QdrantClient(host=host, port=port)
        self.collection_name = "school_docs"

        self.embedding_model = TextEmbedding(model_name="BAAI/bge-small-en-v1.5")

        self._create_collection_if_not_exists(vector_size=settings.embedding_vector_size)

    def _create_collection_if_not_exists(self, vector_size: int):
        if not self.client.collection_exists(self.collection_name):
            self.client.create_collection(
                collection_name=self.collection_name,
                vectors_config=models.VectorParams(
                    size=vector_size,
                    distance=models.Distance.COSINE,
                ),
            )
            print("✅ [EmbeddingService] Collection created.")

    def add_documents(self, documents: List[str], metadatas: List[Dict]):
        """Convert texts to vectors and upload to Qdrant."""
        if not documents:
            return 0

        embeddings = list(self.embedding_model.embed(documents))

        points = [
            models.PointStruct(
                id=uuid.uuid4().hex,
                vector=emb.tolist(),
                payload={"page_content": doc, **meta},
            )
            for doc, emb, meta in zip(documents, embeddings, metadatas)
        ]

        self.client.upsert(collection_name=self.collection_name, points=points)
        return len(points)

    def search(self, query: str, limit: int = 3) -> List[Dict]:
        """Return the most relevant documents for the query."""
        query_vec = list(self.embedding_model.embed([query]))[0]

        hits = self.client.query_points(
            collection_name=self.collection_name, query=query_vec.tolist(), limit=limit
        ).points

        return [
            {
                "content": hit.payload.get("page_content", "") if hit.payload else "",
                "metadata": {k: v for k, v in hit.payload.items() if k != "page_content"}
                if hit.payload
                else {},
                "score": hit.score,
            }
            for hit in hits
        ]
