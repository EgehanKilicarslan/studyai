from typing import Any, Dict, List

from config import Settings
from flashrank import Ranker, RerankRequest


class RerankerService:
    def __init__(self, settings: Settings):
        print(f"📦 [RerankerService] Loading model: {settings.reranker_model_name}")
        self.ranker = Ranker(
            model_name=settings.reranker_model_name, cache_dir="/home/appuser/.cache/models"
        )

    def rerank(
        self, query: str, documents: List[Dict[str, Any]], top_k: int = 5
    ) -> List[Dict[str, Any]]:
        if not documents:
            print("⚠️ [RerankerService] No documents to rerank.")
            return []

        print(f"🔍 [RerankerService] Reranking {len(documents)} documents for query: '{query}'")
        request = RerankRequest(query=query, passages=documents)
        results = self.ranker.rerank(request)
        return results[:top_k]
