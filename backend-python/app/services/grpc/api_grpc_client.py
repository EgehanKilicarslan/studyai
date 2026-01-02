"""
gRPC client for calling Go's RagService.

This client is used by Celery workers to notify Go about document
processing status updates (COMPLETED or ERROR).
"""

import logging
from typing import Optional

import grpc
from config import settings
from pb import rag_service_pb2 as rs
from pb import rag_service_pb2_grpc as rs_grpc


class ApiGrpcClient:
    """
    Synchronous gRPC client for calling Go's RagService.

    Used by Celery workers (which run synchronously) to update
    document status after processing completes.
    """

    def __init__(self, address: Optional[str] = None):
        """
        Initialize the gRPC client.

        Args:
            address: Optional Go gRPC service address. Defaults to settings.
        """
        self.address = address or settings.api_grpc_addr
        self.logger = logging.getLogger(__name__)
        self._channel: Optional[grpc.Channel] = None
        self._stub: Optional[rs_grpc.RagServiceStub] = None

    def _ensure_connected(self) -> rs_grpc.RagServiceStub:
        """Ensure we have a valid connection and return the stub."""
        if self._channel is None or self._stub is None:
            self.logger.info(f"[ApiGrpcClient] Connecting to Go gRPC at {self.address}")
            self._channel = grpc.insecure_channel(self.address)
            self._stub = rs_grpc.RagServiceStub(self._channel)
        return self._stub

    def close(self) -> None:
        """Close the gRPC channel."""
        if self._channel is not None:
            self._channel.close()
            self._channel = None
            self._stub = None

    def update_document_status(
        self,
        document_id: str,
        status: int,
        chunks_count: int = 0,
        error_message: str = "",
    ) -> rs.DocumentStatusResponse:
        """
        Update document status in Go.

        Args:
            document_id: UUID of the document
            status: Processing status (COMPLETED or ERROR) - use rs.DOCUMENT_STATUS_* constants
            chunks_count: Number of chunks created (for COMPLETED)
            error_message: Error message (for ERROR status)

        Returns:
            DocumentStatusResponse from Go

        Raises:
            grpc.RpcError: If the gRPC call fails
        """
        stub = self._ensure_connected()

        request = rs.DocumentStatusRequest(
            document_id=document_id,
            status=status,  # type: ignore[arg-type]
            chunks_count=chunks_count,
            error_message=error_message,
        )

        self.logger.info(
            f"[ApiGrpcClient] Calling UpdateDocumentStatus: "
            f"doc_id={document_id}, status={rs.DocumentProcessingStatus.Name(status)}, "  # type: ignore[arg-type]
            f"chunks={chunks_count}"
        )

        try:
            response = stub.UpdateDocumentStatus(request, timeout=30)

            if response.success:
                self.logger.info(f"[ApiGrpcClient] ✅ Status update successful: {response.message}")
            else:
                self.logger.error(f"[ApiGrpcClient] ❌ Status update failed: {response.message}")

            return response

        except grpc.RpcError as e:
            self.logger.error(f"[ApiGrpcClient] ❌ gRPC error: {e.code()}: {e.details()}")
            raise

    def __enter__(self) -> "ApiGrpcClient":
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        """Context manager exit."""
        self.close()


# Singleton instance for Celery workers
_go_client: Optional[ApiGrpcClient] = None


def get_api_grpc_client() -> ApiGrpcClient:
    """
    Get or create a singleton ApiGrpcClient instance.

    Returns:
        ApiGrpcClient instance
    """
    global _go_client
    if _go_client is None:
        _go_client = ApiGrpcClient()
    return _go_client


def update_document_status_via_grpc(
    document_id: str,
    status: int,
    chunks_count: int = 0,
    error_message: str = "",
) -> bool:
    """
    Convenience function to update document status via gRPC.

    This function handles connection management and error handling,
    making it safe to call from Celery tasks.

    Args:
        document_id: UUID of the document
        status: Processing status (COMPLETED or ERROR) - use rs.DOCUMENT_STATUS_* constants
        chunks_count: Number of chunks created (for COMPLETED)
        error_message: Error message (for ERROR status)

    Returns:
        True if the update was successful, False otherwise
    """
    logger = logging.getLogger(__name__)

    try:
        client = get_api_grpc_client()
        response = client.update_document_status(
            document_id=document_id,
            status=status,
            chunks_count=chunks_count,
            error_message=error_message,
        )
        return response.success

    except grpc.RpcError as e:
        logger.error(
            f"[update_document_status_via_grpc] gRPC call failed: {e.code()}: {e.details()}"
        )
        return False

    except Exception as e:
        logger.error(f"[update_document_status_via_grpc] Unexpected error: {e}")
        return False
