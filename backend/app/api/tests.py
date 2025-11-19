from fastapi import APIRouter, Depends, HTTPException, Query, BackgroundTasks
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, func, and_, desc
from typing import List, Optional, Dict, Any
from datetime import datetime, timedelta
from app.core.database import get_db
from app.models.models import Test, Server, TestStatus
from app.schemas.schemas import TestCreate, TestResponse, TestDetailResponse
from app.services.iperf_service import TestService, IperfService

router = APIRouter(prefix="/tests", tags=["tests"])


@router.get("", response_model=List[TestResponse])
async def list_tests(
    server_id: Optional[int] = None,
    status: Optional[TestStatus] = None,
    from_date: Optional[datetime] = None,
    to_date: Optional[datetime] = None,
    skip: int = Query(0, ge=0),
    limit: int = Query(100, ge=1, le=1000),
    db: AsyncSession = Depends(get_db)
):
    """List test results with optional filters"""
    query = select(Test)
    
    # Apply filters
    filters = []
    if server_id is not None:
        filters.append(Test.server_id == server_id)
    if status is not None:
        filters.append(Test.status == status)
    if from_date is not None:
        filters.append(Test.created_at >= from_date)
    if to_date is not None:
        filters.append(Test.created_at <= to_date)
    
    if filters:
        query = query.where(and_(*filters))
    
    query = query.offset(skip).limit(limit).order_by(desc(Test.created_at))
    
    result = await db.execute(query)
    tests = result.scalars().all()
    
    return tests


@router.get("/{test_id}", response_model=TestDetailResponse)
async def get_test(
    test_id: int,
    db: AsyncSession = Depends(get_db)
):
    """Get a specific test by ID with full details"""
    result = await db.execute(
        select(Test).where(Test.id == test_id)
    )
    test = result.scalar_one_or_none()
    
    if not test:
        raise HTTPException(status_code=404, detail="Test not found")
    
    # Load server relationship
    await db.refresh(test, ["server"])
    
    return test


@router.post("/run", response_model=TestResponse, status_code=201)
async def run_test(
    test_config: TestCreate,
    background_tasks: BackgroundTasks,
    db: AsyncSession = Depends(get_db)
):
    """Run an immediate iperf3 test (asynchronously in background)"""
    try:
        test_service = TestService(db)
        
        # Create the test record first (with PENDING status)
        test = await test_service.create_test(
            server_id=test_config.server_id,
            duration=test_config.duration,
            parallel_streams=test_config.parallel_streams,
            protocol=test_config.protocol,
            direction=test_config.direction
        )
        
        # Run the test in the background
        background_tasks.add_task(
            test_service.run_test_background,
            test.id
        )
        
        return test
        
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Test execution failed: {str(e)}")


@router.delete("/{test_id}", status_code=204)
async def delete_test(
    test_id: int,
    db: AsyncSession = Depends(get_db)
):
    """Delete a test result"""
    result = await db.execute(select(Test).where(Test.id == test_id))
    test = result.scalar_one_or_none()
    
    if not test:
        raise HTTPException(status_code=404, detail="Test not found")
    
    await db.delete(test)
    await db.commit()
    
    return None


@router.get("/server/{server_id}/latest", response_model=Optional[TestResponse])
async def get_latest_test(
    server_id: int,
    db: AsyncSession = Depends(get_db)
):
    """Get the latest test result for a server"""
    result = await db.execute(
        select(Test)
        .where(Test.server_id == server_id, Test.status == TestStatus.COMPLETED)
        .order_by(desc(Test.created_at))
        .limit(1)
    )
    test = result.scalar_one_or_none()
    
    return test


@router.get("/{test_id}/live")
async def get_test_live_status(
    test_id: int,
    db: AsyncSession = Depends(get_db)
) -> Dict[str, Any]:
    """Get live status of a running test"""
    # Check if test exists and is running
    result = await db.execute(
        select(Test, Server)
        .join(Server, Test.server_id == Server.id)
        .where(Test.id == test_id)
    )
    row = result.first()
    
    if not row:
        raise HTTPException(status_code=404, detail="Test not found")
    
    test, server = row
    
    # Get live data from active tests
    live_data = IperfService.get_active_test(test_id)
    
    if live_data:
        # We have live data - show it with proper status
        status = live_data.get("status", "running")
        return {
            "test_id": test_id,
            "server_name": server.name,
            "is_running": status in ["running", "failed", "completed", "pending"],  # Include pending
            "status": status,
            "progress": live_data.get("progress", 0),
            "elapsed_seconds": live_data.get("elapsed_seconds", 0),
            "total_seconds": live_data.get("total_seconds", 0),
            "current_download_mbps": live_data.get("current_download_mbps", 0),
            "current_upload_mbps": live_data.get("current_upload_mbps", 0),
        }
    
    # Test not actively running, return test status
    return {
        "test_id": test_id,
        "server_name": server.name,
        "is_running": test.status in [TestStatus.RUNNING, TestStatus.PENDING],  # Include PENDING
        "status": test.status.value if test.status else "unknown",
        "progress": 100 if test.status == TestStatus.COMPLETED else 0,
        "elapsed_seconds": 0,
        "total_seconds": test.duration,
        "current_download_mbps": test.download_bandwidth_mbps or 0,
        "current_upload_mbps": test.upload_bandwidth_mbps or 0,
    }
