from fastapi import APIRouter, Depends
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, func, and_, desc
from typing import List
from datetime import datetime, timedelta
from app.core.database import get_db
from app.models.models import Test, Server, TestStatus
from app.schemas.schemas import ServerStats, DashboardStats

router = APIRouter(prefix="/stats", tags=["statistics"])


@router.get("/dashboard", response_model=DashboardStats)
async def get_dashboard_stats(db: AsyncSession = Depends(get_db)):
    """Get overall dashboard statistics"""
    
    # Total and active servers
    total_servers_result = await db.execute(select(func.count(Server.id)))
    total_servers = total_servers_result.scalar() or 0
    
    active_servers_result = await db.execute(
        select(func.count(Server.id)).where(Server.enabled == True)
    )
    active_servers = active_servers_result.scalar() or 0
    
    # Total tests
    total_tests_result = await db.execute(select(func.count(Test.id)))
    total_tests = total_tests_result.scalar() or 0
    
    # Tests today
    today = datetime.utcnow().date()
    tests_today_result = await db.execute(
        select(func.count(Test.id)).where(
            func.date(Test.created_at) == today
        )
    )
    tests_today = tests_today_result.scalar() or 0
    
    # Average bandwidth (completed tests only)
    avg_download_result = await db.execute(
        select(func.avg(Test.download_bandwidth_mbps)).where(
            Test.status == TestStatus.COMPLETED,
            Test.download_bandwidth_mbps.isnot(None)
        )
    )
    avg_download_mbps = avg_download_result.scalar()
    
    avg_upload_result = await db.execute(
        select(func.avg(Test.upload_bandwidth_mbps)).where(
            Test.status == TestStatus.COMPLETED,
            Test.upload_bandwidth_mbps.isnot(None)
        )
    )
    avg_upload_mbps = avg_upload_result.scalar()
    
    # Last test timestamp
    last_test_result = await db.execute(
        select(Test.created_at).order_by(desc(Test.created_at)).limit(1)
    )
    last_test_at = last_test_result.scalar_one_or_none()
    
    return DashboardStats(
        total_servers=total_servers,
        active_servers=active_servers,
        total_tests=total_tests,
        tests_today=tests_today,
        avg_download_mbps=avg_download_mbps,
        avg_upload_mbps=avg_upload_mbps,
        last_test_at=last_test_at
    )


@router.get("/servers", response_model=List[ServerStats])
async def get_server_stats(db: AsyncSession = Depends(get_db)):
    """Get statistics for all servers"""
    
    servers_result = await db.execute(select(Server))
    servers = servers_result.scalars().all()
    
    stats_list = []
    
    for server in servers:
        # Total tests
        total_result = await db.execute(
            select(func.count(Test.id)).where(Test.server_id == server.id)
        )
        total_tests = total_result.scalar() or 0
        
        # Successful tests
        success_result = await db.execute(
            select(func.count(Test.id)).where(
                Test.server_id == server.id,
                Test.status == TestStatus.COMPLETED
            )
        )
        successful_tests = success_result.scalar() or 0
        
        # Failed tests
        failed_tests = total_tests - successful_tests
        
        # Averages (successful tests only)
        avg_download_result = await db.execute(
            select(func.avg(Test.download_bandwidth_mbps)).where(
                Test.server_id == server.id,
                Test.status == TestStatus.COMPLETED,
                Test.download_bandwidth_mbps.isnot(None)
            )
        )
        avg_download_mbps = avg_download_result.scalar()
        
        avg_upload_result = await db.execute(
            select(func.avg(Test.upload_bandwidth_mbps)).where(
                Test.server_id == server.id,
                Test.status == TestStatus.COMPLETED,
                Test.upload_bandwidth_mbps.isnot(None)
            )
        )
        avg_upload_mbps = avg_upload_result.scalar()
        
        avg_jitter_result = await db.execute(
            select(func.avg(Test.download_jitter_ms)).where(
                Test.server_id == server.id,
                Test.status == TestStatus.COMPLETED,
                Test.download_jitter_ms.isnot(None)
            )
        )
        avg_jitter_ms = avg_jitter_result.scalar()
        
        avg_loss_result = await db.execute(
            select(func.avg(Test.download_packet_loss_percent)).where(
                Test.server_id == server.id,
                Test.status == TestStatus.COMPLETED,
                Test.download_packet_loss_percent.isnot(None)
            )
        )
        avg_packet_loss_percent = avg_loss_result.scalar()
        
        # Last test timestamp
        last_test_result = await db.execute(
            select(Test.created_at)
            .where(Test.server_id == server.id)
            .order_by(desc(Test.created_at))
            .limit(1)
        )
        last_test_at = last_test_result.scalar_one_or_none()
        
        stats_list.append(ServerStats(
            server_id=server.id,
            server_name=server.name,
            total_tests=total_tests,
            successful_tests=successful_tests,
            failed_tests=failed_tests,
            avg_download_mbps=avg_download_mbps,
            avg_upload_mbps=avg_upload_mbps,
            avg_jitter_ms=avg_jitter_ms,
            avg_packet_loss_percent=avg_packet_loss_percent,
            last_test_at=last_test_at
        ))
    
    return stats_list


@router.get("/servers/{server_id}", response_model=ServerStats)
async def get_server_stat(
    server_id: int,
    db: AsyncSession = Depends(get_db)
):
    """Get statistics for a specific server"""
    
    # Check if server exists
    server_result = await db.execute(select(Server).where(Server.id == server_id))
    server = server_result.scalar_one_or_none()
    
    if not server:
        from fastapi import HTTPException
        raise HTTPException(status_code=404, detail="Server not found")
    
    # Get stats (reuse logic from above)
    stats_list = await get_server_stats(db)
    server_stat = next((s for s in stats_list if s.server_id == server_id), None)
    
    return server_stat
