from config import settings
from database import Database
from database.repositories import DocumentChunkRepository
from database.service import ChunkService
from dependency_injector import containers, providers
from llm import get_llm_provider
from logger import AppLogger
from services import (
    DocumentParser,
    EmbeddingGenerator,
    RerankerService,
    VectorStore,
)
from services.grpc import ChatService, KnowledgeBaseService


class Container(containers.DeclarativeContainer):
    """Dependency injection container for application components."""

    config = providers.Object(settings)

    app_logger = providers.Singleton(AppLogger, settings=config)

    database = providers.Singleton(Database, settings=config, logger=app_logger)

    chunks_repository = providers.Factory(DocumentChunkRepository, db=database)

    chunk_service = providers.Factory(
        ChunkService,
        chunk_repo=chunks_repository,
        logger=app_logger,
    )

    llm_client = providers.Factory(get_llm_provider, settings=config, logger=app_logger)

    document_parser = providers.Factory(DocumentParser, settings=config, logger=app_logger)

    embedding_generator = providers.Factory(EmbeddingGenerator, settings=config, logger=app_logger)

    reranker_service = providers.Factory(RerankerService, settings=config, logger=app_logger)

    vector_store = providers.Factory(
        VectorStore, settings=config, logger=app_logger, embedding_generator=embedding_generator
    )

    chat_service = providers.Factory(
        ChatService,
        logger=app_logger,
        llm_provider=llm_client,
        vector_store=vector_store,
        embedder=embedding_generator,
        reranker=reranker_service,
        chunk_service=chunk_service,
    )

    knowledge_base_service = providers.Factory(
        KnowledgeBaseService,
        settings=config,
        logger=app_logger,
        vector_store=vector_store,
        embedder=embedding_generator,
        parser=document_parser,
        chunk_service=chunk_service,
    )
