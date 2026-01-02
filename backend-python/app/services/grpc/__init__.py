# Note: Imports are explicit to avoid circular import issues.
# go_client/api_grpc_client is NOT imported here because it's used by
# tasks/document_tasks.py, which is imported by knowledge_base_service.py

from .chat_service import ChatService
from .knowledge_base_service import KnowledgeBaseService

__all__ = ["ChatService", "KnowledgeBaseService"]
