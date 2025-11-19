import subprocess
import json
import logging
import os
import shutil
import asyncio
import re
from datetime import datetime
from typing import Optional, Dict, Any, Callable
from app.models.models import ProtocolType, TestDirection, TestStatus

logger = logging.getLogger(__name__)

# Global storage for active tests
active_tests = {}

# Global lock to ensure only one test runs at a time
test_lock = asyncio.Lock()


class IperfService:
    """Service for running iperf3 tests"""
    
    @staticmethod
    def _get_iperf3_command():
        """Get the iperf3 command path (cross-platform)"""
        # Try to find iperf3 in PATH first (works on Linux/Docker and Windows if in PATH)
        iperf3_path = shutil.which("iperf3")
        if iperf3_path:
            logger.debug(f"Found iperf3 in PATH: {iperf3_path}")
            return iperf3_path
        
        # Windows-specific fallback locations (only checked on Windows)
        if os.name == 'nt':  # Windows
            possible_paths = [
                os.path.join(os.environ.get('LOCALAPPDATA', ''), 
                            'Microsoft', 'WinGet', 'Packages', 
                            'ar51an.iPerf3_Microsoft.Winget.Source_8wekyb3d8bbwe', 'iperf3.exe'),
                r"C:\Program Files\iperf3\iperf3.exe",
                r"C:\iperf3\iperf3.exe",
            ]
            
            for path in possible_paths:
                if os.path.exists(path):
                    logger.info(f"Found iperf3 at Windows location: {path}")
                    return path
        
        # Fallback to just "iperf3" - will work in Docker where it's installed via apt
        logger.debug("Using default 'iperf3' command")
        return "iperf3"
    
    @staticmethod
    def get_active_test(test_id: int) -> Optional[Dict[str, Any]]:
        """Get live data from an active test"""
        return active_tests.get(test_id)
    
    @staticmethod
    async def run_test(
        host: str,
        port: int,
        duration: int,
        parallel_streams: int,
        protocol: ProtocolType,
        direction: TestDirection,
        test_id: Optional[int] = None
    ) -> Dict[str, Any]:
        """
        Run an iperf3 test
        
        Args:
            host: Server hostname or IP
            port: Server port
            duration: Test duration in seconds
            parallel_streams: Number of parallel streams
            protocol: TCP or UDP
            direction: Download, Upload, or Bidirectional
            test_id: Optional test ID for live tracking
            
        Returns:
            Dictionary with test results and raw output
        """
        try:
            # Initialize active test tracking if test_id provided
            if test_id is not None:
                active_tests[test_id] = {
                    "status": "running",
                    "progress": 0,
                    "current_download_mbps": 0,
                    "current_upload_mbps": 0,
                    "elapsed_seconds": 0,
                    "total_seconds": duration
                }
            
            # Get iperf3 command
            iperf3_cmd = IperfService._get_iperf3_command()
            
            # Build iperf3 command
            # Use --forceflush to get immediate output
            cmd = [
                iperf3_cmd,
                "-c", host,
                "-p", str(port),
                "-t", str(duration),
                "-P", str(parallel_streams),
                "-i", "1",  # Interval updates every second
                "--forceflush",  # Force flushing output
            ]
            
            # Add protocol flag
            if protocol == ProtocolType.UDP:
                cmd.append("-u")
            
            # Add direction flag
            if direction == TestDirection.UPLOAD:
                cmd.append("-R")
            elif direction == TestDirection.BIDIRECTIONAL:
                cmd.append("--bidir")
            
            logger.info(f"Running iperf3 command: {' '.join(cmd)}")
            
            # Run the command
            # On Windows, we need unbuffered output
            import os
            env = os.environ.copy()
            env['PYTHONUNBUFFERED'] = '1'
            
            process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                bufsize=0,  # Unbuffered
                env=env
            )
            
            # Read output line by line for live updates
            output_lines = []
            stderr_lines = []
            
            if test_id is not None:
                # Non-blocking read with live updates
                import select
                import sys
                
                # For Windows compatibility, we'll use threading
                if sys.platform == 'win32':
                    import threading
                    
                    def read_stdout():
                        for line in iter(process.stdout.readline, ''):
                            if line:
                                output_lines.append(line)
                                logger.info(f"[IPERF OUTPUT] {line.strip()}")  # Log every line for debugging
                                # Parse live bandwidth from normal iperf3 output
                                if test_id and test_id in active_tests:
                                    try:
                                        # Match [SUM] lines (for multi-stream) OR individual stream lines like [  5]
                                        # Also match [SUM][TX-C] and [SUM][RX-C] for bidirectional tests
                                        # Pattern: [SUM]   0.00-1.00   sec  43.0 MBytes   360 Mbits/sec
                                        # Or:      [SUM][TX-C]   0.00-1.00   sec  43.0 MBytes   360 Mbits/sec
                                        # Or:      [SUM][RX-C]   0.00-1.00   sec  98.5 MBytes   825 Mbits/sec
                                        match = re.search(r'\[(?:SUM|\s*\d+)\](?:\[([TR]X-C)\])?\s+([\d.]+)-([\d.]+)\s+sec\s+([\d.]+)\s+([KMG])Bytes\s+([\d.]+)\s+([KMG])bits/sec', line)
                                        
                                        if match:
                                            role = match.group(1)  # TX-C or RX-C or None
                                            speed = float(match.group(6))
                                            unit = match.group(7)
                                            
                                            # Convert to Mbps
                                            if unit == 'K':
                                                mbps = speed / 1000
                                            elif unit == 'G':
                                                mbps = speed * 1000
                                            else:  # M
                                                mbps = speed
                                            
                                            # Skip lines with "sender" or "receiver" (those are final summary)
                                            if 'sender' in line or 'receiver' in line:
                                                continue
                                            
                                            # Update based on direction and role
                                            if direction == TestDirection.BIDIRECTIONAL:
                                                # TX-C = upload (we're sending), RX-C = download (we're receiving)
                                                if role == 'TX-C':
                                                    active_tests[test_id]["current_upload_mbps"] = mbps
                                                elif role == 'RX-C':
                                                    active_tests[test_id]["current_download_mbps"] = mbps
                                                elif role is None:
                                                    # Generic sum - use for download
                                                    active_tests[test_id]["current_download_mbps"] = mbps
                                            elif direction == TestDirection.UPLOAD:
                                                active_tests[test_id]["current_upload_mbps"] = mbps
                                            else:  # DOWNLOAD
                                                active_tests[test_id]["current_download_mbps"] = mbps
                                            
                                            logger.info(f"✅ Live update matched: {mbps:.2f} Mbps (direction: {direction}, role: {role})")
                                    except Exception as e:
                                        logger.error(f"❌ Error parsing live bandwidth: {e}")
                    
                    def read_stderr():
                        for line in iter(process.stderr.readline, ''):
                            if line:
                                stderr_lines.append(line)
                    
                    stdout_thread = threading.Thread(target=read_stdout)
                    stderr_thread = threading.Thread(target=read_stderr)
                    stdout_thread.daemon = True
                    stderr_thread.daemon = True
                    stdout_thread.start()
                    stderr_thread.start()
                    
                    # Monitor progress
                    start_time = datetime.utcnow()
                    while process.poll() is None:
                        await asyncio.sleep(0.5)
                        elapsed = (datetime.utcnow() - start_time).total_seconds()
                        
                        # Update progress
                        if test_id in active_tests:
                            active_tests[test_id]["elapsed_seconds"] = int(elapsed)
                            active_tests[test_id]["progress"] = min(100, int((elapsed / duration) * 100))
                    
                    stdout_thread.join(timeout=1)
                    stderr_thread.join(timeout=1)
                    stdout = ''.join(output_lines)
                    stderr = ''.join(stderr_lines)
                else:
                    # Linux/Mac non-blocking approach
                    stdout, stderr = process.communicate(timeout=duration + 30)
            else:
                # Simple blocking approach without live updates
                stdout, stderr = process.communicate(timeout=duration + 30)
            
            if process.returncode != 0:
                error_msg = stderr.strip() if stderr else "iperf3 command failed"
                logger.error(f"iperf3 error (return code {process.returncode}): {error_msg}")
                logger.error(f"iperf3 stdout: {stdout[:500] if stdout else '(empty)'}")
                logger.error(f"iperf3 stderr: {stderr[:500] if stderr else '(empty)'}")
                
                # Update active test to show error
                if test_id is not None and test_id in active_tests:
                    active_tests[test_id]["status"] = "failed"
                
                return {
                    "status": TestStatus.FAILED,
                    "error_message": error_msg,
                    "raw_output": stdout or stderr
                }
            
            # Parse text output for final results
            logger.info(f"Parsing iperf3 output ({len(stdout)} chars)")
            logger.info(f"First 500 chars: {stdout[:500]}")
            logger.info(f"Last 500 chars: {stdout[-500:]}")
            
            metrics = IperfService._parse_text_output(stdout, direction)
            metrics["status"] = TestStatus.COMPLETED
            metrics["raw_output"] = stdout
            
            logger.info(f"Parsed metrics: {metrics}")
            
            # Update active test with final values before returning
            if test_id is not None and test_id in active_tests:
                active_tests[test_id]["status"] = "completed"
                active_tests[test_id]["progress"] = 100
                active_tests[test_id]["elapsed_seconds"] = duration
                # Update final bandwidth values
                if metrics.get("download_bandwidth_mbps"):
                    active_tests[test_id]["current_download_mbps"] = metrics["download_bandwidth_mbps"]
                if metrics.get("upload_bandwidth_mbps"):
                    active_tests[test_id]["current_upload_mbps"] = metrics["upload_bandwidth_mbps"]
            
            return metrics
            
        except subprocess.TimeoutExpired:
            logger.error("iperf3 test timeout")
            if test_id is not None and test_id in active_tests:
                active_tests[test_id]["status"] = "failed"
            return {
                "status": TestStatus.FAILED,
                "error_message": "Test timeout"
            }
        except FileNotFoundError:
            logger.error("iperf3 not found - ensure it's installed")
            if test_id is not None and test_id in active_tests:
                active_tests[test_id]["status"] = "failed"
            return {
                "status": TestStatus.FAILED,
                "error_message": "iperf3 not installed or not in PATH"
            }
        except Exception as e:
            logger.error(f"Unexpected error running iperf3: {e}")
            if test_id is not None and test_id in active_tests:
                active_tests[test_id]["status"] = "failed"
            return {
                "status": TestStatus.FAILED,
                "error_message": str(e)
            }
    
    @staticmethod
    def _parse_text_output(output: str, direction: TestDirection) -> Dict[str, Any]:
        """Parse iperf3 text output for final results"""
        metrics = {}
        
        try:
            lines = output.split('\n')
            
            # Look for final summary lines with "sender" or "receiver"
            # Format for normal tests: [  5]   0.00-10.00  sec   368 MBytes   309 Mbits/sec                  sender
            # Format for bidirectional: [SUM][TX-C]   0.00-10.00  sec   419 MBytes   351 Mbits/sec                  sender
            #                          [SUM][RX-C]   0.00-10.00  sec  1.10 GBytes   949 Mbits/sec  26979             sender
            for line in lines:
                # Match final summary lines with sender/receiver
                # Also handle GBytes
                match = re.search(r'\[(?:SUM|\s*\d+)\](?:\[([TR]X-C)\])?\s+([\d.]+)-([\d.]+)\s+sec\s+([\d.]+)\s+([KMG])Bytes\s+([\d.]+)\s+([KMG])bits/sec.*?(sender|receiver)', line)
                if match:
                    role = match.group(1)  # TX-C or RX-C or None
                    speed = float(match.group(6))
                    unit = match.group(7)
                    sender_receiver = match.group(8)
                    
                    # Convert to Mbps
                    if unit == 'K':
                        mbps = speed / 1000
                    elif unit == 'G':
                        mbps = speed * 1000
                    else:  # M
                        mbps = speed
                    
                    # Assign based on direction, role, and sender/receiver
                    if direction == TestDirection.BIDIRECTIONAL:
                        # For bidirectional: TX-C = upload, RX-C = download
                        # We want the "receiver" line for accurate measurement
                        if role == 'TX-C' and sender_receiver == 'receiver':
                            metrics["upload_bandwidth_mbps"] = mbps
                        elif role == 'RX-C' and sender_receiver == 'receiver':
                            metrics["download_bandwidth_mbps"] = mbps
                    elif direction == TestDirection.UPLOAD:
                        if sender_receiver == 'sender':
                            metrics["upload_bandwidth_mbps"] = mbps
                    else:  # DOWNLOAD
                        if sender_receiver == 'receiver':
                            metrics["download_bandwidth_mbps"] = mbps
            
            # Set defaults if not found
            if "download_bandwidth_mbps" not in metrics and direction in [TestDirection.DOWNLOAD, TestDirection.BIDIRECTIONAL]:
                metrics["download_bandwidth_mbps"] = 0
            if "upload_bandwidth_mbps" not in metrics and direction in [TestDirection.UPLOAD, TestDirection.BIDIRECTIONAL]:
                metrics["upload_bandwidth_mbps"] = 0
                
        except Exception as e:
            logger.warning(f"Error parsing text output: {e}")
            metrics["download_bandwidth_mbps"] = 0
            metrics["upload_bandwidth_mbps"] = 0
        
        return metrics


class TestService:
    """Service for managing tests"""
    
    def __init__(self, db_session):
        self.db = db_session
    
    async def create_test(
        self,
        server_id: int,
        duration: int,
        parallel_streams: int,
        protocol: ProtocolType,
        direction: TestDirection
    ):
        """Create a test record without running it"""
        from app.models.models import Server, Test
        from sqlalchemy import select
        
        # Get server
        result = await self.db.execute(select(Server).where(Server.id == server_id))
        server = result.scalar_one_or_none()
        
        if not server:
            raise ValueError(f"Server {server_id} not found")
        
        if not server.enabled:
            raise ValueError(f"Server {server.name} is disabled")
        
        # Create test record with PENDING status
        test = Test(
            server_id=server_id,
            protocol=protocol,
            direction=direction,
            duration=duration,
            parallel_streams=parallel_streams,
            status=TestStatus.PENDING
        )
        
        self.db.add(test)
        await self.db.commit()
        await self.db.refresh(test)
        
        return test
    
    async def run_test_background(self, test_id: int):
        """Run a test in the background (for use with BackgroundTasks)"""
        from app.models.models import Server, Test
        from sqlalchemy import select
        from app.core.database import AsyncSessionLocal
        
        # Set test as pending in active_tests before acquiring lock
        active_tests[test_id] = {
            "status": "pending",
            "progress": 0,
            "current_download_mbps": 0,
            "current_upload_mbps": 0,
            "elapsed_seconds": 0,
            "total_seconds": 0
        }
        logger.info(f"Test {test_id} waiting in queue (pending)")
        
        # Acquire lock to ensure only one test runs at a time
        async with test_lock:
            logger.info(f"Acquired test lock for test {test_id}")
            
            # Create a new database session for background task
            async with AsyncSessionLocal() as db:
                # Get test
                result = await db.execute(select(Test).where(Test.id == test_id))
                test = result.scalar_one_or_none()
                
                if not test:
                    logger.error(f"Test {test_id} not found")
                    if test_id in active_tests:
                        del active_tests[test_id]
                    return
                
                # Get server
                result = await db.execute(select(Server).where(Server.id == test.server_id))
                server = result.scalar_one_or_none()
                
                if not server:
                    logger.error(f"Server {test.server_id} not found")
                    test.status = TestStatus.FAILED
                    test.error_message = "Server not found"
                    test.completed_at = datetime.utcnow()
                    await db.commit()
                    if test_id in active_tests:
                        active_tests[test_id]["status"] = "failed"
                    return
                
                # Update status to RUNNING (now that we have the lock)
                test.status = TestStatus.RUNNING
                if test_id in active_tests:
                    active_tests[test_id]["status"] = "running"
                test.started_at = datetime.utcnow()
                await db.commit()
                
                # Run the test
                result = await IperfService.run_test(
                    host=server.host,
                    port=server.port,
                    duration=test.duration,
                    parallel_streams=test.parallel_streams,
                    protocol=test.protocol,
                    direction=test.direction,
                    test_id=test.id
                )
                
                # Update test with results
                test.status = result.get("status", TestStatus.FAILED)
                test.completed_at = datetime.utcnow()
                test.download_bandwidth_mbps = result.get("download_bandwidth_mbps")
                test.download_bytes = result.get("download_bytes")
                test.download_jitter_ms = result.get("download_jitter_ms")
                test.download_packet_loss_percent = result.get("download_packet_loss_percent")
                test.upload_bandwidth_mbps = result.get("upload_bandwidth_mbps")
                test.upload_bytes = result.get("upload_bytes")
                test.upload_jitter_ms = result.get("upload_jitter_ms")
                test.upload_packet_loss_percent = result.get("upload_packet_loss_percent")
                test.retransmits = result.get("retransmits")
                test.cpu_percent = result.get("cpu_percent")
                test.error_message = result.get("error_message")
                test.raw_output = result.get("raw_output")
                
                await db.commit()
                
                # Keep active_tests entry for 10 seconds so frontend can fetch final values
                # This is especially important for tests that fail immediately
                await asyncio.sleep(10)
                
                # Now clean up active test tracking
                if test.id in active_tests:
                    del active_tests[test.id]
            
            logger.info(f"Released test lock for test {test_id}")
    
    async def create_and_run_test(
        self,
        server_id: int,
        duration: int,
        parallel_streams: int,
        protocol: ProtocolType,
        direction: TestDirection
    ) -> Dict[str, Any]:
        """Create and execute a test"""
        from app.models.models import Server, Test
        from sqlalchemy import select
        
        # Create test record first (before lock)
        result = await self.db.execute(select(Server).where(Server.id == server_id))
        server = result.scalar_one_or_none()
        
        if not server:
            raise ValueError(f"Server {server_id} not found")
        
        if not server.enabled:
            raise ValueError(f"Server {server.name} is disabled")
        
        # Create test record with PENDING status
        test = Test(
            server_id=server_id,
            protocol=protocol,
            direction=direction,
            duration=duration,
            parallel_streams=parallel_streams,
            status=TestStatus.PENDING,
            started_at=datetime.utcnow()
        )
        
        self.db.add(test)
        await self.db.commit()
        await self.db.refresh(test)
        
        # Set as pending in active_tests before acquiring lock
        active_tests[test.id] = {
            "status": "pending",
            "progress": 0,
            "current_download_mbps": 0,
            "current_upload_mbps": 0,
            "elapsed_seconds": 0,
            "total_seconds": duration
        }
        logger.info(f"Test {test.id} (scheduled) waiting in queue (pending)")
        
        # Acquire lock to ensure only one test runs at a time
        async with test_lock:
            logger.info(f"Acquired test lock for scheduled test {test.id} on server {server_id}")
            
            # Update status to RUNNING now that we have the lock
            test.status = TestStatus.RUNNING
            await self.db.commit()
            
            # Run the test
            result = await IperfService.run_test(
                host=server.host,
                port=server.port,
                duration=duration,
                parallel_streams=parallel_streams,
                protocol=protocol,
                direction=direction,
                test_id=test.id  # Pass test ID for live tracking
            )
            
            # Update test with results
            test.status = result.get("status", TestStatus.FAILED)
            test.completed_at = datetime.utcnow()
            test.download_bandwidth_mbps = result.get("download_bandwidth_mbps")
            test.download_bytes = result.get("download_bytes")
            test.download_jitter_ms = result.get("download_jitter_ms")
            test.download_packet_loss_percent = result.get("download_packet_loss_percent")
            test.upload_bandwidth_mbps = result.get("upload_bandwidth_mbps")
            test.upload_bytes = result.get("upload_bytes")
            test.upload_jitter_ms = result.get("upload_jitter_ms")
            test.upload_packet_loss_percent = result.get("upload_packet_loss_percent")
            test.retransmits = result.get("retransmits")
            test.cpu_percent = result.get("cpu_percent")
            test.error_message = result.get("error_message")
            test.raw_output = result.get("raw_output")
            
            await self.db.commit()
            await self.db.refresh(test)
            
            # Keep active_tests entry for 10 seconds so frontend can fetch final values
            await asyncio.sleep(10)
            
            # Now clean up active test tracking
            if test.id in active_tests:
                del active_tests[test.id]
            
            logger.info(f"Released test lock for scheduled test on server {server_id}")
            
            return test
