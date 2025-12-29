from typing import Any, Dict, List

from config import Settings
from flashrank import Ranker, RerankRequest
from logger import AppLogger


class RerankerService:
    def __init__(self, settings: Settings, logger: AppLogger):
        self.logger = logger.get_logger(__name__)

        self.logger.info(f"📦 [RerankerService] Loading model: {settings.reranker_model_name}")
        self.ranker = Ranker(
            model_name=settings.reranker_model_name, cache_dir="/home/appuser/.cache/models"
        )

    def rerank(
        self, query: str, documents: List[Dict[str, Any]], top_k: int = 5
    ) -> List[Dict[str, Any]]:
        if not documents:
            self.logger.warning("⚠️ [RerankerService] No documents to rerank.")
            return []

        self.logger.info(
            f"🔍 [RerankerService] Reranking {len(documents)} documents for query: '{query}'"
        )
        request = RerankRequest(query=query, passages=documents)
        results = self.ranker.rerank(request)
        return results[:top_k]
