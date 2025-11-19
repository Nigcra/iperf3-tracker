from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, delete, func
from datetime import datetime, timedelta
from typing import Optional
from app.core.database import get_db
from app.core.auth import get_current_admin_user
from app.models.models import Test, Server, User

router = APIRouter(prefix="/admin", tags=["admin"])


@router.delete("/cleanup/tests")
async def cleanup_tests(
    days: Optional[int] = Query(None, description="Delete tests older than X days"),
    server_id: Optional[int] = Query(None, description="Delete tests for specific server"),
    all: bool = Query(False, description="Delete ALL tests"),
    db: AsyncSession = Depends(get_db),
    current_user: User = Depends(get_current_admin_user)
):
    """Delete test data (admin only)"""
    
    if not all and days is None and server_id is None:
        raise HTTPException(
            status_code=400,
            detail="Must specify 'days', 'server_id', or 'all=true'"
        )
    
    # Build delete query
    query = delete(Test)
    
    conditions = []
    if server_id is not None:
        conditions.append(Test.server_id == server_id)
    
    if days is not None:
        cutoff_date = datetime.utcnow() - timedelta(days=days)
        conditions.append(Test.created_at < cutoff_date)
    
    if conditions:
        query = query.where(*conditions)
    
    # Execute deletion
    result = await db.execute(query)
    await db.commit()
    
    deleted_count = result.rowcount
    
    return {
        "message": f"Deleted {deleted_count} test(s)",
        "deleted_count": deleted_count
    }


@router.get("/stats/database")
async def get_database_stats(
    db: AsyncSession = Depends(get_db),
    current_user: User = Depends(get_current_admin_user)
):
    """Get database statistics (admin only)"""
    
    # Count tests
    test_count_result = await db.execute(select(func.count(Test.id)))
    test_count = test_count_result.scalar()
    
    # Count servers
    server_count_result = await db.execute(select(func.count(Server.id)))
    server_count = server_count_result.scalar()
    
    # Count users
    user_count_result = await db.execute(select(func.count(User.id)))
    user_count = user_count_result.scalar()
    
    # Get oldest test
    oldest_test_result = await db.execute(
        select(Test.created_at).order_by(Test.created_at).limit(1)
    )
    oldest_test = oldest_test_result.scalar_one_or_none()
    
    # Get newest test
    newest_test_result = await db.execute(
        select(Test.created_at).order_by(Test.created_at.desc()).limit(1)
    )
    newest_test = newest_test_result.scalar_one_or_none()
    
    return {
        "total_tests": test_count,
        "total_servers": server_count,
        "total_users": user_count,
        "oldest_test": oldest_test,
        "newest_test": newest_test
    }
