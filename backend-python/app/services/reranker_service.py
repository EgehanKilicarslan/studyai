from typing import Any, Dict, List

from config import Settings
from flashrank import Ranker, RerankRequest
from logger import AppLogger


class RerankerService:
    """
    A service for reranking documents based on a given query using a specified ranking model.

    Attributes:
        logger (Logger): The logger instance for logging messages.
        ranker (Ranker): The ranking model used for reranking documents.

    Methods:
        __init__(settings: Settings, logger: AppLogger):
            Initializes the RerankerService with the specified settings and logger.

        rerank(query: str, documents: List[Dict[str, Any]], top_k: int = 5) -> List[Dict[str, Any]]:
            Reranks a list of documents based on their relevance to the given query and returns the top-k results.
    """

    def __init__(self, settings: Settings, logger: AppLogger):
        """
        Initializes the RerankerService with the specified settings and logger.

        Args:
            settings (Settings): The configuration settings containing the model name
                for the reranker and other related parameters.
            logger (AppLogger): The application logger instance used for logging
                messages and events.

        Attributes:
            logger (Logger): The logger instance for this service, scoped to the
                current module.
            ranker (Ranker): The Ranker instance initialized with the specified
                model name and cache directory.
        """

        self.logger = logger.get_logger(__name__)

        self.logger.info(f"📦 [RerankerService] Loading model: {settings.reranker_model_name}")
        self.ranker = Ranker(
            model_name=settings.reranker_model_name, cache_dir="/home/appuser/.cache/models"
        )

    def rerank(
        self, query: str, documents: List[Dict[str, Any]], top_k: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Rerank a list of documents based on their relevance to a given query.

        Args:
            query (str): The search query to rank the documents against.
            documents (List[Dict[str, Any]]): A list of documents to be reranked. Each document is represented as a dictionary.
            top_k (int, optional): The maximum number of top-ranked documents to return. Defaults to 5.

        Returns:
            List[Dict[str, Any]]: A list of the top-k reranked documents, sorted by relevance.

        Logs:
            - A warning if the input document list is empty.
            - An info message indicating the number of documents being reranked and the query used.
        """

        if not documents:
            self.logger.warning("⚠️ [RerankerService] No documents to rerank.")
            return []

        self.logger.info(
            f"🔍 [RerankerService] Reranking {len(documents)} documents for query: '{query}'"
        )
        request = RerankRequest(query=query, passages=documents)
        results = self.ranker.rerank(request)
        return results[:top_k]
