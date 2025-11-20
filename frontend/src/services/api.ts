import axios from 'axios';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8000/api';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add token to requests
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle 401 errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// Types
export interface User {
  id: number;
  username: string;
  email: string;
  is_active: boolean;
  is_admin: boolean;
  created_at: string;
  last_login?: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface UserCreate {
  username: string;
  email: string;
  password: string;
  is_admin: boolean;
}

export interface TokenResponse {
  access_token: string;
  token_type: string;
  user: User;
}

export interface PublicServer {
  name: string;
  host: string;
  port: number;
  location: string;
  provider: string;
  description: string;
}

export enum ProtocolType {
  TCP = 'tcp',
  UDP = 'udp',
}

export enum TestDirection {
  DOWNLOAD = 'download',
  UPLOAD = 'upload',
  BIDIRECTIONAL = 'bidirectional',
}

export enum TestStatus {
  PENDING = 'pending',
  RUNNING = 'running',
  COMPLETED = 'completed',
  FAILED = 'failed',
}

export interface Server {
  id: number;
  name: string;
  host: string;
  port: number;
  description?: string;
  enabled: boolean;
  default_duration: number;
  default_parallel: number;
  default_num_streams: number;
  default_protocol: ProtocolType;
  default_direction: TestDirection;
  schedule_enabled: boolean;
  schedule_interval_minutes: number;
  auto_trace_enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface ServerCreate {
  name: string;
  host: string;
  port: number;
  description?: string;
  enabled: boolean;
  default_duration: number;
  default_parallel: number;
  default_protocol: ProtocolType;
  default_direction: TestDirection;
  schedule_enabled: boolean;
  schedule_interval_minutes: number;
  auto_trace_enabled: boolean;
}

export interface Test {
  id: number;
  server_id: number;
  protocol: ProtocolType;
  direction: TestDirection;
  duration: number;
  parallel_streams: number;
  status: TestStatus;
  started_at?: string;
  completed_at?: string;
  download_bandwidth_mbps?: number;
  download_bytes?: number;
  download_jitter_ms?: number;
  download_packet_loss_percent?: number;
  upload_bandwidth_mbps?: number;
  upload_bytes?: number;
  upload_jitter_ms?: number;
  upload_packet_loss_percent?: number;
  retransmits?: number;
  cpu_percent?: number;
  error_message?: string;
  created_at: string;
}

export interface TestCreate {
  server_id: number;
  protocol: ProtocolType;
  direction: TestDirection;
  duration: number;
  parallel_streams: number;
}

export interface DashboardStats {
  total_servers: number;
  active_servers: number;
  total_tests: number;
  tests_today: number;
  avg_download_mbps?: number;
  avg_upload_mbps?: number;
  last_test_at?: string;
}

export interface ServerStats {
  server_id: number;
  server_name: string;
  total_tests: number;
  successful_tests: number;
  failed_tests: number;
  avg_download_mbps?: number;
  avg_upload_mbps?: number;
  avg_jitter_ms?: number;
  avg_packet_loss_percent?: number;
  last_test_at?: string;
}

export interface TestLiveStatus {
  test_id: number;
  server_name: string;
  is_running: boolean;
  status: string;
  progress: number;
  elapsed_seconds: number;
  total_seconds: number;
  current_download_mbps: number;
  current_upload_mbps: number;
}

export interface TraceHop {
  id: number;
  hop_number: number;
  ip_address: string | null;
  hostname: string | null;
  latitude: number | null;
  longitude: number | null;
  city: string | null;
  country: string | null;
  country_code: string | null;
  asn: number | null;
  asn_organization: string | null;
  rtt_ms: number | null;
  packet_loss: number | null;
  responded: boolean;
}

export interface Trace {
  id: number;
  test_id: number;
  source_ip: string | null;
  destination_ip: string | null;
  destination_host: string;
  total_hops: number;
  total_rtt_ms: number | null;
  completed: boolean;
  error_message: string | null;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
  hops: TraceHop[];
}

// API Functions

// Servers
export const getServers = async (): Promise<Server[]> => {
  const response = await api.get('/servers');
  return response.data;
};

export const getServer = async (id: number): Promise<Server> => {
  const response = await api.get(`/servers/${id}`);
  return response.data;
};

export const createServer = async (server: ServerCreate): Promise<Server> => {
  const response = await api.post('/servers', server);
  return response.data;
};

export const updateServer = async (id: number, server: Partial<ServerCreate>): Promise<Server> => {
  const response = await api.put(`/servers/${id}`, server);
  return response.data;
};

export const deleteServer = async (id: number): Promise<void> => {
  await api.delete(`/servers/${id}`);
};

// Tests
export const getTests = async (params?: {
  server_id?: number;
  status?: TestStatus;
  from_date?: string;
  to_date?: string;
  skip?: number;
  limit?: number;
}): Promise<Test[]> => {
  const response = await api.get('/tests', { params });
  return response.data;
};

export const getTest = async (id: number): Promise<Test> => {
  const response = await api.get(`/tests/${id}`);
  return response.data;
};

export const runTest = async (test: TestCreate): Promise<Test> => {
  const response = await api.post('/tests/run', test);
  return response.data;
};

export const deleteTest = async (id: number): Promise<void> => {
  await api.delete(`/tests/${id}`);
};

export const getLatestTest = async (serverId: number): Promise<Test | null> => {
  const response = await api.get(`/tests/server/${serverId}/latest`);
  return response.data;
};

export const getTestLiveStatus = async (testId: number): Promise<TestLiveStatus> => {
  const response = await api.get(`/tests/${testId}/live`);
  return response.data;
};

// Statistics
export const getDashboardStats = async (): Promise<DashboardStats> => {
  const response = await api.get('/stats/dashboard');
  return response.data;
};

export const getServerStats = async (): Promise<ServerStats[]> => {
  const response = await api.get('/stats/servers');
  return response.data;
};

export const getServerStat = async (serverId: number): Promise<ServerStats> => {
  const response = await api.get(`/stats/servers/${serverId}`);
  return response.data;
};

// Authentication
export const login = async (credentials: LoginRequest): Promise<TokenResponse> => {
  const response = await api.post('/auth/login', credentials);
  return response.data;
};

export const register = async (userData: UserCreate): Promise<User> => {
  const response = await api.post('/auth/register', userData);
  return response.data;
};

export const getCurrentUser = async (): Promise<User> => {
  const response = await api.get('/auth/me');
  return response.data;
};

export const getUsers = async (): Promise<User[]> => {
  const response = await api.get('/auth/users');
  return response.data;
};

export const deleteUser = async (userId: number): Promise<void> => {
  await api.delete(`/auth/users/${userId}`);
};

export const initAdmin = async (): Promise<any> => {
  const response = await api.post('/auth/init-admin');
  return response.data;
};

// Admin / Cleanup
export const cleanupTests = async (params: {
  days?: number;
  server_id?: number;
  all?: boolean;
}): Promise<{ deleted_count: number }> => {
  const response = await api.delete('/admin/cleanup/tests', { params });
  return response.data;
};

export const cleanupTraces = async (params: {
  days?: number;
  all?: boolean;
}): Promise<{ deleted_traces: number; deleted_hops: number; message: string }> => {
  const response = await api.delete('/admin/cleanup/traces', { params });
  return response.data;
};

export const getDatabaseStats = async (): Promise<{
  total_tests: number;
  total_servers: number;
  total_users: number;
  total_traces: number;
  total_hops: number;
  oldest_test?: string;
  newest_test?: string;
}> => {
  const response = await api.get('/admin/stats/database');
  return response.data;
};

// Public Servers
export const getPublicServers = async (): Promise<PublicServer[]> => {
  const response = await api.get('/public-servers');
  return response.data;
};

export const searchPublicServers = async (query: string): Promise<PublicServer[]> => {
  const response = await api.get('/public-servers/search', { params: { query } });
  return response.data;
};

// Traces
export interface TraceCreate {
  destination: string;
  max_hops?: number;
  timeout?: number;
  count?: number;
}

export const createTrace = async (data: TraceCreate): Promise<Trace> => {
  const response = await api.post('/traces', data);
  return response.data;
};

export const getTrace = async (traceId: number): Promise<Trace> => {
  const response = await api.get(`/traces/${traceId}`);
  return response.data;
};

export const getRecentTraces = async (limit: number = 10): Promise<Trace[]> => {
  const response = await api.get('/traces', { params: { limit } });
  return response.data;
};

export const getTracesByTest = async (testId: number): Promise<Trace[]> => {
  const response = await api.get(`/traces/test/${testId}`);
  return response.data;
};

export const deleteTrace = async (traceId: number): Promise<{ message: string }> => {
  const response = await api.delete(`/traces/${traceId}`);
  return response.data;
};

export default api;
