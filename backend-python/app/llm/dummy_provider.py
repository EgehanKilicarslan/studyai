from typing import Dict, List

from .base import LLMProvider


class DummyProvider(LLMProvider):
    def generate_response(
        self, query: str, context_docs: List[str], history: List[Dict[str, str]]
    ) -> str:
        return (
            f"🤖 [DUMMY AI]: Received the question '{query}'.\n"
            f"📚 Number of Context Documents Used: {len(context_docs)}\n"
            "⚠️ No real model is connected. Please configure the LLM_PROVIDER setting."
        )

    @property
    def provider_name(self) -> str:
        return "dummy"
