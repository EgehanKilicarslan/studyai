from abc import ABC, abstractmethod
from typing import AsyncGenerator, Dict, List


class LLMProvider(ABC):
    """
    All LLM providers (OpenAI, Gemini, Local, Colab, etc.) should inherit from this base class.
    """

    DEFAULT_SYSTEM_PROMPT: str = (
        "You are a helpful and precise AI assistant. "
        "Your task is to answer the user's question based ONLY on the provided context. "
        "If the answer is not present in the context, state that you do not have enough information. "
        "Do not fabricate information or use outside knowledge unless explicitly asked."
    )

    def _build_context_prompt(self, query: str, context_docs: List[str]) -> str:
        """Shared prompt builder for all providers"""
        context_str = "\n\n---\n\n".join(context_docs)
        return (
            f"Please answer the question based on the following context:\n\n"
            f"CONTEXT:\n{context_str}\n\n"
            f"QUESTION: {query}"
        )

    @abstractmethod
    def generate_response(
        self, query: str, context_docs: List[str], history: List[Dict[str, str]]
    ) -> AsyncGenerator[str, None]:
        """
        Generate a response based on the given query, context documents, and conversation history.

        Args:
            query (str): The input query or question for which a response is to be generated.
            context_docs (List[str]): A list of context documents that provide additional information
                relevant to the query.
            history (List[Dict[str, str]]): A list of dictionaries representing the conversation history,
                where each dictionary contains keys such as 'user' and 'assistant' to track the dialogue.

        Returns:
            AsyncGenerator[str, None]: The generated response based on the input query, context, and history.
        """
        pass

    @property
    @abstractmethod
    def provider_name(self) -> str:
        """
        Returns the name of the provider as a string.

        This method should be implemented by subclasses to specify the name
        of the provider they represent.

        Returns:
            str: The name of the provider.
        """
        pass
