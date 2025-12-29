from config import settings
from dependency_injector import containers, providers
from llm import get_llm_provider
from logger import AppLogger
from services import DocumentParser, EmbeddingGenerator, RagService, RerankerService, VectorStore


class Container(containers.DeclarativeContainer):
    config = providers.Object(settings)

    app_logger = providers.Singleton(AppLogger, settings=config)

    llm_client = providers.Factory(get_llm_provider, settings=config, logger=app_logger)

    document_parser = providers.Factory(DocumentParser, settings=config, logger=app_logger)

    embedding_generator = providers.Factory(EmbeddingGenerator, settings=config, logger=app_logger)

    reranker_service = providers.Factory(RerankerService, settings=config, logger=app_logger)

    vector_store = providers.Factory(
        VectorStore, settings=config, logger=app_logger, embedding_generator=embedding_generator
    )

    rag_service = providers.Factory(
        RagService,
        settings=config,
        logger=app_logger,
        llm_provider=llm_client,
        vector_store=vector_store,
        embedder=embedding_generator,
        reranker=reranker_service,
        parser=document_parser,
    )
