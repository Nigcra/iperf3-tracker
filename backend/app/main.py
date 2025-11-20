from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager
import logging
from app.core.config import settings
from app.core.database import init_db
from app.core.logging_config import setup_logging_filters
from app.services.scheduler_service import scheduler_service
from app.api import servers, tests, stats, auth, admin, public_servers, traces, live_trace

# Configure logging
logging.basicConfig(
    level=settings.LOG_LEVEL,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)

# Setup logging filters to reduce noise
setup_logging_filters()

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifecycle handler for startup and shutdown"""
    # Startup
    logger.info("Starting iperf3-Tracker API")  # Reload trigger
    
    # Initialize database
    await init_db()
    logger.info("Database initialized")
    
    # Start scheduler if enabled
    if settings.SCHEDULER_ENABLED:
        scheduler_service.start()
        await scheduler_service.sync_scheduled_servers()
        logger.info("Scheduler started and synced")
    
    yield
    
    # Shutdown
    logger.info("Shutting down iperf3-Tracker API")
    
    if settings.SCHEDULER_ENABLED:
        scheduler_service.shutdown()
        logger.info("Scheduler stopped")


# Create FastAPI app
app = FastAPI(
    title=settings.PROJECT_NAME,
    version=settings.VERSION,
    lifespan=lifespan,
    docs_url="/docs",
    redoc_url="/redoc"
)

# Configure CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.BACKEND_CORS_ORIGINS,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(auth.router, prefix=settings.API_V1_STR)
app.include_router(servers.router, prefix=settings.API_V1_STR)
app.include_router(tests.router, prefix=settings.API_V1_STR)
app.include_router(stats.router, prefix=settings.API_V1_STR)
app.include_router(admin.router, prefix=settings.API_V1_STR)
app.include_router(public_servers.router, prefix=settings.API_V1_STR)
app.include_router(traces.router, prefix=settings.API_V1_STR)
app.include_router(live_trace.router, prefix=settings.API_V1_STR)


@app.get("/")
async def root():
    """Root endpoint"""
    return {
        "name": settings.PROJECT_NAME,
        "version": settings.VERSION,
        "docs": "/docs"
    }


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "scheduler_running": scheduler_service.scheduler.running if settings.SCHEDULER_ENABLED else False
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=8000,
        reload=True
    )
