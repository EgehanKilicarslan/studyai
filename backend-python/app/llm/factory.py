import os

from .base import LLMProvider
from .dummy_provider import DummyProvider


def get_llm_provider() -> LLMProvider:
    provider_type = os.getenv("LLM_PROVIDER", "dummy").lower()

    print(f"🧠 [Factory] Selected LLM Provider: {provider_type}")

    if provider_type == "openai":
        # return OpenAIProvider()
        raise NotImplementedError("OpenAI has not been added yet.")

    elif provider_type == "gemini":
        # return GeminiProvider()
        raise NotImplementedError("Gemini has not been added yet.")

    elif provider_type == "colab":
        # Colab usually works through an API URL
        # return RemoteLLMProvider(url=os.getenv("COLAB_URL"))
        raise NotImplementedError("Colab connection has not been added yet.")

    else:
        return DummyProvider()
