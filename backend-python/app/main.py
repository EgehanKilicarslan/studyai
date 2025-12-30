import asyncio

import grpc
from containers import Container
from pb import rag_service_pb2_grpc


async def serve():
    # 1. Create DI Container
    container = Container()

    # 2. Resolve service
    settings = container.config()

    app_logger = container.app_logger()
    app_logger.setup()

    database = container.database()
    await database.connect()

    logger = app_logger.get_logger(__name__)

    chat_service_instance = container.chat_service()
    knowledge_base_service_instance = container.knowledge_base_service()

    # 3. Start gRPC Server in async mode
    server = grpc.aio.server()

    # Save service
    rag_service_pb2_grpc.add_ChatServiceServicer_to_server(chat_service_instance, server)
    rag_service_pb2_grpc.add_KnowledgeBaseServiceServicer_to_server(
        knowledge_base_service_instance, server
    )

    server.add_insecure_port(f"[::]:{settings.ai_service_port}")

    logger.info("🚀 [Python] AI Service Started (DI Enabled)!")
    logger.info(f"   -> Active LLM: {settings.llm_provider}")

    await server.start()

    # Keep the server running
    await server.wait_for_termination()


if __name__ == "__main__":
    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        pass  # Graceful exit on Ctrl+C
