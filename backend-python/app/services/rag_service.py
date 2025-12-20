from typing import List

from pb import rag_service_pb2, rag_service_pb2_grpc

from ..llm.base import LLMProvider


class RagService(rag_service_pb2_grpc.RagServiceServicer):
    def __init__(self, llm_provider: LLMProvider):
        self.llm: LLMProvider = llm_provider

    def Chat(self, request: rag_service_pb2.ChatRequest, context) -> rag_service_pb2.ChatResponse:
        print(f"[RagService] Request received. LLM: {self.llm.provider_name}")

        # Mock documents (We will inject VectorDB Service here in the future)
        mock_docs: List[str] = ["Document A", "Document B"]

        try:
            answer: str = self.llm.generate_response(
                query=request.query, context_docs=mock_docs, history=[]
            )
        except Exception as e:
            print(f"Error: {e}")
            answer = "An error occurred."

        return rag_service_pb2.ChatResponse(
            answer=answer, source_documents=[], processing_time_ms=10.0
        )
