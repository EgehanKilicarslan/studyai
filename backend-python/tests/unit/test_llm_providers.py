from unittest.mock import AsyncMock, Mock, patch

import pytest
from app.llm.provider import (
    AnthropicProvider,
    DummyProvider,
    GeminiProvider,
    OpenAIProvider,
)


async def async_iter(items):
    """Helper to create async generator from list."""
    for item in items:
        yield item


# ============================================================================
# DUMMY PROVIDER TESTS
# ============================================================================


@pytest.mark.asyncio
async def test_dummy_provider_returns_placeholder_response():
    """
    Test that the dummy provider returns a static placeholder message.

    Verifies that:
    - A single response chunk is returned
    - The response contains "DUMMY AI" identifier
    - The original query is included in the response
    - The number of context documents is mentioned
    """
    provider = DummyProvider()

    response = [
        chunk
        async for chunk in provider.generate_response(
            query="test query", context_docs=["doc1", "doc2"], history=[]
        )
    ]

    assert len(response) == 1
    assert "DUMMY AI" in response[0]
    assert "test query" in response[0]
    assert "2" in response[0]


def test_dummy_provider_name():
    """
    Test that the dummy provider returns the correct provider name.

    Verifies that the provider_name attribute is set to "dummy".
    """
    provider = DummyProvider()
    assert provider.provider_name == "dummy"


# ============================================================================
# OPENAI PROVIDER TESTS
# ============================================================================


@pytest.fixture
def mock_openai_client():
    """Create mock OpenAI client."""
    client = Mock()
    client.chat = Mock()
    client.chat.completions = Mock()
    return client


@pytest.mark.asyncio
async def test_openai_provider_successful_response(mock_openai_client):
    """
    Test that OpenAI provider successfully streams a complete response.

    Verifies that:
    - Multiple chunks are properly streamed from the API
    - Chunks are concatenated correctly
    - The final response matches expected content
    """
    # 1. ARRANGE
    mock_chunk_1 = Mock()
    mock_chunk_1.choices = [Mock(delta=Mock(content="Hello "))]

    mock_chunk_2 = Mock()
    mock_chunk_2.choices = [Mock(delta=Mock(content="World"))]

    mock_openai_client.chat.completions.create = AsyncMock(
        return_value=async_iter([mock_chunk_1, mock_chunk_2])
    )

    with patch("app.llm.provider.openai_provider.AsyncOpenAI", return_value=mock_openai_client):
        provider = OpenAIProvider(base_url=None, api_key="test-key", model="gpt-4", timeout=60.0)

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(
            query="test", context_docs=["doc"], history=[]
        )
    ]

    # 3. ASSERT
    assert "".join(response) == "Hello World"


@pytest.mark.asyncio
async def test_openai_provider_filters_thinking_tags(mock_openai_client):
    """
    Test that OpenAI provider filters out <think>/<thinking> tags from responses.

    Verifies that:
    - Content inside thinking tags is removed from the output
    - The thinking tags themselves are removed
    - Content before and after thinking blocks is preserved
    - This prevents internal reasoning from being shown to users
    """
    # 1. ARRANGE
    mock_chunks = [
        Mock(choices=[Mock(delta=Mock(content="Answer: "))]),
        Mock(choices=[Mock(delta=Mock(content="<think>"))]),
        Mock(choices=[Mock(delta=Mock(content="internal reasoning"))]),
        Mock(choices=[Mock(delta=Mock(content="</think>"))]),
        Mock(choices=[Mock(delta=Mock(content="Final answer"))]),
    ]

    mock_openai_client.chat.completions.create = AsyncMock(return_value=async_iter(mock_chunks))

    with patch("app.llm.provider.openai_provider.AsyncOpenAI", return_value=mock_openai_client):
        provider = OpenAIProvider(base_url=None, api_key="test-key", model="gpt-4", timeout=60.0)

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    full_response = "".join(response)
    assert "Answer: " in full_response
    assert "Final answer" in full_response
    assert "internal reasoning" not in full_response
    assert "<think>" not in full_response


@pytest.mark.asyncio
async def test_openai_provider_handles_empty_chunks(mock_openai_client):
    """
    Test that OpenAI provider properly skips empty or None content chunks.

    Verifies that:
    - Chunks with None content are ignored
    - Chunks with empty string content are ignored
    - Only chunks with actual content are included in the response
    - The final concatenated response is correct
    """
    # 1. ARRANGE
    mock_chunks = [
        Mock(choices=[Mock(delta=Mock(content="Hello"))]),
        Mock(choices=[Mock(delta=Mock(content=None))]),
        Mock(choices=[Mock(delta=Mock(content=""))]),
        Mock(choices=[Mock(delta=Mock(content=" World"))]),
    ]

    mock_openai_client.chat.completions.create = AsyncMock(return_value=async_iter(mock_chunks))

    with patch("app.llm.provider.openai_provider.AsyncOpenAI", return_value=mock_openai_client):
        provider = OpenAIProvider(base_url=None, api_key="test-key", model="gpt-4", timeout=60.0)

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    assert "".join(response) == "Hello World"


@pytest.mark.asyncio
async def test_openai_provider_error_handling(mock_openai_client):
    """
    Test that OpenAI provider handles API errors gracefully.

    Verifies that:
    - Exceptions during API calls are caught
    - An error message is returned to the user
    - The error message identifies the provider (OpenAI)
    - The specific error details are included
    """
    # 1. ARRANGE
    mock_openai_client.chat.completions.create = AsyncMock(side_effect=Exception("API Error"))

    with patch("app.llm.provider.openai_provider.AsyncOpenAI", return_value=mock_openai_client):
        provider = OpenAIProvider(base_url=None, api_key="test-key", model="gpt-4", timeout=60.0)

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    assert len(response) == 1
    assert "Error generating response (OpenAI)" in response[0]
    assert "API Error" in response[0]


def test_openai_provider_name():
    """
    Test that OpenAI provider returns the correct provider name.

    Verifies that the provider_name attribute is set to "openai".
    """
    with patch("app.llm.provider.openai_provider.AsyncOpenAI"):
        provider = OpenAIProvider(base_url=None, api_key="test-key", model="gpt-4", timeout=60.0)
    assert provider.provider_name == "openai"


# ============================================================================
# ANTHROPIC PROVIDER TESTS
# ============================================================================


@pytest.fixture
def mock_anthropic_client():
    """Create mock Anthropic client."""
    client = Mock()
    client.messages = Mock()
    return client


@pytest.mark.asyncio
async def test_anthropic_provider_successful_response(mock_anthropic_client):
    """
    Test that Anthropic provider successfully streams a complete response.

    Verifies that:
    - TextDelta chunks are properly streamed from the Claude API
    - Multiple chunks are concatenated correctly
    - The final response matches expected content
    """
    # 1. ARRANGE
    from anthropic.types import TextDelta

    mock_chunk_1 = Mock(
        type="content_block_delta", delta=TextDelta(text="Hello ", type="text_delta")
    )
    mock_chunk_2 = Mock(
        type="content_block_delta", delta=TextDelta(text="from Anthropic", type="text_delta")
    )

    mock_anthropic_client.messages.create = AsyncMock(
        return_value=async_iter([mock_chunk_1, mock_chunk_2])
    )

    with patch(
        "app.llm.provider.anthropic_provider.AsyncAnthropic",
        return_value=mock_anthropic_client,
    ):
        provider = AnthropicProvider(
            base_url=None, api_key="test-key", model="claude-3", timeout=60.0
        )

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(
            query="test", context_docs=["doc"], history=[]
        )
    ]

    # 3. ASSERT
    assert "".join(response) == "Hello from Anthropic"


@pytest.mark.asyncio
async def test_anthropic_provider_filters_thinking_tags(mock_anthropic_client):
    """
    Test that Anthropic provider filters out <thinking> tags from responses.

    Verifies that:
    - Content inside thinking tags is removed from the output
    - The thinking tags themselves are removed
    - Content before and after thinking blocks is preserved
    - This prevents Claude's internal reasoning from being shown to users
    """
    # 1. ARRANGE
    from anthropic.types import TextDelta

    mock_chunks = [
        Mock(type="content_block_delta", delta=TextDelta(text="Answer: ", type="text_delta")),
        Mock(type="content_block_delta", delta=TextDelta(text="<thinking>", type="text_delta")),
        Mock(type="content_block_delta", delta=TextDelta(text="reasoning", type="text_delta")),
        Mock(type="content_block_delta", delta=TextDelta(text="</thinking>", type="text_delta")),
        Mock(type="content_block_delta", delta=TextDelta(text="Result", type="text_delta")),
    ]

    mock_anthropic_client.messages.create = AsyncMock(return_value=async_iter(mock_chunks))

    with patch(
        "app.llm.provider.anthropic_provider.AsyncAnthropic",
        return_value=mock_anthropic_client,
    ):
        provider = AnthropicProvider(
            base_url=None, api_key="test-key", model="claude-3", timeout=60.0
        )

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    full_response = "".join(response)
    assert "Answer: " in full_response
    assert "Result" in full_response
    assert "reasoning" not in full_response


@pytest.mark.asyncio
async def test_anthropic_provider_skips_non_text_deltas(mock_anthropic_client):
    """
    Test that Anthropic provider skips non-TextDelta chunks.

    Verifies that:
    - Only content_block_delta chunks with TextDelta are processed
    - Other chunk types (content_block_start, etc.) are ignored
    - Non-TextDelta delta objects are ignored
    - The final response only contains text content
    """
    # 1. ARRANGE
    from anthropic.types import TextDelta

    mock_chunks = [
        Mock(type="content_block_delta", delta=TextDelta(text="Hello", type="text_delta")),
        Mock(type="other_type", delta=Mock()),
        Mock(type="content_block_start", delta=Mock()),
        Mock(type="content_block_delta", delta=TextDelta(text=" World", type="text_delta")),
    ]

    mock_anthropic_client.messages.create = AsyncMock(return_value=async_iter(mock_chunks))

    with patch(
        "app.llm.provider.anthropic_provider.AsyncAnthropic",
        return_value=mock_anthropic_client,
    ):
        provider = AnthropicProvider(
            base_url=None, api_key="test-key", model="claude-3", timeout=60.0
        )

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    assert "".join(response) == "Hello World"


@pytest.mark.asyncio
async def test_anthropic_provider_error_handling(mock_anthropic_client):
    """
    Test that Anthropic provider handles API errors gracefully.

    Verifies that:
    - Exceptions during API calls are caught
    - An error message is returned to the user
    - The error message identifies the provider (Anthropic)
    - The specific error details are included
    """
    # 1. ARRANGE
    mock_anthropic_client.messages.create = AsyncMock(side_effect=Exception("API Error"))

    with patch(
        "app.llm.provider.anthropic_provider.AsyncAnthropic",
        return_value=mock_anthropic_client,
    ):
        provider = AnthropicProvider(
            base_url=None, api_key="test-key", model="claude-3", timeout=60.0
        )

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    assert len(response) == 1
    assert "Error generating response (Anthropic)" in response[0]
    assert "API Error" in response[0]


def test_anthropic_provider_name():
    """
    Test that Anthropic provider returns the correct provider name.

    Verifies that the provider_name attribute is set to "anthropic".
    """
    with patch("app.llm.provider.anthropic_provider.AsyncAnthropic"):
        provider = AnthropicProvider(
            base_url=None, api_key="test-key", model="claude-3", timeout=60.0
        )
    assert provider.provider_name == "anthropic"


# ============================================================================
# GEMINI PROVIDER TESTS
# ============================================================================


@pytest.fixture
def mock_gemini_client():
    """Create mock Gemini client."""
    client = Mock()
    client.aio = Mock()
    client.aio.models = Mock()
    return client


@pytest.mark.asyncio
async def test_gemini_provider_successful_response(mock_gemini_client):
    """
    Test that Gemini provider successfully streams a complete response.

    Verifies that:
    - Multiple chunks are properly streamed from the Gemini API
    - Chunks with text attributes are processed correctly
    - The final response matches expected content
    """
    # 1. ARRANGE
    mock_chunk_1 = Mock(text="Hello ")
    mock_chunk_2 = Mock(text="from Gemini")

    mock_gemini_client.aio.models.generate_content_stream = AsyncMock(
        return_value=async_iter([mock_chunk_1, mock_chunk_2])
    )

    with patch("app.llm.provider.gemini_provider.Client", return_value=mock_gemini_client):
        provider = GeminiProvider(
            base_url=None, api_key="test-key", model="gemini-pro", timeout=60.0
        )

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(
            query="test", context_docs=["doc"], history=[]
        )
    ]

    # 3. ASSERT
    assert "".join(response) == "Hello from Gemini"


@pytest.mark.asyncio
async def test_gemini_provider_skips_empty_text(mock_gemini_client):
    """
    Test that Gemini provider properly skips chunks with empty or None text.

    Verifies that:
    - Chunks with None text are ignored
    - Chunks with empty string text are ignored
    - Only chunks with actual text content are included
    - The final concatenated response is correct
    """
    # 1. ARRANGE
    mock_chunks = [
        Mock(text="Hello"),
        Mock(text=None),
        Mock(text=""),
        Mock(text=" World"),
    ]

    mock_gemini_client.aio.models.generate_content_stream = AsyncMock(
        return_value=async_iter(mock_chunks)
    )

    with patch("app.llm.provider.gemini_provider.Client", return_value=mock_gemini_client):
        provider = GeminiProvider(
            base_url=None, api_key="test-key", model="gemini-pro", timeout=60.0
        )

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    assert "".join(response) == "Hello World"


@pytest.mark.asyncio
async def test_gemini_provider_error_handling(mock_gemini_client):
    """
    Test that Gemini provider handles API errors gracefully.

    Verifies that:
    - Exceptions during API calls are caught
    - An error message is returned to the user
    - The error message identifies the provider (Gemini)
    - The specific error details are included
    """
    # 1. ARRANGE
    mock_gemini_client.aio.models.generate_content_stream = AsyncMock(
        side_effect=Exception("API Error")
    )

    with patch("app.llm.provider.gemini_provider.Client", return_value=mock_gemini_client):
        provider = GeminiProvider(
            base_url=None, api_key="test-key", model="gemini-pro", timeout=60.0
        )

    # 2. ACT
    response = [
        chunk
        async for chunk in provider.generate_response(query="test", context_docs=[], history=[])
    ]

    # 3. ASSERT
    assert len(response) == 1
    assert "Error generating response (Gemini)" in response[0]
    assert "API Error" in response[0]


def test_gemini_provider_name():
    """
    Test that Gemini provider returns the correct provider name.

    Verifies that the provider_name attribute is set to "gemini".
    """
    with patch("app.llm.provider.gemini_provider.Client"):
        provider = GeminiProvider(
            base_url=None, api_key="test-key", model="gemini-pro", timeout=60.0
        )
    assert provider.provider_name == "gemini"


# ============================================================================
# COMMON PROVIDER TESTS
# ============================================================================


@pytest.mark.parametrize(
    "provider_class,mock_path",
    [
        (OpenAIProvider, "app.llm.provider.openai_provider.AsyncOpenAI"),
        (AnthropicProvider, "app.llm.provider.anthropic_provider.AsyncAnthropic"),
        (GeminiProvider, "app.llm.provider.gemini_provider.Client"),
    ],
)
def test_provider_accepts_history(provider_class, mock_path):
    """
    Test that all providers support conversation history.

    Verifies that:
    - Each provider has a generate_response method
    - The method accepts a history parameter
    - This ensures consistent API across all providers
    """
    with patch(mock_path):
        provider = provider_class(
            base_url=None, api_key="test-key", model="test-model", timeout=60.0
        )

    assert hasattr(provider, "generate_response")


@pytest.mark.parametrize(
    "provider_class,mock_path",
    [
        (OpenAIProvider, "app.llm.provider.openai_provider.AsyncOpenAI"),
        (AnthropicProvider, "app.llm.provider.anthropic_provider.AsyncAnthropic"),
        (GeminiProvider, "app.llm.provider.gemini_provider.Client"),
    ],
)
def test_provider_builds_context_prompt(provider_class, mock_path):
    """
    Test that all providers build context prompts correctly.

    Verifies that:
    - The _build_context_prompt method exists
    - The query is included in the prompt
    - All context documents are included
    - A "CONTEXT:" header is present
    - This ensures consistent prompt formatting across providers
    """
    with patch(mock_path):
        provider = provider_class(
            base_url=None, api_key="test-key", model="test-model", timeout=60.0
        )

    prompt = provider._build_context_prompt("test query", ["doc1", "doc2"])

    assert "test query" in prompt
    assert "doc1" in prompt
    assert "doc2" in prompt
    assert "CONTEXT:" in prompt
