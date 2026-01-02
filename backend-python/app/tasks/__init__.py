"""
Tasks package for Celery async processing.
"""

from .document_tasks import process_document_task

__all__ = ["process_document_task"]
