from config import settings
from dependency_injector import containers, providers
from llm import get_llm_provider
from services import DocumentParser, EmbeddingGenerator, RagService, RerankerService, VectorStore


class Container(containers.DeclarativeContainer):
    config = providers.Object(settings)

    llm_client = providers.Factory(get_llm_provider, settings=config)

    document_parser = providers.Factory(DocumentParser, settings=config)

    embedding_generator = providers.Factory(EmbeddingGenerator, settings=config)

    reranker_service = providers.Factory(RerankerService, settings=config)

    vector_store = providers.Factory(
        VectorStore, settings=config, embedding_generator=embedding_generator
    )

    rag_service = providers.Factory(
        RagService,
        settings=config,
        llm_provider=llm_client,
        vector_store=vector_store,
        embedder=embedding_generator,
        reranker=reranker_service,
        parser=document_parser,
    )
