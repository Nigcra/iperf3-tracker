from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, delete
from typing import List, Optional
from app.core.database import get_db
from app.models.models import Server
from app.schemas.schemas import ServerCreate, ServerUpdate, ServerResponse
from app.services.scheduler_service import scheduler_service

router = APIRouter(prefix="/servers", tags=["servers"])


@router.get("", response_model=List[ServerResponse])
async def list_servers(
    enabled: Optional[bool] = None,
    skip: int = Query(0, ge=0),
    limit: int = Query(100, ge=1, le=1000),
    db: AsyncSession = Depends(get_db)
):
    """List all server profiles"""
    query = select(Server)
    
    if enabled is not None:
        query = query.where(Server.enabled == enabled)
    
    query = query.offset(skip).limit(limit).order_by(Server.created_at.desc())
    
    result = await db.execute(query)
    servers = result.scalars().all()
    
    return servers


@router.get("/{server_id}", response_model=ServerResponse)
async def get_server(
    server_id: int,
    db: AsyncSession = Depends(get_db)
):
    """Get a specific server by ID"""
    result = await db.execute(select(Server).where(Server.id == server_id))
    server = result.scalar_one_or_none()
    
    if not server:
        raise HTTPException(status_code=404, detail="Server not found")
    
    return server


@router.post("", response_model=ServerResponse, status_code=201)
async def create_server(
    server: ServerCreate,
    db: AsyncSession = Depends(get_db)
):
    """Create a new server profile"""
    # Check if name already exists
    result = await db.execute(select(Server).where(Server.name == server.name))
    if result.scalar_one_or_none():
        raise HTTPException(status_code=400, detail="Server with this name already exists")
    
    db_server = Server(**server.model_dump())
    db.add(db_server)
    await db.commit()
    await db.refresh(db_server)
    
    # Schedule tests if enabled
    if db_server.schedule_enabled:
        await scheduler_service.schedule_server_tests(
            db_server.id,
            db_server.schedule_interval_minutes
        )
    
    return db_server


@router.put("/{server_id}", response_model=ServerResponse)
async def update_server(
    server_id: int,
    server_update: ServerUpdate,
    db: AsyncSession = Depends(get_db)
):
    """Update a server profile"""
    result = await db.execute(select(Server).where(Server.id == server_id))
    db_server = result.scalar_one_or_none()
    
    if not db_server:
        raise HTTPException(status_code=404, detail="Server not found")
    
    # Update fields
    update_data = server_update.model_dump(exclude_unset=True)
    
    # Check name uniqueness if changing name
    if "name" in update_data and update_data["name"] != db_server.name:
        result = await db.execute(select(Server).where(Server.name == update_data["name"]))
        if result.scalar_one_or_none():
            raise HTTPException(status_code=400, detail="Server with this name already exists")
    
    for field, value in update_data.items():
        setattr(db_server, field, value)
    
    await db.commit()
    await db.refresh(db_server)
    
    # Update scheduling
    if db_server.schedule_enabled:
        await scheduler_service.schedule_server_tests(
            db_server.id,
            db_server.schedule_interval_minutes
        )
    else:
        await scheduler_service.unschedule_server_tests(db_server.id)
    
    return db_server


@router.delete("/{server_id}", status_code=204)
async def delete_server(
    server_id: int,
    db: AsyncSession = Depends(get_db)
):
    """Delete a server profile"""
    result = await db.execute(select(Server).where(Server.id == server_id))
    db_server = result.scalar_one_or_none()
    
    if not db_server:
        raise HTTPException(status_code=404, detail="Server not found")
    
    # Unschedule tests
    await scheduler_service.unschedule_server_tests(server_id)
    
    await db.execute(delete(Server).where(Server.id == server_id))
    await db.commit()
    
    return None
