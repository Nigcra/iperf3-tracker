import logging
from typing import Any


class EndpointFilter(logging.Filter):
    """Filter out /healthz and frequent polling endpoints from access logs"""
    
    def filter(self, record: logging.LogRecord) -> bool:
        # Filter out noisy endpoints
        message = record.getMessage()
        
        # List of endpoints to filter from logs
        filtered_endpoints = [
            "GET /api/tests?status=running",  # Dashboard polling
            "GET /api/tests/",  # Test live status (partial match)
            "/live HTTP/1.1",  # Live status endpoints
        ]
        
        # Only filter INFO level access logs
        if record.levelno == logging.INFO:
            for endpoint in filtered_endpoints:
                if endpoint in message:
                    return False
        
        return True


def setup_logging_filters():
    """Add filters to uvicorn access logger"""
    # Add filter to uvicorn access logger
    logging.getLogger("uvicorn.access").addFilter(EndpointFilter())
