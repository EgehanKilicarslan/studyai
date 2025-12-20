import os
import sys
import time
from concurrent import futures

import grpc

current_dir = os.path.dirname(os.path.abspath(__file__))
parent_dir = os.path.dirname(current_dir)
sys.path.append(parent_dir)
sys.path.append(os.path.join(parent_dir, "pb"))

from pb import rag_service_pb2_grpc  # noqa: E402

# Containers and Services
from app.containers import Container  # noqa: E402


def serve():
    container = Container()

    # If i needed, load configuration from a file or environment variables
    # container.config.from_yaml("config.yml")

    # 2. Request the service from the container (Resolution)
    rag_service_instance = container.llm_service()

    # 3. Start the gRPC Server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

    # Register the created instance to the server
    rag_service_pb2_grpc.add_RagServiceServicer_to_server(rag_service_instance, server)

    port = "50051"
    server.add_insecure_port(f"[::]:{port}")

    print("🚀 [Python] AI Service Started (DI Enabled)!")
    print(f"   -> Active LLM: {rag_service_instance.llm.provider_name}")

    server.start()
    try:
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        server.stop(0)


if __name__ == "__main__":
    serve()
