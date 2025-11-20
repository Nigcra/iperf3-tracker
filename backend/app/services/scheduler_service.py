from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.interval import IntervalTrigger
from datetime import datetime
import logging
import asyncio
from sqlalchemy.ext.asyncio import AsyncSession
from app.core.database import AsyncSessionLocal
from app.models.models import Server
from app.services.iperf_service import TestService
from app.services.traceroute_service import TracerouteService
from sqlalchemy import select

logger = logging.getLogger(__name__)


class SchedulerService:
    """Service for scheduling automatic tests"""
    
    def __init__(self):
        self.scheduler = AsyncIOScheduler()
        self.jobs = {}
    
    def start(self):
        """Start the scheduler"""
        if not self.scheduler.running:
            self.scheduler.start()
            logger.info("Scheduler started")
    
    def shutdown(self):
        """Shutdown the scheduler"""
        if self.scheduler.running:
            self.scheduler.shutdown()
            logger.info("Scheduler stopped")
    
    async def schedule_server_tests(self, server_id: int, interval_minutes: int):
        """Schedule periodic tests for a server"""
        job_id = f"server_{server_id}"
        
        # Remove existing job if any
        if job_id in self.jobs:
            self.scheduler.remove_job(job_id)
        
        # Add new job
        self.scheduler.add_job(
            self._run_scheduled_test,
            trigger=IntervalTrigger(minutes=interval_minutes),
            args=[server_id],
            id=job_id,
            name=f"Test for server {server_id}",
            replace_existing=True
        )
        
        self.jobs[job_id] = True
        logger.info(f"Scheduled tests for server {server_id} every {interval_minutes} minutes")
    
    async def unschedule_server_tests(self, server_id: int):
        """Remove scheduled tests for a server"""
        job_id = f"server_{server_id}"
        
        if job_id in self.jobs:
            self.scheduler.remove_job(job_id)
            del self.jobs[job_id]
            logger.info(f"Unscheduled tests for server {server_id}")
    
    async def _run_scheduled_test(self, server_id: int):
        """Run a scheduled test for a server"""
        async with AsyncSessionLocal() as db:
            try:
                # Get server configuration
                result = await db.execute(select(Server).where(Server.id == server_id))
                server = result.scalar_one_or_none()
                
                if not server or not server.enabled or not server.schedule_enabled:
                    logger.warning(f"Server {server_id} not found or disabled, skipping test")
                    return
                
                logger.info(f"Running scheduled test for server: {server.name}")
                
                # Run test using the service
                test_service = TestService(db)
                test = await test_service.create_and_run_test(
                    server_id=server_id,
                    duration=server.default_duration,
                    parallel_streams=server.default_parallel,
                    protocol=server.default_protocol,
                    direction=server.default_direction
                )
                
                logger.info(f"Scheduled test completed for server: {server.name}")
                
                # Run automatic traceroute if enabled
                if server.auto_trace_enabled:
                    logger.info(f"Running automatic traceroute for server: {server.name}")
                    try:
                        traceroute_service = TracerouteService(db)
                        
                        # Run traceroute with 30-second timeout
                        trace_task = asyncio.create_task(
                            traceroute_service.run_traceroute(
                                destination=server.host,
                                test_id=test.id if test else None,
                                max_hops=30,
                                timeout_per_hop=1.0
                            )
                        )
                        
                        # Wait for traceroute with 30-second timeout
                        await asyncio.wait_for(trace_task, timeout=30.0)
                        logger.info(f"Automatic traceroute completed for server: {server.name}")
                        
                    except asyncio.TimeoutError:
                        logger.warning(f"Automatic traceroute timed out for server: {server.name}")
                    except Exception as e:
                        logger.error(f"Error running automatic traceroute for server {server.name}: {e}")
                
            except Exception as e:
                logger.error(f"Error running scheduled test for server {server_id}: {e}")
    
    async def sync_scheduled_servers(self):
        """Sync scheduler with database - schedule all enabled servers"""
        async with AsyncSessionLocal() as db:
            try:
                result = await db.execute(
                    select(Server).where(Server.enabled == True, Server.schedule_enabled == True)
                )
                servers = result.scalars().all()
                
                for server in servers:
                    await self.schedule_server_tests(
                        server.id,
                        server.schedule_interval_minutes
                    )
                
                logger.info(f"Synced {len(servers)} scheduled servers")
                
            except Exception as e:
                logger.error(f"Error syncing scheduled servers: {e}")


# Global scheduler instance
scheduler_service = SchedulerService()
