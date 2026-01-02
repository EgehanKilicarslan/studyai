import uuid
from typing import List, Optional

from config import Settings
from logger import AppLogger
from pydantic import BaseModel
from qdrant_client import AsyncQdrantClient, QdrantClient, models

from .embedding_generator import EmbeddingGenerator


class CacheHit(BaseModel):
    """Represents a semantic cache hit result."""

    response_text: str
    score: float
    cache_id: str


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
        self.collection_name = settings.qdrant_docs_collection_name
        self.cache_collection_name = settings.qdrant_cache_collection_name
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

            # Ensure semantic cache collection exists
            if not sync_client.collection_exists(self.cache_collection_name):
                sync_client.create_collection(
                    collection_name=self.cache_collection_name,
                    vectors_config=models.VectorParams(
                        size=self.vector_size,
                        distance=models.Distance.COSINE,
                    ),
                )
                self.logger.info(
                    "✅ [VectorStore] Semantic cache collection created (Startup check)."
                )
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

    async def upsert_vectors_with_metadata(
        self,
        vectors: List[List[float]],
        chunk_ids: List[str],
        document_id: str,
        filename: str,
        organization_id: int,
        group_id: Optional[int] = None,
        owner_id: Optional[int] = None,
    ) -> int:
        """
        Upserts vectors into the vector store with multi-tenant metadata for filtering.

        The actual content is stored in PostgreSQL, Qdrant stores:
        - The vector embedding
        - chunk_id: Reference to the DocumentChunk in PostgreSQL
        - document_id: Reference to the Document (Go source of truth)
        - filename: For display purposes
        - organization_id: For multi-tenant filtering
        - group_id: For group-level filtering (None if org-wide)
        - owner_id: For owner-based filtering

        Args:
            vectors (List[List[float]]): A list of vectors to be upserted.
            chunk_ids (List[str]): A list of chunk IDs from the database.
            document_id (str): The document ID (UUID from Go).
            filename (str): The filename for display purposes.
            organization_id (int): Organization ID for multi-tenancy.
            group_id (Optional[int]): Group ID (None if org-wide document).
            owner_id (Optional[int]): Owner user ID.

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
                    "organization_id": organization_id,
                    "group_id": group_id,  # None for org-wide documents
                    "owner_id": owner_id,
                },
            )
            for vec, chunk_id in zip(vectors, chunk_ids)
        ]

        await self.client.upsert(collection_name=self.collection_name, points=points)
        self.logger.info(
            f"[VectorStore] Upserted {len(points)} vectors for document {document_id} "
            f"(org={organization_id}, group={group_id})"
        )
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

    async def search_with_tenant_filter(
        self,
        query_vector: List[float],
        organization_id: Optional[int] = None,
        group_ids: Optional[List[int]] = None,
        user_id: Optional[int] = None,
        limit: int = 25,
    ) -> List[models.ScoredPoint]:
        """
        Perform a tenant-scoped vector similarity search on the collection.

        Documents belong to groups, not organizations directly. Organizations are just
        containers for groups. Search priority:
        1. If group_ids provided: Search in those specific groups
        2. If no group_ids but user_id provided: Search user's personal documents (owner_id)

        Args:
            query_vector (List[float]): The vector to query against the collection.
            organization_id (Optional[int]): The organization ID (for logging/metadata only).
            group_ids (Optional[List[int]]): List of group IDs to search in.
            user_id (Optional[int]): The user ID for personal document filtering.
            limit (int, optional): The maximum number of results to return. Defaults to 25.

        Returns:
            List[models.ScoredPoint]: A list of scored points representing the search results.
        """
        filter_conditions: list[models.Condition] = []

        if group_ids:
            # Group-scoped search: documents belong to these groups
            filter_conditions.append(
                models.FieldCondition(
                    key="group_id",
                    match=models.MatchAny(any=group_ids),
                )
            )
            self.logger.info(
                f"[VectorStore] Group-scoped search: groups={group_ids}, org={organization_id}"
            )
        elif user_id is not None:
            # User-level search (personal documents only)
            filter_conditions.append(
                models.FieldCondition(
                    key="owner_id",
                    match=models.MatchValue(value=user_id),
                )
            )
            self.logger.info(f"[VectorStore] User-level search: user_id={user_id}")
        else:
            # No filtering context - return empty results for safety
            self.logger.warning(
                "[VectorStore] No group or user context provided, returning empty results"
            )
            return []

        query_filter = models.Filter(must=filter_conditions)

        res = await self.client.query_points(
            collection_name=self.collection_name,
            query=query_vector,
            query_filter=query_filter,
            limit=limit,
            with_payload=True,
        )
        return res.points

    # -------------------------------------------------------------------------
    # Semantic Cache Methods (Tenant-Aware)
    # -------------------------------------------------------------------------

    async def search_cache(
        self,
        query_vector: List[float],
        organization_id: Optional[int] = None,
        user_id: Optional[int] = None,
        group_ids: Optional[List[int]] = None,
        threshold: float = 0.95,
    ) -> Optional[CacheHit]:
        """
        Search for a cached response matching the query with tenant isolation.

        Supports three chat scopes:
        1. User scope: user_id only (personal queries, no org/groups)
        2. Group scope (no org): group_ids without organization_id
        3. Group under organization: organization_id + group_ids

        Cache isolation is enforced by:
        - organization_id: If provided, always filter by it for multi-tenant isolation
        - group_ids: If provided, cache is scoped to those groups
        - user_id: If no groups, cache is scoped to user's personal queries

        Args:
            query_vector: The embedding vector of the query to search for.
            organization_id: The organization ID for multi-tenant isolation (optional).
            user_id: The user ID for personal cache isolation.
            group_ids: The group IDs for group-level cache isolation.
            threshold: Minimum similarity score to consider a cache hit (default: 0.95).

        Returns:
            CacheHit if a sufficiently similar cached response is found, None otherwise.
        """
        try:
            # Build filter based on context - need at least one identifier
            filter_conditions: list[models.Condition] = []

            # Scope 3: Group under organization - filter by org_id
            if organization_id is not None:
                filter_conditions.append(
                    models.FieldCondition(
                        key="organization_id",
                        match=models.MatchValue(value=organization_id),
                    )
                )

            if group_ids:
                # Scope 2 & 3: Group-scoped cache
                filter_conditions.append(
                    models.FieldCondition(
                        key="group_ids",
                        match=models.MatchAny(any=group_ids),
                    )
                )
            elif user_id is not None:
                # Scope 1: User-scoped cache (personal queries)
                filter_conditions.append(
                    models.FieldCondition(
                        key="user_id",
                        match=models.MatchValue(value=user_id),
                    )
                )
            else:
                # No valid scope context - skip cache
                self.logger.debug("[VectorStore] No cache scope context provided, skipping cache")
                return None

            cache_filter = models.Filter(must=filter_conditions)

            # Search for similar queries
            results = await self.client.query_points(
                collection_name=self.cache_collection_name,
                query=query_vector,
                query_filter=cache_filter,
                limit=1,
                with_payload=True,
                score_threshold=threshold,
            )

            if results.points:
                hit = results.points[0]
                payload = hit.payload or {}
                response_text = payload.get("response_text", "")

                self.logger.info(
                    f"[VectorStore] Cache HIT (score={hit.score:.4f}, id={hit.id}, "
                    f"org={organization_id}, user={user_id}, groups={group_ids})"
                )

                return CacheHit(
                    response_text=response_text,
                    score=hit.score,
                    cache_id=str(hit.id),
                )

            self.logger.debug(
                f"[VectorStore] Cache MISS for org={organization_id}, user={user_id}, groups={group_ids}"
            )
            return None

        except Exception as e:
            self.logger.error(f"[VectorStore] Error searching cache: {e}")
            return None

    async def save_to_cache(
        self,
        query_vector: List[float],
        response_text: str,
        organization_id: Optional[int] = None,
        user_id: Optional[int] = None,
        group_ids: Optional[List[int]] = None,
    ) -> Optional[str]:
        """
        Save a response to the semantic cache with tenant metadata.

        Supports three chat scopes:
        1. User scope: user_id only (personal queries, no org/groups)
        2. Group scope (no org): group_ids without organization_id
        3. Group under organization: organization_id + group_ids

        Args:
            query_vector: The embedding vector of the query.
            response_text: The LLM-generated response to cache.
            organization_id: The organization ID for multi-tenant isolation (optional).
            user_id: The user ID for personal cache isolation.
            group_ids: The group IDs for group-level cache isolation.

        Returns:
            The cache entry ID if successful, None otherwise.
        """
        try:
            # Need at least one scope identifier to save
            if not group_ids and user_id is None:
                self.logger.debug("[VectorStore] No cache scope context, skipping cache save")
                return None

            cache_id = str(uuid.uuid4())

            payload: dict = {
                "response_text": response_text,
            }

            # Store organization_id if present (Scope 3)
            if organization_id is not None:
                payload["organization_id"] = organization_id

            # Store group_ids if present (Scope 2 & 3)
            if group_ids:
                payload["group_ids"] = group_ids

            # Store user_id if present (Scope 1)
            if user_id is not None:
                payload["user_id"] = user_id

            point = models.PointStruct(
                id=cache_id,
                vector=query_vector,
                payload=payload,
            )

            await self.client.upsert(
                collection_name=self.cache_collection_name,
                points=[point],
            )

            self.logger.info(
                f"[VectorStore] Saved cache entry (id={cache_id}, org={organization_id}, "
                f"user={user_id}, groups={group_ids})"
            )

            return cache_id

        except Exception as e:
            self.logger.error(f"[VectorStore] Error saving to cache: {e}")
            return None
