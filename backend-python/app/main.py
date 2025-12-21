import asyncio
import os
import sys

import grpc

current_dir = os.path.dirname(os.path.abspath(__file__))
parent_dir = os.path.dirname(current_dir)
sys.path.append(parent_dir)
sys.path.append(os.path.join(parent_dir, "pb"))

from pb import rag_service_pb2_grpc  # noqa: E402

from app.containers import Container  # noqa: E402


async def serve():
    # 1. Create DI Container
    container = Container()

    # 2. Resolve service
    settings = container.config()
    rag_service_instance = container.rag_service()

    # 3. Start gRPC Server in async mode
    server = grpc.aio.server()

    # Save service
    rag_service_pb2_grpc.add_RagServiceServicer_to_server(rag_service_instance, server)

    server.add_insecure_port(f"[::]:{settings.python_port}")

    print("🚀 [Python] AI Service Started (DI Enabled)!")
    print(f"   -> Active LLM: {rag_service_instance.llm.provider_name}")

    await server.start()

    # Keep the server running
    await server.wait_for_termination()


if __name__ == "__main__":
    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        pass  # Graceful exit on Ctrl+C
