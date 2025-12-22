from typing import AsyncGenerator, Dict, List

from google.genai import Client
from google.genai.types import GenerateContentConfig

from ..base import LLMProvider


class GeminiProvider(LLMProvider):
    def __init__(self, api_key: str, model: str, timeout: float) -> None:
        self.client = Client(api_key=api_key).aio
        self.model = model

    async def generate_response(
        self, query: str, context_docs: List[str], history: List[Dict[str, str]]
    ) -> AsyncGenerator[str, None]:
        messages = []
        if history:
            messages.extend(history)

        messages.append(super()._build_context_prompt(query, context_docs))

        try:
            response = await self.client.models.generate_content_stream(
                model=self.model,
                contents=messages,
                config=GenerateContentConfig(
                    system_instruction=super().DEFAULT_SYSTEM_PROMPT,
                    max_output_tokens=1024,
                    temperature=0.1,
                ),
            )

            if not response:
                raise ValueError("Response text is None")

            async for chunk in response:
                if chunk.text:
                    yield chunk.text

        except Exception as e:
            yield f"Error generating response (Gemini): {str(e)}"

    @property
    def provider_name(self) -> str:
        return "gemini"
