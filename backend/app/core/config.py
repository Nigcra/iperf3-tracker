from pydantic_settings import BaseSettings
from typing import Optional


class Settings(BaseSettings):
    """Application settings"""
    
    # API Settings
    API_V1_STR: str = "/api"
    PROJECT_NAME: str = "iperf3-Tracker"
    VERSION: str = "1.0.0"
    
    # CORS - Allow all origins by default (can be restricted via environment variable)
    BACKEND_CORS_ORIGINS: list = ["*"]
    
    # Database
    DATABASE_URL: str = "sqlite+aiosqlite:///./iperf_tracker.db"
    
    # iperf3 Settings
    IPERF3_DEFAULT_PORT: int = 5201
    IPERF3_DEFAULT_DURATION: int = 10
    IPERF3_DEFAULT_PARALLEL: int = 1
    IPERF3_MAX_PARALLEL: int = 128
    
    # Scheduler
    SCHEDULER_ENABLED: bool = True
    DEFAULT_TEST_INTERVAL_MINUTES: int = 30
    
    # Logging
    LOG_LEVEL: str = "INFO"
    
    class Config:
        case_sensitive = True
        env_file = ".env"


settings = Settings()
