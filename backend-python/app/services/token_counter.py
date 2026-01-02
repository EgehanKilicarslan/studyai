"""
Token counting and context truncation utilities for LLM calls.

This module provides token estimation and context truncation to prevent
LLM crashes due to context length limits.
"""

from typing import List, Tuple

import tiktoken
from config import Settings
from logger import AppLogger


class TokenCounter:
    """
    Token counter with support for tiktoken (OpenAI models) and character approximation fallback.

    The counter provides:
    - Accurate token counting using tiktoken for OpenAI models
    - Character-based approximation (1 token ≈ 4 chars) as fallback
    - Context truncation to fit within model limits
    """

    # Characters per token approximation (industry standard)
    CHARS_PER_TOKEN = 4

    def __init__(self, settings: Settings, logger: AppLogger) -> None:
        """
        Initialize the token counter.

        Args:
            settings: Application settings with model configuration.
            logger: Application logger for warnings and info.
        """
        self.model_name = settings.llm_model_name
        self.logger = logger.get_logger(__name__)
        self.max_context_tokens = settings.max_context_tokens
        self.reserve_output_tokens = settings.reserve_output_tokens
        self._encoding = None

        try:
            self._encoding = tiktoken.encoding_for_model(self.model_name)
        except KeyError:
            # Fall back to cl100k_base for unknown models
            self._encoding = tiktoken.get_encoding("cl100k_base")

    def count_tokens(self, text: str) -> int:
        """
        Count the number of tokens in the given text.

        Args:
            text: The text to count tokens for.

        Returns:
            Estimated number of tokens.
        """
        if not text:
            return 0

        if self._encoding:
            return len(self._encoding.encode(text))

        # Character approximation fallback
        return len(text) // self.CHARS_PER_TOKEN

    def count_tokens_for_context(
        self,
        system_prompt: str,
        query: str,
        context_docs: List[str],
        history: List[dict] | None = None,
    ) -> Tuple[int, int, int, int, int]:
        """
        Count tokens for all components of an LLM request.

        Args:
            system_prompt: The system prompt text.
            query: The user's query.
            context_docs: List of context document strings.
            history: Optional chat history list.

        Returns:
            Tuple of (total_tokens, system_tokens, query_tokens, context_tokens, history_tokens)
        """
        system_tokens = self.count_tokens(system_prompt)
        query_tokens = self.count_tokens(query)

        # Count context document tokens
        context_tokens = sum(self.count_tokens(doc) for doc in context_docs)

        # Count history tokens
        history_tokens = 0
        if history:
            for msg in history:
                history_tokens += self.count_tokens(msg.get("content", ""))

        total_tokens = system_tokens + query_tokens + context_tokens + history_tokens

        return total_tokens, system_tokens, query_tokens, context_tokens, history_tokens

    def truncate_context_docs(
        self,
        system_prompt: str,
        query: str,
        context_docs: List[dict],
        history: List[dict] | None = None,
    ) -> Tuple[List[dict], bool]:
        """
        Truncate context documents to fit within token limits.

        Documents are removed starting from the lowest scored ones (end of list)
        since they are typically sorted by relevance score in descending order.

        Args:
            system_prompt: The system prompt text.
            query: The user's query.
            context_docs: List of context document dicts with 'text' and optionally 'score' keys.
                         Expected to be sorted by relevance score (highest first).
            history: Optional chat history list.
            max_tokens: Maximum total tokens allowed for the model.
            reserve_output_tokens: Tokens to reserve for model output.

        Returns:
            Tuple of (truncated_docs, was_truncated) where:
            - truncated_docs: List of document dicts that fit within limits
            - was_truncated: True if any documents were removed
        """
        if not context_docs:
            return [], False

        # Calculate available tokens for context
        system_tokens = self.count_tokens(system_prompt)
        query_tokens = self.count_tokens(query)

        history_tokens = 0
        if history:
            for msg in history:
                history_tokens += self.count_tokens(msg.get("content", ""))

        # Account for prompt formatting overhead (~50 tokens)
        formatting_overhead = 50
        available_tokens = (
            self.max_context_tokens
            - system_tokens
            - query_tokens
            - history_tokens
            - self.reserve_output_tokens
            - formatting_overhead
        )

        if available_tokens <= 0:
            self.logger.warning(
                f"No tokens available for context after system prompt ({system_tokens}), "
                f"query ({query_tokens}), and history ({history_tokens})"
            )
            return [], True

        # Calculate token count for each document
        doc_tokens = [(doc, self.count_tokens(doc.get("text", ""))) for doc in context_docs]

        # Greedily select documents that fit (maintain original order - highest relevance first)
        selected_docs = []
        used_tokens = 0
        was_truncated = False

        for doc, tokens in doc_tokens:
            if used_tokens + tokens <= available_tokens:
                selected_docs.append(doc)
                used_tokens += tokens
            else:
                was_truncated = True
                # Continue checking smaller docs that might fit
                continue

        if was_truncated:
            original_count = len(context_docs)
            selected_count = len(selected_docs)
            self.logger.warning(
                f"Context truncated: {original_count} -> {selected_count} documents "
                f"({used_tokens}/{available_tokens} tokens used)"
            )

        return selected_docs, was_truncated

    def truncate_context_texts(
        self,
        system_prompt: str,
        query: str,
        context_texts: List[str],
        history: List[dict] | None = None,
    ) -> Tuple[List[str], bool]:
        """
        Truncate context text strings to fit within token limits.

        This is a convenience method for when you only have text strings
        without metadata. Documents are removed from the end of the list
        (assumed to be lower relevance).

        Args:
            system_prompt: The system prompt text.
            query: The user's query.
            context_texts: List of context text strings, ordered by relevance.
            history: Optional chat history list.
            max_tokens: Maximum total tokens allowed for the model.
            reserve_output_tokens: Tokens to reserve for model output.

        Returns:
            Tuple of (truncated_texts, was_truncated)
        """
        # Convert texts to doc format
        docs = [{"text": text} for text in context_texts]

        truncated_docs, was_truncated = self.truncate_context_docs(
            system_prompt=system_prompt,
            query=query,
            context_docs=docs,
            history=history,
        )

        return [doc["text"] for doc in truncated_docs], was_truncated
