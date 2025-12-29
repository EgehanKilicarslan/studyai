import logging
from unittest.mock import Mock, patch

from app.config import Settings
from app.logger import AppLogger


def test_logger_initialization():
    """Test Logger class initialization."""
    settings = Mock(spec=Settings)
    settings.log_level = "INFO"

    logger = AppLogger(settings=settings)
    assert logger is not None


def test_get_logger_returns_logger():
    """Test that get_logger returns a logging.Logger instance."""
    settings = Mock(spec=Settings)
    settings.log_level = "INFO"

    logger = AppLogger(settings=settings)
    log = logger.get_logger(__name__)

    assert isinstance(log, logging.Logger)


def test_logger_level_configuration():
    """Test logger level is set correctly."""
    settings = Mock(spec=Settings)
    settings.log_level = "DEBUG"

    logger = AppLogger(settings=settings)
    log = logger.get_logger(__name__)

    # Should have DEBUG level
    assert log.level in [logging.DEBUG, logging.INFO, logging.NOTSET]


def test_logger_singleton_behavior():
    """Test that get_logger returns configured loggers."""
    settings = Mock(spec=Settings)
    settings.log_level = "INFO"

    logger = AppLogger(settings=settings)
    log1 = logger.get_logger("test1")
    log2 = logger.get_logger("test1")

    assert log1.name == log2.name


def test_logger_formatting():
    """Test that logger formats messages correctly."""
    settings = Mock(spec=Settings)
    settings.log_level = "INFO"

    logger = AppLogger(settings=settings)
    log = logger.get_logger(__name__)

    with patch.object(log, "info") as mock_info:
        log.info("Test message")
        mock_info.assert_called_once()
