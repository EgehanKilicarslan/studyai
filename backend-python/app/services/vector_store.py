from typing import List

from config import Settings
from logger import AppLogger
from qdrant_client import AsyncQdrantClient, QdrantClient, models

from .embedding_generator import EmbeddingGenerator


class VectorStore:
    """
    A service class for managing a vector store using Qdrant, which supports operations
    such as ensuring the collection exists, upserting vectors, and performing vector searches.

    Attributes:
        logger (logging.Logger): Logger instance for logging messages.
        client (AsyncQdrantClient): Asynchronous Qdrant client for interacting with the vector database.
        collection_name (str): Name of the Qdrant collection used for storing vectors.
        vector_size (int): Dimensionality of the vectors stored in the collection.

    Methods:
        __init__(settings: Settings, logger: AppLogger, embedding_generator: EmbeddingGenerator):
            Initializes the VectorStore instance and ensures the collection exists.

        _ensure_collection(settings: Settings) -> None:
            Ensures the Qdrant collection exists, creating it if necessary.

        async upsert_vectors(vectors: List[List[float]], contents: List[str], metadatas: List[Dict]) -> int:
            Upserts vectors into the collection along with their associated content and metadata.

        async search(query_vector: List[float], limit: int = 25) -> List[models.ScoredPoint]:
            Searches the collection for the most similar vectors to the given query vector.
    """

    def __init__(
        self, settings: Settings, logger: AppLogger, embedding_generator: EmbeddingGenerator
    ) -> None:
        """
        Initializes the VectorStore service.

        Args:
            settings (Settings): The application settings containing configuration values such as
                Qdrant host, port, and collection name.
            logger (AppLogger): The application logger instance for logging messages.
            embedding_generator (EmbeddingGenerator): The embedding generator instance used to determine
                the vector size.

        Attributes:
            logger (logging.Logger): The logger instance for this class.
            client (AsyncQdrantClient): The asynchronous Qdrant client for interacting with the Qdrant database.
            collection_name (str): The name of the Qdrant collection to use.
            vector_size (int): The size of the vectors used in the collection.

        Raises:
            Any exceptions raised during the initialization of the Qdrant client or collection setup.
        """

        self.logger = logger.get_logger(__name__)
        self.client = AsyncQdrantClient(host=settings.qdrant_host, port=settings.qdrant_port)
        self.collection_name = settings.qdrant_collection_name
        self.vector_size = embedding_generator.vector_size

        self._ensure_collection(settings)

    def _ensure_collection(self, settings: Settings) -> None:
        """
        Ensures the existence of a Qdrant collection with the specified configuration.

        This method checks if a collection with the given name exists in the Qdrant vector database.
        If the collection does not exist, it creates one with the specified vector size and distance
        metric (cosine similarity). The method also logs the creation of the collection.

        Args:
            settings (Settings): The configuration settings containing the Qdrant host and port.

        Raises:
            Any exceptions raised during the collection existence check or creation will propagate
            to the caller.

        Note:
            The Qdrant client is closed after the operation, regardless of success or failure.
        """

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

    async def upsert_vectors_with_chunk_ids(
        self,
        vectors: List[List[float]],
        chunk_ids: List[str],
        document_id: str,
        filename: str,
    ) -> int:
        """
        Upserts vectors into the vector store with references to database chunk IDs.

        The actual content is stored in PostgreSQL, Qdrant only stores:
        - The vector embedding
        - chunk_id: Reference to the DocumentChunk in PostgreSQL
        - document_id: Reference to the Document in PostgreSQL
        - filename: For display purposes

        Args:
            vectors (List[List[float]]): A list of vectors to be upserted.
            chunk_ids (List[str]): A list of chunk IDs from the database.
            document_id (str): The document ID these chunks belong to.
            filename (str): The filename for display purposes.

        Returns:
            int: The number of vectors successfully upserted.
        """
        if not vectors:
            return 0

        points = [
            models.PointStruct(
                id=chunk_id,  # Use chunk_id as the point ID for easy lookup
                vector=vec,
                payload={
                    "chunk_id": chunk_id,
                    "document_id": document_id,
                    "filename": filename,
                },
            )
            for vec, chunk_id in zip(vectors, chunk_ids)
        ]

        await self.client.upsert(collection_name=self.collection_name, points=points)
        self.logger.info(f"[VectorStore] Upserted {len(points)} vectors for document {document_id}")
        return len(points)

    async def delete_by_document_id(self, document_id: str) -> None:
        """
        Deletes all vectors associated with a specific document ID.

        Args:
            document_id (str): The unique identifier of the document whose vectors should be deleted.
        """
        # Delete points where the payload contains the document_id
        await self.client.delete(
            collection_name=self.collection_name,
            points_selector=models.FilterSelector(
                filter=models.Filter(
                    must=[
                        models.FieldCondition(
                            key="document_id",
                            match=models.MatchValue(value=document_id),
                        )
                    ]
                )
            ),
        )
        self.logger.info(f"[VectorStore] Deleted vectors for document {document_id}")

    async def search(self, query_vector: List[float], limit: int = 25) -> List[models.ScoredPoint]:
        """
        Perform a vector similarity search on the collection.

        Args:
            query_vector (List[float]): The vector to query against the collection.
            limit (int, optional): The maximum number of results to return. Defaults to 25.

        Returns:
            List[models.ScoredPoint]: A list of scored points representing the search results.
        """

        res = await self.client.query_points(
            collection_name=self.collection_name,
            query=query_vector,
            limit=limit,
            with_payload=True,
        )
        return res.points
