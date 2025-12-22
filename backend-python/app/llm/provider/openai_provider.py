from typing import AsyncGenerator, Dict, List, cast

from openai import AsyncOpenAI
from openai.types.chat import ChatCompletionMessageParam

from ..base import LLMProvider


class OpenAIProvider(LLMProvider):
    def __init__(self, api_key: str, model: str, timeout: float) -> None:
        self.client = AsyncOpenAI(api_key=api_key, timeout=timeout)
        self.model = model

    async def generate_response(
        self, query: str, context_docs: List[str], history: List[Dict[str, str]]
    ) -> AsyncGenerator[str, None]:
        system_prompt = (
            "You are a helpful and precise AI assistant. "
            "Your task is to answer the user's question based ONLY on the provided context. "
            "If the answer is not present in the context, state that you do not have enough information. "
            "Do not fabricate information or use outside knowledge unless explicitly asked."
        )

        messages: List[ChatCompletionMessageParam] = [{"role": "system", "content": system_prompt}]

        if history:
            messages.extend(
                cast(
                    List[ChatCompletionMessageParam],
                    [{"role": h["role"], "content": h["content"]} for h in history],
                )
            )

        messages.append(
            {"role": "user", "content": super()._build_context_prompt(query, context_docs)}
        )

        try:
            response = await self.client.chat.completions.create(
                model=self.model,
                messages=messages,
                temperature=0.1,
                max_tokens=1024,
                stream=True,
            )

            if not response:
                raise ValueError("Response text is None")

            async for chunk in response:
                content = chunk.choices[0].delta.content
                if content:
                    yield content

        except Exception as e:
            yield f"Error generating response (OpenAI): {str(e)}"

    @property
    def provider_name(self) -> str:
        return "openai"
