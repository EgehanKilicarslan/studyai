from abc import ABC, abstractmethod
from typing import Dict, List


class LLMProvider(ABC):
    """
    All LLM providers (OpenAI, Gemini, Local, Colab, etc.) should inherit from this base class.
    """

    @abstractmethod
    def generate_response(
        self, query: str, context_docs: List[str], history: List[Dict[str, str]]
    ) -> str:
        """
        Generate a response based on the given query, context documents, and conversation history.

        Args:
            query (str): The input query or question for which a response is to be generated.
            context_docs (List[str]): A list of context documents that provide additional information
                relevant to the query.
            history (List[Dict[str, str]]): A list of dictionaries representing the conversation history,
                where each dictionary contains keys such as 'user' and 'assistant' to track the dialogue.

        Returns:
            str: The generated response based on the input query, context, and history.
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
