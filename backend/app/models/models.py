from sqlalchemy import Column, Integer, String, Boolean, Float, DateTime, ForeignKey, Text, Enum
from sqlalchemy.orm import relationship
from sqlalchemy.sql import func
from datetime import datetime
import enum
from app.core.database import Base


class ProtocolType(str, enum.Enum):
    """Protocol types for iperf3 tests"""
    TCP = "tcp"
    UDP = "udp"


class TestDirection(str, enum.Enum):
    """Test direction"""
    DOWNLOAD = "download"
    UPLOAD = "upload"
    BIDIRECTIONAL = "bidirectional"


class TestStatus(str, enum.Enum):
    """Test execution status"""
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"


class User(Base):
    """User model for authentication"""
    __tablename__ = "users"
    
    id = Column(Integer, primary_key=True, index=True)
    username = Column(String(50), unique=True, nullable=False, index=True)
    email = Column(String(100), unique=True, nullable=False, index=True)
    hashed_password = Column(String(255), nullable=False)
    is_active = Column(Boolean, default=True)
    is_admin = Column(Boolean, default=False)
    created_at = Column(DateTime, default=func.now())
    last_login = Column(DateTime, nullable=True)


class Server(Base):
    """iperf3 server profile"""
    __tablename__ = "servers"
    
    id = Column(Integer, primary_key=True, index=True)
    name = Column(String(100), nullable=False, unique=True)
    host = Column(String(255), nullable=False)
    port = Column(Integer, nullable=False, default=5201)
    description = Column(Text, nullable=True)
    enabled = Column(Boolean, default=True)
    
    # Test configuration defaults
    default_duration = Column(Integer, default=10)
    default_parallel = Column(Integer, default=1)
    default_num_streams = Column(Integer, default=1)
    default_protocol = Column(Enum(ProtocolType), default=ProtocolType.TCP)
    default_direction = Column(Enum(TestDirection), default=TestDirection.DOWNLOAD)
    
    # Scheduling
    schedule_enabled = Column(Boolean, default=False)
    schedule_interval_minutes = Column(Integer, default=30)
    
    # Auto-tracing configuration
    auto_trace_enabled = Column(Boolean, default=False)
    
    # Metadata
    created_at = Column(DateTime, default=func.now())
    updated_at = Column(DateTime, default=func.now(), onupdate=func.now())
    
    # Relationships
    tests = relationship("Test", back_populates="server", cascade="all, delete-orphan")


class Test(Base):
    """iperf3 test result"""
    __tablename__ = "tests"
    
    id = Column(Integer, primary_key=True, index=True)
    server_id = Column(Integer, ForeignKey("servers.id", ondelete="CASCADE"), nullable=False)
    
    # Test parameters
    protocol = Column(Enum(ProtocolType), nullable=False)
    direction = Column(Enum(TestDirection), nullable=False)
    duration = Column(Integer, nullable=False)
    parallel_streams = Column(Integer, nullable=False, default=1)
    
    # Test execution
    status = Column(Enum(TestStatus), default=TestStatus.PENDING)
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)
    
    # Results - Download
    download_bandwidth_mbps = Column(Float, nullable=True)  # Megabits per second
    download_bytes = Column(Integer, nullable=True)
    download_jitter_ms = Column(Float, nullable=True)  # Milliseconds
    download_packet_loss_percent = Column(Float, nullable=True)
    
    # Results - Upload
    upload_bandwidth_mbps = Column(Float, nullable=True)
    upload_bytes = Column(Integer, nullable=True)
    upload_jitter_ms = Column(Float, nullable=True)
    upload_packet_loss_percent = Column(Float, nullable=True)
    
    # Additional metrics
    retransmits = Column(Integer, nullable=True)
    cpu_percent = Column(Float, nullable=True)
    
    # Error handling
    error_message = Column(Text, nullable=True)
    
    # Raw JSON output from iperf3
    raw_output = Column(Text, nullable=True)
    
    # Metadata
    created_at = Column(DateTime, default=func.now())
    
    # Relationships
    server = relationship("Server", back_populates="tests")
    trace = relationship("Trace", back_populates="test", uselist=False, cascade="all, delete-orphan")


class Trace(Base):
    """Network trace for a test"""
    __tablename__ = "traces"
    
    id = Column(Integer, primary_key=True, index=True)
    test_id = Column(Integer, ForeignKey("tests.id", ondelete="CASCADE"), nullable=True, index=True)
    
    # Source and destination
    source_ip = Column(String(45), nullable=True)  # IPv4 or IPv6
    destination_ip = Column(String(45), nullable=True)
    destination_host = Column(String(255), nullable=False)
    
    # Trace metadata
    total_hops = Column(Integer, nullable=False, default=0)
    total_rtt_ms = Column(Float, nullable=True)
    completed = Column(Boolean, default=False)
    error_message = Column(Text, nullable=True)
    
    # Timestamps
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, default=func.now())
    
    # Relationships
    test = relationship("Test", back_populates="trace")
    hops = relationship("TraceHop", back_populates="trace", cascade="all, delete-orphan", order_by="TraceHop.hop_number")


class TraceHop(Base):
    """Individual hop in a network trace"""
    __tablename__ = "trace_hops"
    
    id = Column(Integer, primary_key=True, index=True)
    trace_id = Column(Integer, ForeignKey("traces.id", ondelete="CASCADE"), nullable=False)
    
    # Hop details
    hop_number = Column(Integer, nullable=False)
    ip_address = Column(String(45), nullable=True)  # May be None if hop doesn't respond
    hostname = Column(String(255), nullable=True)
    
    # Geolocation
    latitude = Column(Float, nullable=True)
    longitude = Column(Float, nullable=True)
    city = Column(String(100), nullable=True)
    country = Column(String(100), nullable=True)
    country_code = Column(String(2), nullable=True)
    asn = Column(Integer, nullable=True)
    asn_organization = Column(String(255), nullable=True)
    geoip_interpolated = Column(Boolean, default=False)  # True if coordinates were interpolated
    
    # Performance metrics
    rtt_ms = Column(Float, nullable=True)  # Round trip time
    packet_loss = Column(Float, nullable=True)
    
    # Metadata
    responded = Column(Boolean, default=True)
    
    # Relationships
    trace = relationship("Trace", back_populates="hops")
