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

    logger = app_logger.get_logger(__name__)

    rag_service_instance = container.rag_service()

    # 3. Start gRPC Server in async mode
    server = grpc.aio.server()

    # Save service
    rag_service_pb2_grpc.add_RagServiceServicer_to_server(rag_service_instance, server)

    server.add_insecure_port(f"[::]:{settings.ai_service_port}")

    logger.info("🚀 [Python] AI Service Started (DI Enabled)!")
    logger.info(f"   -> Active LLM: {rag_service_instance.llm.provider_name}")

    await server.start()

    # Keep the server running
    await server.wait_for_termination()


if __name__ == "__main__":
    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        pass  # Graceful exit on Ctrl+C
