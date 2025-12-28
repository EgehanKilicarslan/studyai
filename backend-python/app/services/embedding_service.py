import asyncio
import uuid
from typing import Any, Dict, List

from fastembed import TextEmbedding
from flashrank import Ranker, RerankRequest
from qdrant_client import AsyncQdrantClient, QdrantClient, models

from app.config import Settings


class EmbeddingService:
    """
    Service for managing document embeddings and vector search using Qdrant.
    Handles embedding generation with FastEmbed and vector storage/retrieval.
    """

    def __init__(self, settings: Settings):
        # Initialize connection parameters from settings
        self.host = settings.qdrant_host
        self.port = settings.qdrant_port
        self.collection_name = settings.qdrant_collection
        self.vector_size = settings.embedding_vector_size

        # Load the BGE small embedding model (384 dimensions)
        self.embedding_model = TextEmbedding(
            model_name="BAAI/bge-small-en-v1.5", cache_dir="/home/appuser/.cache/models"
        )

        # Load the ranker model for semantic search ranking
        self.ranker = Ranker(
            model_name="ms-marco-MiniLM-L-12-v2", cache_dir="/home/appuser/.cache/models"
        )
        print(f"📦 [EmbeddingService] Connecting to Qdrant at {self.host}:{self.port}")

        # Use synchronous client for initialization to ensure collection exists
        sync_client = QdrantClient(host=self.host, port=self.port)
        try:
            self._create_collection_if_not_exists(sync_client)
        finally:
            sync_client.close()

        # Create async client for runtime operations
        self.client = AsyncQdrantClient(host=self.host, port=self.port)

    def _create_collection_if_not_exists(self, client: QdrantClient):
        """Create Qdrant collection with cosine similarity if it doesn't exist."""
        if not client.collection_exists(self.collection_name):
            client.create_collection(
                collection_name=self.collection_name,
                vectors_config=models.VectorParams(
                    size=self.vector_size,
                    distance=models.Distance.COSINE,  # Use cosine similarity for semantic search
                ),
            )
            print("✅ [EmbeddingService] Collection created (Startup check).")

    def _generate_embeddings_sync(self, documents: List[str]) -> List[List[float]]:
        """Generate embeddings synchronously using FastEmbed model."""
        embeddings_generator = self.embedding_model.embed(documents)
        # Convert numpy arrays to lists for JSON serialization
        return [e.tolist() for e in embeddings_generator]

    async def add_documents(
        self, documents: List[str], metadatas: List[Dict], batch_size: int = 32
    ):
        """
        Add documents to the vector store in batches.

        Args:
            documents: List of text documents to embed and store
            metadatas: List of metadata dicts corresponding to each document
            batch_size: Number of documents to process per batch (default: 32)

        Returns:
            Total number of points added to the collection
        """
        if not documents:
            return 0

        total_points = 0

        # Process documents in batches to manage memory and API limits
        for i in range(0, len(documents), batch_size):
            batch_docs = documents[i : i + batch_size]
            batch_meta = metadatas[i : i + batch_size]

            # Generate embeddings in a thread pool to avoid blocking async loop
            embeddings = await asyncio.to_thread(self._generate_embeddings_sync, batch_docs)

            # Create point structures with unique IDs for Qdrant
            points = [
                models.PointStruct(
                    id=uuid.uuid4().hex,  # Generate unique ID for each point
                    vector=emb,
                    payload={"page_content": doc, **meta},  # Store content and metadata
                )
                for doc, emb, meta in zip(batch_docs, embeddings, batch_meta)
            ]

            # Upsert points to Qdrant (insert or update if ID exists)
            await self.client.upsert(collection_name=self.collection_name, points=points)
            total_points += len(points)

        return total_points

    async def search(self, query: str, limit: int = 5) -> List[Dict[str, Any]]:
        """
        Perform semantic search for similar documents.

        Args:
            query: Search query text
            limit: Maximum number of results to return (default: 5)

        Returns:
            List of dicts containing content, metadata, and similarity score
        """
        # Generate embedding for the query
        query_embeddings = await asyncio.to_thread(self._generate_embeddings_sync, [query])
        query_vec = query_embeddings[0]

        # Query Qdrant for similar vectors
        search_result = await self.client.query_points(
            collection_name=self.collection_name,
            query=query_vec,
            limit=25,
            with_payload=True,
        )

        if not search_result.points:
            return []

        hits = search_result.points

        passages = [
            {
                "id": hit.id,
                "text": hit.payload.get("page_content", "") if hit.payload else "",
                "meta": hit.payload,
            }
            for hit in hits
        ]

        rerank_request = RerankRequest(query=query, passages=passages)
        results = self.ranker.rerank(rerank_request)

        final_results = results[:limit]

        return [
            {
                "content": res["text"],
                "metadata": {k: v for k, v in res["meta"].items() if k != "page_content"}
                if res.get("meta")
                else {},
                "score": res.get("score", 0.0),
            }
            for res in final_results
        ]

    async def close(self):
        """Close the async Qdrant client connection."""
        await self.client.close()
