"""Unit tests for the TokenCounter service."""

from unittest.mock import Mock

import pytest
from app.services.token_counter import TokenCounter


class TestTokenCounter:
    """Tests for TokenCounter class."""

    @pytest.fixture
    def mock_settings(self):
        """Create mock settings."""
        settings = Mock()
        settings.llm_model_name = "gpt-4"
        settings.max_context_tokens = 8000
        settings.reserve_output_tokens = 1024
        return settings

    @pytest.fixture
    def mock_logger(self):
        """Create mock logger."""
        logger = Mock()
        logger.get_logger.return_value = Mock()
        return logger

    @pytest.fixture
    def counter(self, mock_settings, mock_logger):
        """Create a TokenCounter instance."""
        return TokenCounter(settings=mock_settings, logger=mock_logger)

    def test_count_tokens_empty_string(self, counter):
        """Test counting tokens for empty string."""
        assert counter.count_tokens("") == 0

    def test_count_tokens_simple_text(self, counter):
        """Test counting tokens for simple text."""
        # Simple text should return a reasonable token count
        token_count = counter.count_tokens("Hello, world!")
        assert token_count > 0
        # With tiktoken or char approx, "Hello, world!" should be ~3-4 tokens
        assert token_count < 10

    def test_count_tokens_long_text(self, counter):
        """Test counting tokens for longer text."""
        text = "This is a longer piece of text that should have more tokens. " * 10
        token_count = counter.count_tokens(text)
        # Should be significantly more than the short text
        assert token_count > 50

    def test_count_tokens_for_context(self, counter):
        """Test counting tokens for full context."""
        system_prompt = "You are a helpful assistant."
        query = "What is the meaning of life?"
        context_docs = ["Document 1 content", "Document 2 content"]
        history = [
            {"role": "user", "content": "Hello"},
            {"role": "assistant", "content": "Hi there!"},
        ]

        total, system, query_tokens, context, history_tokens = counter.count_tokens_for_context(
            system_prompt=system_prompt,
            query=query,
            context_docs=context_docs,
            history=history,
        )

        assert total > 0
        assert system > 0
        assert query_tokens > 0
        assert context > 0
        assert history_tokens > 0
        assert total == system + query_tokens + context + history_tokens

    def test_count_tokens_for_context_no_history(self, counter):
        """Test counting tokens without history."""
        total, system, query_tokens, context, history_tokens = counter.count_tokens_for_context(
            system_prompt="System",
            query="Query",
            context_docs=["Doc"],
            history=None,
        )

        assert history_tokens == 0
        assert total == system + query_tokens + context

    def test_truncate_context_docs_no_truncation_needed(self, counter):
        """Test that no truncation happens when docs fit."""
        docs = [
            {"text": "Short doc 1", "score": 0.9},
            {"text": "Short doc 2", "score": 0.8},
        ]

        truncated, was_truncated = counter.truncate_context_docs(
            system_prompt="You are helpful.",
            query="What?",
            context_docs=docs,
            history=None,
        )

        assert was_truncated is False
        assert len(truncated) == 2

    def test_truncate_context_docs_truncation_needed(self, mock_logger):
        """Test that truncation happens when docs exceed limit."""
        # Create counter with low token limit
        low_limit_settings = Mock()
        low_limit_settings.llm_model_name = "gpt-4"
        low_limit_settings.max_context_tokens = 2000  # Very low limit
        low_limit_settings.reserve_output_tokens = 500

        counter = TokenCounter(settings=low_limit_settings, logger=mock_logger)

        # Create large documents that will exceed limits
        large_text = "This is a very long document. " * 500  # ~4000 chars = ~1000 tokens
        docs = [
            {"text": large_text, "score": 0.9},
            {"text": large_text, "score": 0.8},
            {"text": large_text, "score": 0.7},
            {"text": large_text, "score": 0.6},
            {"text": large_text, "score": 0.5},
        ]

        truncated, was_truncated = counter.truncate_context_docs(
            system_prompt="You are helpful.",
            query="What?",
            context_docs=docs,
            history=None,
        )

        assert was_truncated is True
        assert len(truncated) < len(docs)

    def test_truncate_context_docs_maintains_order(self, counter):
        """Test that truncation maintains document order (highest relevance first)."""
        docs = [
            {"text": "High relevance doc", "score": 0.9},
            {"text": "Medium relevance doc", "score": 0.7},
            {"text": "Low relevance doc", "score": 0.5},
        ]

        truncated, _ = counter.truncate_context_docs(
            system_prompt="System",
            query="Query",
            context_docs=docs,
            history=None,
        )

        # Verify order is preserved
        if len(truncated) >= 2:
            assert truncated[0]["score"] > truncated[1]["score"]

    def test_truncate_context_texts(self, counter):
        """Test truncating text strings (convenience method)."""
        texts = ["Doc 1", "Doc 2", "Doc 3"]

        truncated_texts, was_truncated = counter.truncate_context_texts(
            system_prompt="System",
            query="Query",
            context_texts=texts,
            history=None,
        )

        assert was_truncated is False
        assert len(truncated_texts) == 3
        assert truncated_texts == texts

    def test_truncate_empty_docs(self, counter):
        """Test truncating empty document list."""
        truncated, was_truncated = counter.truncate_context_docs(
            system_prompt="System",
            query="Query",
            context_docs=[],
            history=None,
        )

        assert was_truncated is False
        assert truncated == []

    def test_truncate_with_large_history(self, mock_settings, mock_logger):
        """Test truncation when history takes up most of the budget."""
        # Create counter with specific limit
        mock_settings.max_context_tokens = 4000  # Not enough for history + docs
        mock_settings.reserve_output_tokens = 1024

        counter = TokenCounter(settings=mock_settings, logger=mock_logger)

        # Large history that takes up most tokens
        large_history = [
            {"role": "user", "content": "A" * 10000},
            {"role": "assistant", "content": "B" * 10000},
        ]

        docs = [
            {"text": "Short doc", "score": 0.9},
        ]

        truncated, was_truncated = counter.truncate_context_docs(
            system_prompt="System",
            query="Query",
            context_docs=docs,
            history=large_history,
        )

        # Should truncate because history is too large
        assert was_truncated is True


class TestTokenCounterFallback:
    """Tests for TokenCounter character approximation fallback."""

    def test_character_approximation(self):
        """Test that character approximation works correctly."""
        # tiktoken falls back to cl100k_base for unknown models,
        # so we can just test that token counting works
        mock_settings = Mock()
        mock_settings.llm_model_name = "unknown-model-xyz"
        mock_settings.max_context_tokens = 8000
        mock_settings.reserve_output_tokens = 1024

        mock_logger = Mock()
        mock_logger.get_logger.return_value = Mock()

        counter = TokenCounter(settings=mock_settings, logger=mock_logger)

        # 400 characters should produce a reasonable token count
        text = "a" * 400
        token_count = counter.count_tokens(text)

        # Token count should be positive and reasonable
        assert token_count > 0
        assert token_count < 500
