-- Add traces and trace_hops tables for network path tracing

CREATE TABLE IF NOT EXISTS traces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    test_id INTEGER NOT NULL UNIQUE,
    source_ip VARCHAR(45),
    destination_ip VARCHAR(45) NOT NULL,
    destination_host VARCHAR(255) NOT NULL,
    total_hops INTEGER NOT NULL DEFAULT 0,
    total_rtt_ms REAL,
    completed BOOLEAN NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (test_id) REFERENCES tests(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS trace_hops (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trace_id INTEGER NOT NULL,
    hop_number INTEGER NOT NULL,
    ip_address VARCHAR(45),
    hostname VARCHAR(255),
    latitude REAL,
    longitude REAL,
    city VARCHAR(100),
    country VARCHAR(100),
    country_code VARCHAR(2),
    asn INTEGER,
    asn_organization VARCHAR(255),
    rtt_ms REAL,
    packet_loss REAL,
    responded BOOLEAN NOT NULL DEFAULT 1,
    FOREIGN KEY (trace_id) REFERENCES traces(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_traces_test_id ON traces(test_id);
CREATE INDEX IF NOT EXISTS idx_traces_created_at ON traces(created_at);
CREATE INDEX IF NOT EXISTS idx_trace_hops_trace_id ON trace_hops(trace_id);
CREATE INDEX IF NOT EXISTS idx_trace_hops_hop_number ON trace_hops(hop_number);
