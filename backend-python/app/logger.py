import logging
import sys

from pythonjsonlogger.json import JsonFormatter

from app.config import Settings


class AppLogger:
    def __init__(self, settings: Settings):
        self.settings = settings
        self.logger = logging.getLogger()

    def setup(self):
        self.logger.setLevel(self.settings.log_level.upper())

        handler = logging.StreamHandler(sys.stdout)

        if self.settings.app_env.lower() == "production":
            #  JSON format
            formatter = JsonFormatter("%(asctime)s %(levelname)s %(name)s %(message)s %(filename)s")
        else:
            # Human-readable format
            formatter = logging.Formatter("[%(asctime)s] %(levelname)s [%(name)s]: %(message)s")

        handler.setFormatter(formatter)

        # Reset handlers to avoid duplicate logs
        if self.logger.hasHandlers():
            self.logger.handlers.clear()
        self.logger.addHandler(handler)

    def get_logger(self, name: str) -> logging.Logger:
        return logging.getLogger(name)
