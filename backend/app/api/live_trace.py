from fastapi import APIRouter, Depends, Query
from fastapi.responses import StreamingResponse
from sqlalchemy.ext.asyncio import AsyncSession
from app.core.database import get_db
from app.core.auth import get_current_user
from app.models.models import User
from app.services.traceroute_service import TracerouteService
import asyncio
import json
import logging
import subprocess
import re

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/live-trace", tags=["live-trace"])


@router.get("/stream/{destination}")
async def stream_traceroute(
    destination: str,
    token: str = Query(..., description="Authentication token"),
    db: AsyncSession = Depends(get_db)
):
    """
    Stream traceroute progress in real-time using Server-Sent Events
    Token is passed as query parameter since EventSource doesn't support custom headers
    """
    
    # Verify token manually
    from app.core.auth import verify_token
    try:
        user = await verify_token(token, db)
        if not user:
            return StreamingResponse(
                iter([f"data: {json.dumps({'type': 'error', 'message': 'Unauthorized'})}\n\n"]),
                media_type="text/event-stream"
            )
    except Exception as e:
        return StreamingResponse(
            iter([f"data: {json.dumps({'type': 'error', 'message': 'Invalid token'})}\n\n"]),
            media_type="text/event-stream"
        )
    
    async def event_generator():
        """Generate SSE events for traceroute progress"""
        try:
            # Send start event
            yield f"data: {json.dumps({'type': 'start', 'destination': destination})}\n\n"
            
            # Create traceroute service
            trace_service = TracerouteService()
            
            # Store hops for database saving
            all_hops = []
            
            # Run tracert and stream hops as they come in
            loop = asyncio.get_event_loop()
            
            async def run_tracert_streaming():
                """Run tracert and yield hops progressively"""
                try:
                    # Start tracert process using Popen for real-time output
                    loop = asyncio.get_event_loop()
                    
                    def run_tracert_sync():
                        """Run tracert synchronously and return lines as they come"""
                        import subprocess
                        process = subprocess.Popen(
                            ['tracert', '-h', '30', '-w', '2000', '-d', destination],
                            stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE,
                            text=True,
                            encoding='cp850',
                            errors='ignore',
                            bufsize=1  # Line buffered
                        )
                        return process
                    
                    # Start process in executor
                    process = await loop.run_in_executor(None, run_tracert_sync)
                    
                    # Read lines as they come
                    while True:
                        line = await loop.run_in_executor(None, process.stdout.readline)
                        if not line:
                            break
                        
                        line = line.strip()
                        if not line:
                            continue
                        
                        # Match hop lines
                        match = re.match(r'^\s*(\d+)\s+(.+)$', line)
                        if not match:
                            continue
                        
                        hop_num = int(match.group(1))
                        hop_data = match.group(2)
                        
                        # Parse hop data
                        hop_info = {'hop_number': hop_num}
                        
                        # Check for timeout
                        if 'Request timed out' in hop_data or 'Zeitüberschreitung' in hop_data or '*' in hop_data[:20]:
                            hop_info['responded'] = False
                            hop_info['ip_address'] = None
                            hop_info['hostname'] = None
                            hop_info['rtt_ms'] = None
                        else:
                            # Extract IP
                            ip_match = re.search(r'((?:\d{1,3}\.){3}\d{1,3})', hop_data)
                            if ip_match:
                                ip_address = ip_match.group(1)
                                hop_info['responded'] = True
                                hop_info['ip_address'] = ip_address
                                
                                # Extract RTT
                                rtt_matches = re.findall(r'(\d+)\s*ms', hop_data)
                                if rtt_matches:
                                    hop_info['rtt_ms'] = float(rtt_matches[0])
                                else:
                                    hop_info['rtt_ms'] = None
                                
                                # Try to resolve hostname
                                hostname = await trace_service.resolve_hostname(ip_address)
                                hop_info['hostname'] = hostname
                                
                                # Get GeoIP info
                                geoip_info = trace_service.get_geoip_info(ip_address)
                                if geoip_info:
                                    hop_info.update(geoip_info)
                        
                        # Yield hop update
                        yield hop_info
                    
                    # Wait for process to finish
                    await loop.run_in_executor(None, process.wait)
                    
                except Exception as e:
                    logger.error(f"Error in streaming tracert: {e}")
                    raise
            
            # Stream hops
            async for hop in run_tracert_streaming():
                all_hops.append(hop)
                yield f"data: {json.dumps({'type': 'hop', 'data': hop})}\n\n"
            
            # Save trace to database
            try:
                from app.models.models import Trace, TraceHop
                from datetime import datetime
                
                # Create trace record
                trace = Trace(
                    destination_host=destination,
                    destination_ip=all_hops[-1]['ip_address'] if all_hops and all_hops[-1].get('ip_address') else None,
                    total_hops=len(all_hops),
                    total_rtt_ms=sum(h.get('rtt_ms', 0) or 0 for h in all_hops),
                    completed=True,
                    created_at=datetime.utcnow()
                )
                db.add(trace)
                await db.flush()  # Get trace.id
                
                # Create hop records
                for hop_data in all_hops:
                    hop = TraceHop(
                        trace_id=trace.id,
                        hop_number=hop_data['hop_number'],
                        ip_address=hop_data.get('ip_address'),
                        hostname=hop_data.get('hostname'),
                        rtt_ms=hop_data.get('rtt_ms'),
                        responded=hop_data.get('responded', False),
                        latitude=hop_data.get('latitude'),
                        longitude=hop_data.get('longitude'),
                        city=hop_data.get('city'),
                        country=hop_data.get('country'),
                        country_code=hop_data.get('country_code'),
                        asn=hop_data.get('asn'),
                        asn_organization=hop_data.get('asn_organization')
                    )
                    db.add(hop)
                
                await db.commit()
                logger.info(f"Saved trace {trace.id} with {len(all_hops)} hops to database")
                
            except Exception as e:
                logger.error(f"Error saving trace to database: {e}")
                await db.rollback()
            
            # Send complete event
            yield f"data: {json.dumps({'type': 'complete'})}\n\n"
                    
        except Exception as e:
            logger.error(f"Error in SSE stream: {e}")
            yield f"data: {json.dumps({'type': 'error', 'message': str(e)})}\n\n"
    
    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no"
        }
    )
