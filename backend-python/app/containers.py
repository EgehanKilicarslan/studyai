from dependency_injector import containers, providers

from .config import settings
from .llm import get_llm_provider
from .services import EmbeddingService, RagService


class Container(containers.DeclarativeContainer):
    config = providers.Object(settings)

    llm_client = providers.Factory(get_llm_provider, settings=config)

    embedding_service = providers.Factory(EmbeddingService, settings=config)

    rag_service = providers.Factory(
        RagService, llm_provider=llm_client, embedding_service=embedding_service
    )
