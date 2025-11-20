from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, delete, func
from datetime import datetime, timedelta
from typing import Optional
from app.core.database import get_db
from app.core.auth import get_current_admin_user
from app.models.models import Test, Server, User, Trace, TraceHop

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


@router.delete("/cleanup/traces")
async def cleanup_traces(
    days: Optional[int] = Query(None, description="Delete traces older than X days"),
    all: bool = Query(False, description="Delete ALL traces"),
    db: AsyncSession = Depends(get_db),
    current_user: User = Depends(get_current_admin_user)
):
    """Delete trace data (admin only)"""
    
    if not all and days is None:
        raise HTTPException(
            status_code=400,
            detail="Must specify 'days' or 'all=true'"
        )
    
    # Build delete query for hops first (foreign key constraint)
    if all:
        hop_result = await db.execute(delete(TraceHop))
        trace_result = await db.execute(delete(Trace))
    else:
        cutoff_date = datetime.utcnow() - timedelta(days=days)
        
        # Get trace IDs to delete
        trace_ids_result = await db.execute(
            select(Trace.id).where(Trace.started_at < cutoff_date)
        )
        trace_ids = [row[0] for row in trace_ids_result.fetchall()]
        
        if trace_ids:
            # Delete hops first
            hop_result = await db.execute(
                delete(TraceHop).where(TraceHop.trace_id.in_(trace_ids))
            )
            # Then delete traces
            trace_result = await db.execute(
                delete(Trace).where(Trace.id.in_(trace_ids))
            )
        else:
            hop_result = None
            trace_result = None
    
    await db.commit()
    
    deleted_traces = trace_result.rowcount if trace_result else 0
    deleted_hops = hop_result.rowcount if hop_result else 0
    
    return {
        "message": f"Deleted {deleted_traces} trace(s) and {deleted_hops} hop(s)",
        "deleted_traces": deleted_traces,
        "deleted_hops": deleted_hops
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
    
    # Count traces
    trace_count_result = await db.execute(select(func.count(Trace.id)))
    trace_count = trace_count_result.scalar()
    
    # Count trace hops
    hop_count_result = await db.execute(select(func.count(TraceHop.id)))
    hop_count = hop_count_result.scalar()
    
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
        "total_traces": trace_count,
        "total_hops": hop_count,
        "oldest_test": oldest_test,
        "newest_test": newest_test
    }
