from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, and_
from typing import List
import logging
from datetime import datetime

from app.core.database import get_db
from app.core.auth import get_current_user
from app.models.models import User, Test, Trace, TraceHop
from app.schemas.schemas import TraceResponse, TraceHopResponse, TraceCreate
from app.services.traceroute_service import TracerouteService

router = APIRouter()
logger = logging.getLogger(__name__)


async def run_trace_background(test_id: int):
    """Run traceroute in background and save to database"""
    from app.core.database import AsyncSessionLocal
    
    async with AsyncSessionLocal() as db:
        try:
            # Get test
            result = await db.execute(select(Test).where(Test.id == test_id))
            test = result.scalar_one_or_none()
            
            if not test:
                logger.error(f"Test {test_id} not found for tracing")
                return
            
            # Get server info
            from app.models.models import Server
            result = await db.execute(select(Server).where(Server.id == test.server_id))
            server = result.scalar_one_or_none()
            
            if not server:
                logger.error(f"Server {test.server_id} not found for tracing")
                return
            
            # Check if trace already exists
            result = await db.execute(select(Trace).where(Trace.test_id == test_id))
            existing_trace = result.scalar_one_or_none()
            
            if existing_trace:
                logger.info(f"Trace already exists for test {test_id}")
                return
            
            # Create trace record
            trace = Trace(
                test_id=test_id,
                destination_host=server.host,
                started_at=datetime.utcnow()
            )
            db.add(trace)
            await db.commit()
            await db.refresh(trace)
            
            # Run traceroute
            traceroute_service = TracerouteService()
            result = await traceroute_service.run_traceroute(server.host)
            
            # Update trace with results
            trace.destination_ip = result.get("destination_ip")
            trace.source_ip = result.get("source_ip")
            trace.total_hops = result.get("total_hops", 0)
            trace.total_rtt_ms = result.get("total_rtt_ms")
            trace.completed = result.get("completed", False)
            trace.error_message = result.get("error_message")
            trace.completed_at = datetime.utcnow()
            
            # Save hops
            for hop_data in result.get("hops", []):
                hop = TraceHop(
                    trace_id=trace.id,
                    hop_number=hop_data["hop_number"],
                    ip_address=hop_data["ip_address"],
                    hostname=hop_data["hostname"],
                    latitude=hop_data["latitude"],
                    longitude=hop_data["longitude"],
                    city=hop_data["city"],
                    country=hop_data["country"],
                    country_code=hop_data["country_code"],
                    asn=hop_data["asn"],
                    asn_organization=hop_data["asn_organization"],
                    rtt_ms=hop_data["rtt_ms"],
                    packet_loss=hop_data["packet_loss"],
                    responded=hop_data["responded"]
                )
                db.add(hop)
            
            await db.commit()
            logger.info(f"Trace completed for test {test_id}: {trace.total_hops} hops")
            
        except Exception as e:
            logger.error(f"Error running trace for test {test_id}: {e}")


@router.post("/tests/{test_id}/trace", status_code=202)
async def create_trace(
    test_id: int,
    background_tasks: BackgroundTasks,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Start a traceroute for a test
    """
    # Check if test exists
    result = await db.execute(select(Test).where(Test.id == test_id))
    test = result.scalar_one_or_none()
    
    if not test:
        raise HTTPException(status_code=404, detail="Test not found")
    
    # Check if trace already exists
    result = await db.execute(select(Trace).where(Trace.test_id == test_id))
    existing_trace = result.scalar_one_or_none()
    
    if existing_trace:
        raise HTTPException(status_code=400, detail="Trace already exists for this test")
    
    # Start trace in background
    background_tasks.add_task(run_trace_background, test_id)
    
    return {"message": "Trace started", "test_id": test_id}


@router.get("/tests/{test_id}/trace", response_model=TraceResponse)
async def get_trace(
    test_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Get trace for a test
    """
    result = await db.execute(
        select(Trace)
        .where(Trace.test_id == test_id)
    )
    trace = result.scalar_one_or_none()
    
    if not trace:
        raise HTTPException(status_code=404, detail="Trace not found")
    
    # Get hops
    result = await db.execute(
        select(TraceHop)
        .where(TraceHop.trace_id == trace.id)
        .order_by(TraceHop.hop_number)
    )
    hops = result.scalars().all()
    
    return {
        "id": trace.id,
        "test_id": trace.test_id,
        "source_ip": trace.source_ip,
        "destination_ip": trace.destination_ip,
        "destination_host": trace.destination_host,
        "total_hops": trace.total_hops,
        "total_rtt_ms": trace.total_rtt_ms,
        "completed": trace.completed,
        "error_message": trace.error_message,
        "started_at": trace.started_at,
        "completed_at": trace.completed_at,
        "created_at": trace.created_at,
        "hops": [
            {
                "id": hop.id,
                "hop_number": hop.hop_number,
                "ip_address": hop.ip_address,
                "hostname": hop.hostname,
                "latitude": hop.latitude,
                "longitude": hop.longitude,
                "city": hop.city,
                "country": hop.country,
                "country_code": hop.country_code,
                "asn": hop.asn,
                "asn_organization": hop.asn_organization,
                "rtt_ms": hop.rtt_ms,
                "packet_loss": hop.packet_loss,
                "responded": hop.responded
            }
            for hop in hops
        ]
    }


@router.post("/traces", response_model=TraceResponse, status_code=201)
async def create_trace_direct(
    trace_data: TraceCreate,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Create a new trace directly (without test)
    """
    # Create trace record
    trace = Trace(
        destination_host=trace_data.destination,
        started_at=datetime.utcnow()
    )
    db.add(trace)
    await db.commit()
    await db.refresh(trace)
    
    # Run traceroute
    traceroute_service = TracerouteService()
    result = await traceroute_service.run_traceroute(
        trace_data.destination,
        max_hops=trace_data.max_hops,
        timeout=trace_data.timeout,
        count=trace_data.count
    )
    
    # Update trace with results
    trace.destination_ip = result.get("destination_ip")
    trace.source_ip = result.get("source_ip")
    trace.total_hops = result.get("total_hops", 0)
    trace.total_rtt_ms = result.get("total_rtt_ms")
    trace.completed = result.get("completed", False)
    trace.error_message = result.get("error_message")
    trace.completed_at = datetime.utcnow()
    
    # Save hops
    for hop_data in result.get("hops", []):
        hop = TraceHop(
            trace_id=trace.id,
            hop_number=hop_data["hop_number"],
            ip_address=hop_data["ip_address"],
            hostname=hop_data["hostname"],
            latitude=hop_data["latitude"],
            longitude=hop_data["longitude"],
            city=hop_data["city"],
            country=hop_data["country"],
            country_code=hop_data["country_code"],
            asn=hop_data["asn"],
            asn_organization=hop_data["asn_organization"],
            rtt_ms=hop_data["rtt_ms"],
            packet_loss=hop_data["packet_loss"],
            responded=hop_data["responded"]
        )
        db.add(hop)
    
    await db.commit()
    await db.refresh(trace)
    
    # Get hops
    result = await db.execute(
        select(TraceHop)
        .where(TraceHop.trace_id == trace.id)
        .order_by(TraceHop.hop_number)
    )
    hops = result.scalars().all()
    
    return {
        "id": trace.id,
        "test_id": trace.test_id,
        "source_ip": trace.source_ip,
        "destination_ip": trace.destination_ip,
        "destination_host": trace.destination_host,
        "total_hops": trace.total_hops,
        "total_rtt_ms": trace.total_rtt_ms,
        "completed": trace.completed,
        "error_message": trace.error_message,
        "started_at": trace.started_at,
        "completed_at": trace.completed_at,
        "created_at": trace.created_at,
        "hops": [
            {
                "id": hop.id,
                "hop_number": hop.hop_number,
                "ip_address": hop.ip_address,
                "hostname": hop.hostname,
                "latitude": hop.latitude,
                "longitude": hop.longitude,
                "city": hop.city,
                "country": hop.country,
                "country_code": hop.country_code,
                "asn": hop.asn,
                "asn_organization": hop.asn_organization,
                "rtt_ms": hop.rtt_ms,
                "packet_loss": hop.packet_loss,
                "responded": hop.responded
            }
            for hop in hops
        ]
    }


# Legacy endpoint - must be before /{trace_id} to avoid conflicts
@router.get("/traces/recent", response_model=List[TraceResponse])
async def get_recent_traces(
    limit: int = 10,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Get recent completed traces
    """
    result = await db.execute(
        select(Trace)
        .where(Trace.completed == True)
        .order_by(Trace.created_at.desc())
        .limit(limit)
    )
    traces = result.scalars().all()
    
    traces_data = []
    for trace in traces:
        # Get hops for each trace
        result = await db.execute(
            select(TraceHop)
            .where(TraceHop.trace_id == trace.id)
            .order_by(TraceHop.hop_number)
        )
        hops = result.scalars().all()
        
        traces_data.append({
            "id": trace.id,
            "test_id": trace.test_id,
            "source_ip": trace.source_ip,
            "destination_ip": trace.destination_ip,
            "destination_host": trace.destination_host,
            "total_hops": trace.total_hops,
            "total_rtt_ms": trace.total_rtt_ms,
            "completed": trace.completed,
            "error_message": trace.error_message,
            "started_at": trace.started_at,
            "completed_at": trace.completed_at,
            "created_at": trace.created_at,
            "hops": [
                {
                    "id": hop.id,
                    "hop_number": hop.hop_number,
                    "ip_address": hop.ip_address,
                    "hostname": hop.hostname,
                    "latitude": hop.latitude,
                    "longitude": hop.longitude,
                    "city": hop.city,
                    "country": hop.country,
                    "country_code": hop.country_code,
                    "asn": hop.asn,
                    "asn_organization": hop.asn_organization,
                    "rtt_ms": hop.rtt_ms,
                    "packet_loss": hop.packet_loss,
                    "responded": hop.responded
                }
                for hop in hops
            ]
        })
    
    return traces_data


@router.get("/traces/{trace_id}", response_model=TraceResponse)
async def get_trace_by_id(
    trace_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Get trace by ID
    """
    result = await db.execute(
        select(Trace)
        .where(Trace.id == trace_id)
    )
    trace = result.scalar_one_or_none()
    
    if not trace:
        raise HTTPException(status_code=404, detail="Trace not found")
    
    # Get hops
    result = await db.execute(
        select(TraceHop)
        .where(TraceHop.trace_id == trace.id)
        .order_by(TraceHop.hop_number)
    )
    hops = result.scalars().all()
    
    return {
        "id": trace.id,
        "test_id": trace.test_id,
        "source_ip": trace.source_ip,
        "destination_ip": trace.destination_ip,
        "destination_host": trace.destination_host,
        "total_hops": trace.total_hops,
        "total_rtt_ms": trace.total_rtt_ms,
        "completed": trace.completed,
        "error_message": trace.error_message,
        "started_at": trace.started_at,
        "completed_at": trace.completed_at,
        "created_at": trace.created_at,
        "hops": [
            {
                "id": hop.id,
                "hop_number": hop.hop_number,
                "ip_address": hop.ip_address,
                "hostname": hop.hostname,
                "latitude": hop.latitude,
                "longitude": hop.longitude,
                "city": hop.city,
                "country": hop.country,
                "country_code": hop.country_code,
                "asn": hop.asn,
                "asn_organization": hop.asn_organization,
                "rtt_ms": hop.rtt_ms,
                "packet_loss": hop.packet_loss,
                "responded": hop.responded
            }
            for hop in hops
        ]
    }


@router.get("/traces", response_model=List[TraceResponse])
async def get_all_traces(
    limit: int = 10,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Get all traces
    """
    result = await db.execute(
        select(Trace)
        .order_by(Trace.created_at.desc())
        .limit(limit)
    )
    traces = result.scalars().all()
    
    traces_data = []
    for trace in traces:
        # Get hops for each trace
        result = await db.execute(
            select(TraceHop)
            .where(TraceHop.trace_id == trace.id)
            .order_by(TraceHop.hop_number)
        )
        hops = result.scalars().all()
        
        traces_data.append({
            "id": trace.id,
            "test_id": trace.test_id,
            "source_ip": trace.source_ip,
            "destination_ip": trace.destination_ip,
            "destination_host": trace.destination_host,
            "total_hops": trace.total_hops,
            "total_rtt_ms": trace.total_rtt_ms,
            "completed": trace.completed,
            "error_message": trace.error_message,
            "started_at": trace.started_at,
            "completed_at": trace.completed_at,
            "created_at": trace.created_at,
            "hops": [
                {
                    "id": hop.id,
                    "hop_number": hop.hop_number,
                    "ip_address": hop.ip_address,
                    "hostname": hop.hostname,
                    "latitude": hop.latitude,
                    "longitude": hop.longitude,
                    "city": hop.city,
                    "country": hop.country,
                    "country_code": hop.country_code,
                    "asn": hop.asn,
                    "asn_organization": hop.asn_organization,
                    "rtt_ms": hop.rtt_ms,
                    "packet_loss": hop.packet_loss,
                    "responded": hop.responded
                }
                for hop in hops
            ]
        })
    
    return traces_data


@router.get("/traces/test/{test_id}", response_model=List[TraceResponse])
async def get_traces_by_test(
    test_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Get all traces for a specific test
    """
    result = await db.execute(
        select(Trace)
        .where(Trace.test_id == test_id)
        .order_by(Trace.created_at.desc())
    )
    traces = result.scalars().all()
    
    traces_data = []
    for trace in traces:
        # Get hops for each trace
        result = await db.execute(
            select(TraceHop)
            .where(TraceHop.trace_id == trace.id)
            .order_by(TraceHop.hop_number)
        )
        hops = result.scalars().all()
        
        traces_data.append({
            "id": trace.id,
            "test_id": trace.test_id,
            "source_ip": trace.source_ip,
            "destination_ip": trace.destination_ip,
            "destination_host": trace.destination_host,
            "total_hops": trace.total_hops,
            "total_rtt_ms": trace.total_rtt_ms,
            "completed": trace.completed,
            "error_message": trace.error_message,
            "started_at": trace.started_at,
            "completed_at": trace.completed_at,
            "created_at": trace.created_at,
            "hops": [
                {
                    "id": hop.id,
                    "hop_number": hop.hop_number,
                    "ip_address": hop.ip_address,
                    "hostname": hop.hostname,
                    "latitude": hop.latitude,
                    "longitude": hop.longitude,
                    "city": hop.city,
                    "country": hop.country,
                    "country_code": hop.country_code,
                    "asn": hop.asn,
                    "asn_organization": hop.asn_organization,
                    "rtt_ms": hop.rtt_ms,
                    "packet_loss": hop.packet_loss,
                    "responded": hop.responded
                }
                for hop in hops
            ]
        })
    
    return traces_data


@router.delete("/traces/{trace_id}")
async def delete_trace_by_id(
    trace_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Delete a trace by ID
    """
    result = await db.execute(select(Trace).where(Trace.id == trace_id))
    trace = result.scalar_one_or_none()
    
    if not trace:
        raise HTTPException(status_code=404, detail="Trace not found")
    
    await db.delete(trace)
    await db.commit()
    
    return {"message": "Trace deleted"}


@router.delete("/tests/{test_id}/trace")
async def delete_trace(
    test_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Delete a trace
    """
    result = await db.execute(select(Trace).where(Trace.test_id == test_id))
    trace = result.scalar_one_or_none()
    
    if not trace:
        raise HTTPException(status_code=404, detail="Trace not found")
    
    await db.delete(trace)
    await db.commit()
    
    return {"message": "Trace deleted"}
