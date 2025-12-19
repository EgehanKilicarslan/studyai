import os
import sys
import time
from concurrent import futures

import grpc

# Add parent directory and pb to sys.path
current_dir = os.path.dirname(os.path.abspath(__file__))  # .../app
parent_dir = os.path.dirname(current_dir)  # .../backend-python
sys.path.append(parent_dir)
sys.path.append(os.path.join(parent_dir, "pb"))

from pb import rag_service_pb2, rag_service_pb2_grpc  # noqa: E402


class RagService(rag_service_pb2_grpc.RagServiceServicer):
    """
    Python implementation of the RagService defined in the proto file.
    """

    def Chat(self, request, context):
        print(f"[Python] Question Received: {request.query} (Session: {request.session_id})")

        # MOCK ANSWER (No LLM yet)
        # AI logic will run here.
        fake_answer = f"Hello! I received your question '{request.query}'. My brain isn't fully operational yet, but our connection is working!"
        # Dummy Source
        dummy_source = rag_service_pb2.Source(
            filename="test_doc.pdf", page_number=1, snippet="This is a test document...", score=0.95
        )

        # Return in the ChatResponse format defined in the proto file
        return rag_service_pb2.ChatResponse(
            answer=fake_answer, source_documents=[dummy_source], processing_time_ms=120.5
        )


def serve():
    # Start the gRPC server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    rag_service_pb2_grpc.add_RagServiceServicer_to_server(RagService(), server)

    port = "50051"
    server.add_insecure_port(f"[::]:{port}")
    print(f"🚀 [Python] AI Service Started! Listening on port: {port}...")

    server.start()
    try:
        # Keep the server running
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        server.stop(0)


if __name__ == "__main__":
    serve()
