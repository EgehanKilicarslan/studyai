from typing import AsyncGenerator, Dict, List

from anthropic import AsyncAnthropic
from anthropic.types import TextDelta
from llm.base import LLMProvider


class AnthropicProvider(LLMProvider):
    def __init__(self, base_url: str | None, api_key: str, model: str, timeout: float) -> None:
        self.client = AsyncAnthropic(base_url=base_url, api_key=api_key, timeout=timeout)
        self.model = model

    async def generate_response(
        self, query: str, context_docs: List[str], history: List[Dict[str, str]]
    ) -> AsyncGenerator[str, None]:
        messages = []

        if history:
            messages.extend([{"role": h["role"], "content": h["content"]} for h in history])

        messages.append(
            {"role": "user", "content": self._build_context_prompt(query, context_docs)}
        )

        try:
            response = await self.client.messages.create(
                model=self.model,
                system=self.DEFAULT_SYSTEM_PROMPT,
                messages=messages,
                temperature=0.1,
                max_tokens=1024,
                stream=True,
            )

            if not response:
                raise ValueError("Response text is None")

            # Buffer to accumulate content chunks
            buffer = ""
            # Flag to track if we're currently inside thinking tags
            is_thinking = False

            # Tags that mark the start of thinking sections (content to hide)
            START_TAGS = ["<think>", "<thinking>"]
            # Tags that mark the end of thinking sections
            END_TAGS = ["</think>", "</thinking>"]

            async for chunk in response:
                # Handle different chunk types from Anthropic
                if chunk.type == "content_block_delta":
                    if isinstance(chunk.delta, TextDelta):
                        content = chunk.delta.text
                    else:
                        continue
                else:
                    continue

                if not content:
                    continue

                # Add new content to buffer
                buffer += content

                # If we're inside thinking tags, look for end tag
                if is_thinking:
                    for tag in END_TAGS:
                        if tag in buffer:
                            # Split at end tag and keep only content after it
                            _, remaining = buffer.split(tag, 1)
                            buffer = remaining
                            is_thinking = False
                            break

                    # Skip yielding content while thinking
                    continue

                else:
                    # Check if a start tag appears in buffer
                    found_start_tag = False
                    for tag in START_TAGS:
                        if tag in buffer:
                            # Split at start tag: yield content before, save content after
                            pre_tag, post_tag = buffer.split(tag, 1)
                            if pre_tag:
                                yield pre_tag

                            buffer = post_tag
                            is_thinking = True
                            found_start_tag = True
                            break

                    if found_start_tag:
                        continue

                    # Handle partial tags: if "<" appears, it might be an incomplete tag
                    if "<" in buffer:
                        last_open_pos = buffer.rfind("<")

                        # Extract potential incomplete tag from last "<"
                        potential_tag = buffer[last_open_pos:]

                        # Check if it's a partial match for any start tag
                        is_partial_tag = False
                        for tag in START_TAGS:
                            if tag.startswith(potential_tag) and potential_tag != tag:
                                is_partial_tag = True
                                break

                        # If partial tag detected, yield safe part and keep potential tag
                        if is_partial_tag:
                            safe_part = buffer[:last_open_pos]
                            if safe_part:
                                yield safe_part
                                buffer = buffer[last_open_pos:]
                            continue

                    # No tags detected: yield entire buffer
                    yield buffer
                    buffer = ""

            # After stream ends, yield any remaining buffered content (if not thinking)
            if buffer and not is_thinking:
                yield buffer

        except Exception as e:
            yield f"Error generating response (Anthropic): {str(e)}"

    @property
    def provider_name(self) -> str:
        return "anthropic"
