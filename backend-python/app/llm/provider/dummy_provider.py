from typing import AsyncGenerator, Dict, List

from ..base import LLMProvider


class DummyProvider(LLMProvider):
    async def generate_response(
        self, query: str, context_docs: List[str], history: List[Dict[str, str]]
    ) -> AsyncGenerator[str, None]:
        yield (
            f"🤖 [DUMMY AI]: Received the question '{query}'.\n"
            f"📚 Number of Context Documents Used: {len(context_docs)}\n"
            "⚠️ No real model is connected. Please configure the LLM_PROVIDER setting."
        )

    @property
    def provider_name(self) -> str:
        return "dummy"
