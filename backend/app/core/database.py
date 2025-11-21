from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession, async_sessionmaker
from sqlalchemy.orm import DeclarativeBase
from app.core.config import settings


# Create async engine
engine = create_async_engine(
    settings.DATABASE_URL,
    echo=True if settings.LOG_LEVEL == "DEBUG" else False,
    future=True
)

# Create async session factory
AsyncSessionLocal = async_sessionmaker(
    engine,
    class_=AsyncSession,
    expire_on_commit=False,
    autocommit=False,
    autoflush=False,
)


class Base(DeclarativeBase):
    """Base class for all models"""
    pass


async def get_db():
    """Dependency to get database session"""
    async with AsyncSessionLocal() as session:
        try:
            yield session
        finally:
            await session.close()


async def init_db():
    """Initialize database - create all tables"""
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)


async def init_default_admin():
    """Create default admin user if no users exist"""
    from app.models.models import User
    from app.core.auth import get_password_hash
    from sqlalchemy import select, func
    
    async with AsyncSessionLocal() as session:
        # Check if any users exist
        result = await session.execute(select(func.count(User.id)))
        user_count = result.scalar()
        
        if user_count == 0:
            # Create default admin user
            admin = User(
                username="admin",
                email="admin@iperf-tracker.local",
                hashed_password=get_password_hash("admin"),
                is_admin=True,
                is_active=True
            )
            session.add(admin)
            await session.commit()
            
            import logging
            logger = logging.getLogger(__name__)
            logger.info("✅ Default admin user created (username: admin, password: admin)")
            logger.warning("⚠️  Please change the default admin password after first login!")
