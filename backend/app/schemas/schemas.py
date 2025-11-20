from pydantic import BaseModel, Field, field_validator
from datetime import datetime
from typing import Optional
from app.models.models import ProtocolType, TestDirection, TestStatus


# User Schemas
class UserBase(BaseModel):
    """Base user schema"""
    username: str = Field(..., min_length=3, max_length=50)
    email: str = Field(..., min_length=3, max_length=100)


class UserCreate(UserBase):
    """Schema for creating a user"""
    password: str = Field(..., min_length=6)
    is_admin: bool = False


class UserResponse(UserBase):
    """Schema for user response"""
    id: int
    is_active: bool
    is_admin: bool
    created_at: datetime
    last_login: Optional[datetime]
    
    class Config:
        from_attributes = True


class LoginRequest(BaseModel):
    """Schema for login request"""
    username: str
    password: str


class TokenResponse(BaseModel):
    """Schema for token response"""
    access_token: str
    token_type: str = "bearer"
    user: UserResponse


# Server Schemas
class ServerBase(BaseModel):
    """Base server schema"""
    name: str = Field(..., min_length=1, max_length=100)
    host: str = Field(..., min_length=1, max_length=255)
    port: int = Field(default=5201, ge=1, le=65535)
    description: Optional[str] = None
    enabled: bool = True
    
    # Test defaults
    default_duration: int = Field(default=10, ge=1, le=300)
    default_parallel: int = Field(default=1, ge=1, le=128)
    default_num_streams: int = Field(default=1, ge=1, le=128)
    default_protocol: ProtocolType = ProtocolType.TCP
    default_direction: TestDirection = TestDirection.DOWNLOAD
    
    # Scheduling
    schedule_enabled: bool = False
    schedule_interval_minutes: int = Field(default=30, ge=1)
    
    # Auto-tracing
    auto_trace_enabled: bool = False


class ServerCreate(ServerBase):
    """Schema for creating a server"""
    pass


class ServerUpdate(BaseModel):
    """Schema for updating a server"""
    name: Optional[str] = Field(None, min_length=1, max_length=100)
    host: Optional[str] = Field(None, min_length=1, max_length=255)
    port: Optional[int] = Field(None, ge=1, le=65535)
    description: Optional[str] = None
    enabled: Optional[bool] = None
    default_duration: Optional[int] = Field(None, ge=1, le=300)
    default_parallel: Optional[int] = Field(None, ge=1, le=128)
    default_num_streams: Optional[int] = Field(None, ge=1, le=128)
    default_protocol: Optional[ProtocolType] = None
    default_direction: Optional[TestDirection] = None
    schedule_enabled: Optional[bool] = None
    schedule_interval_minutes: Optional[int] = Field(None, ge=1)
    auto_trace_enabled: Optional[bool] = None


class ServerResponse(ServerBase):
    """Schema for server response"""
    id: int
    created_at: datetime
    updated_at: datetime
    
    class Config:
        from_attributes = True


# Test Schemas
class TestBase(BaseModel):
    """Base test schema"""
    protocol: ProtocolType = ProtocolType.TCP
    direction: TestDirection = TestDirection.DOWNLOAD
    duration: int = Field(default=10, ge=1, le=300)
    parallel_streams: int = Field(default=1, ge=1, le=128)


class TestCreate(TestBase):
    """Schema for creating/running a test"""
    server_id: int


class TestResponse(TestBase):
    """Schema for test response"""
    id: int
    server_id: int
    status: TestStatus
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    
    # Results
    download_bandwidth_mbps: Optional[float]
    download_bytes: Optional[int]
    download_jitter_ms: Optional[float]
    download_packet_loss_percent: Optional[float]
    
    upload_bandwidth_mbps: Optional[float]
    upload_bytes: Optional[int]
    upload_jitter_ms: Optional[float]
    upload_packet_loss_percent: Optional[float]
    
    retransmits: Optional[int]
    cpu_percent: Optional[float]
    error_message: Optional[str]
    
    created_at: datetime
    
    class Config:
        from_attributes = True


class TestDetailResponse(TestResponse):
    """Detailed test response including server info"""
    server: ServerResponse
    raw_output: Optional[str]


# Statistics Schemas
class ServerStats(BaseModel):
    """Statistics for a server"""
    server_id: int
    server_name: str
    total_tests: int
    successful_tests: int
    failed_tests: int
    avg_download_mbps: Optional[float]
    avg_upload_mbps: Optional[float]
    avg_jitter_ms: Optional[float]
    avg_packet_loss_percent: Optional[float]
    last_test_at: Optional[datetime]


class DashboardStats(BaseModel):
    """Overall dashboard statistics"""
    total_servers: int
    active_servers: int
    total_tests: int
    tests_today: int
    avg_download_mbps: Optional[float]
    avg_upload_mbps: Optional[float]
    last_test_at: Optional[datetime]


# Trace Schemas
class TraceCreate(BaseModel):
    """Schema for creating a trace"""
    destination: str = Field(..., min_length=1, max_length=255)
    max_hops: int = Field(default=30, ge=1, le=64)
    timeout: int = Field(default=2, ge=1, le=10)
    count: int = Field(default=3, ge=1, le=10)


class TraceHopResponse(BaseModel):
    """Schema for a trace hop"""
    id: int
    hop_number: int
    ip_address: Optional[str]
    hostname: Optional[str]
    latitude: Optional[float]
    longitude: Optional[float]
    city: Optional[str]
    country: Optional[str]
    country_code: Optional[str]
    asn: Optional[int]
    asn_organization: Optional[str]
    rtt_ms: Optional[float]
    packet_loss: Optional[float]
    responded: bool
    
    class Config:
        from_attributes = True


class TraceResponse(BaseModel):
    """Schema for a trace"""
    id: int
    test_id: Optional[int]  # Nullable for standalone traces
    source_ip: Optional[str]
    destination_ip: Optional[str]
    destination_host: str
    total_hops: int
    total_rtt_ms: Optional[float]
    completed: bool
    error_message: Optional[str]
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    created_at: datetime
    hops: list[TraceHopResponse]
    
    class Config:
        from_attributes = True
