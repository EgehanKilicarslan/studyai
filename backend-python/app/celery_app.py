"""
Celery application configuration for async document processing.

This module configures Celery to use Redis as the message broker
for asynchronous task processing.
"""

from celery import Celery
from config import settings

# Create Celery app with Redis as broker and result backend
celery_app = Celery(
    "studyai_worker",
    broker=settings.celery_broker_url,
    backend=settings.celery_result_backend,
    include=["tasks.document_tasks"],
)

# Celery configuration
celery_app.conf.update(
    # Task settings
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="UTC",
    enable_utc=True,
    # Task execution settings
    task_acks_late=True,  # Acknowledge task after completion (for reliability)
    task_reject_on_worker_lost=True,  # Reject task if worker dies
    worker_prefetch_multiplier=1,  # Process one task at a time per worker
    # Result settings
    result_expires=3600,  # Results expire after 1 hour
    # Task routes (optional, for future scaling)
    task_routes={
        "tasks.document_tasks.*": {"queue": "document_processing"},
    },
    # Retry settings
    task_default_retry_delay=60,  # 1 minute default retry delay
    task_max_retries=3,
)

# Export for use in tasks
__all__ = ["celery_app"]
