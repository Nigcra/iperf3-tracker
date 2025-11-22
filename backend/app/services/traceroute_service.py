import asyncio
import logging
import socket
import re
import subprocess
import platform
import shutil
from typing import List, Dict, Any, Optional
from datetime import datetime
import geoip2.database
import geoip2.errors
from pathlib import Path

logger = logging.getLogger(__name__)


class TracerouteService:
    """Service for performing traceroutes and geolocation lookups"""
    
    def __init__(self):
        self.geoip_reader = None
        self._init_geoip()
    
    def _init_geoip(self):
        """Initialize GeoIP database"""
        # Try multiple possible locations for GeoIP database
        possible_paths = [
            Path(__file__).parent.parent.parent / "geoip" / "GeoLite2-City.mmdb",
            Path("/var/lib/GeoIP/GeoLite2-City.mmdb"),
            Path("./geoip/GeoLite2-City.mmdb"),
        ]
        
        for db_path in possible_paths:
            if db_path.exists():
                try:
                    self.geoip_reader = geoip2.database.Reader(str(db_path))
                    logger.info(f"GeoIP database loaded from {db_path}")
                    return
                except Exception as e:
                    logger.error(f"Error loading GeoIP database from {db_path}: {e}")
        
        logger.warning("GeoIP database not found - geolocation will not be available")
    
    def get_geoip_info(self, ip_address: str) -> Optional[Dict[str, Any]]:
        """Get geolocation information for an IP address"""
        if not self.geoip_reader:
            return None
        
        try:
            response = self.geoip_reader.city(ip_address)
            
            return {
                "latitude": response.location.latitude,
                "longitude": response.location.longitude,
                "city": response.city.name,
                "country": response.country.name,
                "country_code": response.country.iso_code,
            }
        except geoip2.errors.AddressNotFoundError:
            logger.debug(f"IP {ip_address} not found in GeoIP database")
            return None
        except Exception as e:
            logger.error(f"Error looking up IP {ip_address}: {e}")
            return None
    
    async def resolve_hostname(self, ip_address: str) -> Optional[str]:
        """Resolve IP address to hostname"""
        try:
            loop = asyncio.get_event_loop()
            hostname = await loop.run_in_executor(
                None,
                lambda: socket.gethostbyaddr(ip_address)[0]
            )
            return hostname
        except (socket.herror, socket.gaierror, OSError):
            return None
    
    async def _run_windows_tracert(self, destination: str, max_hops: int) -> List[Dict[str, Any]]:
        """Run Windows tracert command and parse output"""
        try:
            # Run tracert command using executor (asyncio.create_subprocess_exec not always reliable on Windows)
            loop = asyncio.get_event_loop()
            result = await loop.run_in_executor(
                None,
                lambda: subprocess.run(
                    ['tracert', '-h', str(max_hops), '-w', '2000', '-d', destination],
                    capture_output=True,
                    text=True,
                    encoding='cp850',
                    errors='ignore'
                )
            )
            
            output = result.stdout
            
            logger.info(f"Tracert output:\n{output[:500]}")  # Log first 500 chars for debugging
            
            hops = []
            
            # Parse tracert output
            for line in output.split('\n'):
                line = line.strip()
                if not line:
                    continue
                
                # Match hop lines (start with number followed by whitespace and time/asterisk)
                # Examples:
                #   "  1    <1 ms    <1 ms    <1 ms  10.10.80.1"
                #   "  2     *        *        *     Zeitüberschreitung der Anforderung."
                match = re.match(r'^\s*(\d+)\s+(.+)$', line)
                if not match:
                    continue
                
                hop_num = int(match.group(1))
                hop_data = match.group(2)
                
                # Check for timeout (English: "Request timed out", German: "Zeitüberschreitung")
                if 'Request timed out' in hop_data or 'Zeitüberschreitung' in hop_data or '*' in hop_data[:20]:
                    hops.append({
                        'hop_number': hop_num,
                        'responded': False,
                        'ip_address': None,
                        'hostname': None,
                        'rtt_ms': None
                    })
                    logger.debug(f"Hop {hop_num}: timeout/no response")
                    continue
                
                # Extract IP address (can be standalone or in brackets)
                ip_match = re.search(r'((?:\d{1,3}\.){3}\d{1,3})', hop_data)
                if not ip_match:
                    logger.debug(f"Hop {hop_num}: no IP found in '{hop_data}'")
                    continue
                
                ip_address = ip_match.group(1)
                
                # Extract RTT values (take the first actual number, skip <1)
                rtt_ms = None
                rtt_matches = re.findall(r'(\d+)\s*ms', hop_data)
                if rtt_matches:
                    try:
                        # Find first non-zero value or use first value
                        for rtt in rtt_matches:
                            rtt_val = float(rtt)
                            if rtt_val > 0:
                                rtt_ms = rtt_val
                                break
                        if rtt_ms is None and rtt_matches:
                            rtt_ms = float(rtt_matches[0])
                    except ValueError:
                        pass
                
                # Extract hostname (text before IP, but after RTT values)
                # Example: "  1    <1 ms    <1 ms    <1 ms  unifi.localdomain [10.10.80.1]"
                hostname = None
                # Look for text between last RTT and IP
                hostname_match = re.search(r'ms\s+([^\d\[]+?)(?:\[|(?:\d{1,3}\.))', hop_data)
                if hostname_match:
                    hostname = hostname_match.group(1).strip()
                    if hostname:
                        # Clean up hostname
                        hostname = re.sub(r'\s+', ' ', hostname).strip()
                
                logger.debug(f"Hop {hop_num}: {ip_address} ({hostname}) - {rtt_ms}ms")
                
                hops.append({
                    'hop_number': hop_num,
                    'responded': True,
                    'ip_address': ip_address,
                    'hostname': hostname,
                    'rtt_ms': rtt_ms,
                    'packet_loss': 0.0  # Windows tracert doesn't provide packet loss per hop
                })
            
            logger.info(f"Parsed {len(hops)} hops from tracert output")
            return hops
            
        except Exception as e:
            logger.error(f"Error running Windows tracert: {e}", exc_info=True)
            return []
    
    async def _run_linux_traceroute(self, destination: str, max_hops: int) -> List[Dict[str, Any]]:
        """Run Linux traceroute command and parse output"""
        try:
            # Resolve destination to know when to stop
            try:
                dest_ip = socket.gethostbyname(destination)
            except socket.gaierror:
                dest_ip = None
            
            # Run traceroute command using asyncio subprocess
            # -n: no DNS resolution (faster, we'll resolve later if needed)
            # -m: max hops
            # -w: wait time (seconds)
            process = await asyncio.create_subprocess_exec(
                'traceroute', '-n', '-m', str(max_hops), '-w', '2', destination,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE
            )
            
            stdout, stderr = await process.communicate()
            output = stdout.decode('utf-8', errors='ignore')
            
            if process.returncode != 0:
                logger.error(f"Traceroute failed: {stderr.decode('utf-8', errors='ignore')}")
                return []
            
            logger.info(f"Traceroute output:\n{output[:500]}")
            
            hops = []
            reached_destination = False
            consecutive_timeouts = 0
            
            # Parse traceroute output
            # Format: " 1  192.168.1.1  1.234 ms  1.123 ms  1.456 ms"
            for line in output.split('\n'):
                line = line.strip()
                if not line:
                    continue
                
                # Match hop lines
                match = re.match(r'^\s*(\d+)\s+(.+)$', line)
                if not match:
                    continue
                
                hop_num = int(match.group(1))
                hop_data = match.group(2)
                
                # Check for timeout (indicated by *)
                if hop_data.strip().startswith('*') or '* * *' in hop_data:
                    consecutive_timeouts += 1
                    
                    # If we've reached destination and have 3+ consecutive timeouts, stop parsing
                    if reached_destination and consecutive_timeouts >= 3:
                        logger.info(f"Stopping at hop {hop_num}: reached destination and {consecutive_timeouts} consecutive timeouts")
                        break
                    
                    hops.append({
                        'hop_number': hop_num,
                        'responded': False,
                        'ip_address': None,
                        'hostname': None,
                        'rtt_ms': None
                    })
                    logger.debug(f"Hop {hop_num}: timeout/no response")
                    continue
                
                consecutive_timeouts = 0
                # Extract IP and RTT
                # Format: "192.168.1.1  1.234 ms  1.123 ms  1.456 ms"
                parts = hop_data.split()
                if len(parts) < 2:
                    continue
                
                ip_address = parts[0]
                
                # Check if we reached the destination
                if dest_ip and ip_address == dest_ip:
                    reached_destination = True
                    logger.info(f"Reached destination at hop {hop_num}: {ip_address}")
                
                # Extract RTT values and calculate average
                rtt_values = []
                for i, part in enumerate(parts[1:]):
                    if part == 'ms' and i > 0:
                        try:
                            rtt_values.append(float(parts[i]))
                        except ValueError:
                            pass
                
                rtt_ms = sum(rtt_values) / len(rtt_values) if rtt_values else None
                
                logger.debug(f"Hop {hop_num}: {ip_address} - {rtt_ms}ms")
                
                hops.append({
                    'hop_number': hop_num,
                    'responded': True,
                    'ip_address': ip_address,
                    'hostname': None,  # Will be resolved later
                    'rtt_ms': rtt_ms,
                    'packet_loss': 0.0
                })
            
            logger.info(f"Parsed {len(hops)} hops from traceroute output (reached_destination={reached_destination})")
            return hops
            
        except FileNotFoundError:
            logger.error("traceroute command not found. Please install traceroute package.")
            return []
        except Exception as e:
            logger.error(f"Error running Linux traceroute: {e}", exc_info=True)
            return []
    
    def _interpolate_missing_geoip(self, hops_data: List[Dict[str, Any]]) -> None:
        """
        Interpolate missing GeoIP data for hops in the same subnet.
        Marks interpolated hops with a special flag.
        """
        for i, hop in enumerate(hops_data):
            # Skip if hop already has coordinates or didn't respond
            if hop['latitude'] is not None or not hop['responded'] or not hop['ip_address']:
                continue
            
            # Find nearest hop with GeoIP data in same /24 subnet
            hop_ip_parts = hop['ip_address'].split('.')
            if len(hop_ip_parts) != 4:
                continue
            
            hop_subnet = '.'.join(hop_ip_parts[:3])  # /24 subnet
            
            # Look for nearby hops with GeoIP data in same subnet
            donor_hop = None
            
            # First try: look backwards for a hop in same subnet
            for j in range(i - 1, max(0, i - 5), -1):
                if (hops_data[j]['latitude'] is not None and 
                    hops_data[j]['ip_address'] and 
                    hops_data[j]['ip_address'].startswith(hop_subnet + '.')):
                    donor_hop = hops_data[j]
                    break
            
            # Second try: look forwards
            if not donor_hop:
                for j in range(i + 1, min(len(hops_data), i + 5)):
                    if (hops_data[j]['latitude'] is not None and 
                        hops_data[j]['ip_address'] and 
                        hops_data[j]['ip_address'].startswith(hop_subnet + '.')):
                        donor_hop = hops_data[j]
                        break
            
            # Third try: just use nearest hop with GeoIP (different subnet)
            if not donor_hop:
                for j in range(max(0, i - 3), min(len(hops_data), i + 4)):
                    if j != i and hops_data[j]['latitude'] is not None:
                        donor_hop = hops_data[j]
                        break
            
            if donor_hop:
                # Add small random offset to distinguish from original
                # This creates a "nearby" location on the map
                import random
                lat_offset = random.uniform(-0.05, 0.05)  # ~5km offset
                lon_offset = random.uniform(-0.05, 0.05)
                
                hop['latitude'] = donor_hop['latitude'] + lat_offset
                hop['longitude'] = donor_hop['longitude'] + lon_offset
                hop['city'] = f"? {donor_hop['city']}" if donor_hop['city'] else "? Unknown"
                hop['country'] = donor_hop['country']
                hop['country_code'] = donor_hop['country_code']
                hop['geoip_interpolated'] = True  # Mark as interpolated
                
                logger.debug(f"Interpolated GeoIP for hop {hop['hop_number']} from hop with IP {donor_hop.get('ip_address')}")
    
    async def run_traceroute(
        self,
        destination: str,
        max_hops: int = 30,
        timeout: int = 2,
        count: int = 3,
        max_total_timeout: int = 90
    ) -> Dict[str, Any]:
        """Run traceroute to destination (cross-platform)"""
        try:
            # Detect which command is available
            has_traceroute = shutil.which('traceroute') is not None
            has_tracert = shutil.which('tracert') is not None
            
            logger.info(f"Command availability check: traceroute={has_traceroute}, tracert={has_tracert}")
            
            if has_traceroute:
                use_windows = False
                logger.info(f"Starting traceroute to {destination} using traceroute command")
            elif has_tracert:
                use_windows = True
                logger.info(f"Starting traceroute to {destination} using tracert command")
            else:
                logger.error("Neither traceroute nor tracert command found on system")
                return {
                    "completed": False,
                    "error_message": "Neither traceroute nor tracert command found on system",
                    "destination_ip": None,
                    "destination_host": destination,
                    "hops": [],
                    "total_hops": 0,
                    "total_rtt_ms": 0,
                    "started_at": datetime.utcnow(),
                    "completed_at": datetime.utcnow()
                }
            
            started_at = datetime.utcnow()
            
            # Resolve destination to IP
            try:
                dest_ip = socket.gethostbyname(destination)
            except socket.gaierror as e:
                return {
                    "completed": False,
                    "error_message": f"Failed to resolve hostname: {e}",
                    "destination_ip": None,
                    "destination_host": destination,
                    "hops": [],
                    "total_hops": 0,
                    "total_rtt_ms": 0,
                    "started_at": started_at,
                    "completed_at": datetime.utcnow()
                }
            
            # Get source IP
            source_ip = None
            try:
                with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
                    s.connect((dest_ip, 80))
                    source_ip = s.getsockname()[0]
            except Exception as e:
                logger.warning(f"Could not determine source IP: {e}")
            
            # Run platform-specific traceroute with timeout
            try:
                if use_windows:
                    raw_hops = await asyncio.wait_for(
                        self._run_windows_tracert(destination, max_hops),
                        timeout=max_total_timeout
                    )
                else:  # Linux, Darwin (macOS), etc.
                    raw_hops = await asyncio.wait_for(
                        self._run_linux_traceroute(destination, max_hops),
                        timeout=max_total_timeout
                    )
            except asyncio.TimeoutError:
                logger.warning(f"Traceroute to {destination} exceeded timeout")
                return {
                    "completed": False,
                    "error_message": f"Traceroute exceeded maximum timeout of {max_total_timeout} seconds",
                    "destination_ip": dest_ip,
                    "destination_host": destination,
                    "source_ip": source_ip,
                    "hops": [],
                    "total_hops": 0,
                    "total_rtt_ms": 0,
                    "started_at": started_at,
                    "completed_at": datetime.utcnow()
                }
            
            # Process results
            hops_data = []
            total_rtt = 0
            
            for i, hop in enumerate(raw_hops):
                hop_info = {
                    "hop_number": hop['hop_number'],
                    "ip_address": hop['ip_address'],
                    "hostname": hop['hostname'],
                    "responded": hop['responded'],
                    "rtt_ms": hop['rtt_ms'],
                    "packet_loss": hop.get('packet_loss', 0.0),
                    "latitude": None,
                    "longitude": None,
                    "city": None,
                    "country": None,
                    "country_code": None,
                    "asn": None,
                    "asn_organization": None
                }
                
                # Get geolocation for responding hops
                if hop['responded'] and hop['ip_address']:
                    # Resolve hostname if not provided
                    if not hop['hostname']:
                        hop_info['hostname'] = await self.resolve_hostname(hop['ip_address'])
                    
                    # Get GeoIP data
                    geoip = self.get_geoip_info(hop['ip_address'])
                    if geoip:
                        hop_info.update({
                            "latitude": geoip['latitude'],
                            "longitude": geoip['longitude'],
                            "city": geoip['city'],
                            "country": geoip['country'],
                            "country_code": geoip['country_code']
                        })
                    
                    if hop['rtt_ms']:
                        total_rtt += hop['rtt_ms']
                    
                    logger.info(f"Hop {hop['hop_number']}: {hop['ip_address']} ({hop_info['hostname']}) - {hop['rtt_ms']}ms")
                
                hops_data.append(hop_info)
            
            # Fill in missing GeoIP data by interpolating from nearby hops in same subnet
            self._interpolate_missing_geoip(hops_data)
            
            completed_at = datetime.utcnow()
            
            return {
                "completed": True,
                "error_message": None,
                "destination_ip": dest_ip,
                "destination_host": destination,
                "source_ip": source_ip,
                "hops": hops_data,
                "total_hops": len(hops_data),
                "total_rtt_ms": total_rtt,
                "started_at": started_at,
                "completed_at": completed_at
            }
            
        except Exception as e:
            logger.error(f"Traceroute error: {e}")
            return {
                "completed": False,
                "error_message": str(e),
                "destination_ip": None,
                "destination_host": destination,
                "source_ip": None,
                "hops": [],
                "total_hops": 0,
                "total_rtt_ms": 0,
                "started_at": started_at if 'started_at' in locals() else datetime.utcnow(),
                "completed_at": datetime.utcnow()
            }
