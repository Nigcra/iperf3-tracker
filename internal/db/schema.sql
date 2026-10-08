-- Schema des iperf3-Trackers. Entspricht den SQLAlchemy-Modellen des
-- Python-Backends (backend/app/models/models.py) inkl. der Migrationen 001–004.
-- Enums werden als lowercase TEXT gespeichert, Zeitstempel als TEXT in UTC
-- (Format siehe time.go).

CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        VARCHAR(50)  NOT NULL UNIQUE,
    email           VARCHAR(100) NOT NULL UNIQUE,
    hashed_password VARCHAR(255) NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT 1,
    is_admin        BOOLEAN NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL,
    last_login      TEXT
);

CREATE TABLE IF NOT EXISTS servers (
    id                        INTEGER PRIMARY KEY AUTOINCREMENT,
    name                      VARCHAR(100) NOT NULL UNIQUE,
    host                      VARCHAR(255) NOT NULL,
    port                      INTEGER NOT NULL DEFAULT 5201,
    description               TEXT,
    enabled                   BOOLEAN NOT NULL DEFAULT 1,
    default_duration          INTEGER NOT NULL DEFAULT 10,
    default_parallel          INTEGER NOT NULL DEFAULT 1,
    default_num_streams       INTEGER NOT NULL DEFAULT 1,
    default_protocol          TEXT    NOT NULL DEFAULT 'tcp'
                              CHECK (default_protocol IN ('tcp', 'udp')),
    default_direction         TEXT    NOT NULL DEFAULT 'download'
                              CHECK (default_direction IN ('download', 'upload', 'bidirectional')),
    -- Zielbandbreite für UDP-Tests in Mbit/s; NULL = iperf3-Standard (1 Mbit/s).
    default_udp_bandwidth_mbps REAL,
    schedule_enabled          BOOLEAN NOT NULL DEFAULT 0,
    schedule_interval_minutes INTEGER NOT NULL DEFAULT 30,
    auto_trace_enabled        BOOLEAN NOT NULL DEFAULT 0,
    created_at                TEXT    NOT NULL,
    updated_at                TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS tests (
    id                           INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id                    INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    protocol                     TEXT    NOT NULL CHECK (protocol IN ('tcp', 'udp')),
    direction                    TEXT    NOT NULL CHECK (direction IN ('download', 'upload', 'bidirectional')),
    duration                     INTEGER NOT NULL,
    parallel_streams             INTEGER NOT NULL DEFAULT 1,
    udp_bandwidth_mbps           REAL,
    status                       TEXT    NOT NULL DEFAULT 'pending'
                                 CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    started_at                   TEXT,
    completed_at                 TEXT,
    download_bandwidth_mbps      REAL,
    download_bytes               INTEGER,
    download_jitter_ms           REAL,
    download_packet_loss_percent REAL,
    upload_bandwidth_mbps        REAL,
    upload_bytes                 INTEGER,
    upload_jitter_ms             REAL,
    upload_packet_loss_percent   REAL,
    retransmits                  INTEGER,
    cpu_percent                  REAL,
    error_message                TEXT,
    raw_output                   TEXT,
    created_at                   TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tests_server_id  ON tests(server_id);
CREATE INDEX IF NOT EXISTS idx_tests_created_at ON tests(created_at);

-- test_id ist nullable: Traces können auch ohne Test (Live-/Einzel-Trace) existieren.
CREATE TABLE IF NOT EXISTS traces (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    test_id          INTEGER REFERENCES tests(id) ON DELETE CASCADE,
    source_ip        VARCHAR(45),
    destination_ip   VARCHAR(45),
    destination_host VARCHAR(255) NOT NULL,
    total_hops       INTEGER NOT NULL DEFAULT 0,
    total_rtt_ms     REAL,
    completed        BOOLEAN NOT NULL DEFAULT 0,
    error_message    TEXT,
    started_at       TEXT,
    completed_at     TEXT,
    created_at       TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_traces_test_id    ON traces(test_id);
CREATE INDEX IF NOT EXISTS idx_traces_created_at ON traces(created_at);

CREATE TABLE IF NOT EXISTS trace_hops (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    trace_id           INTEGER NOT NULL REFERENCES traces(id) ON DELETE CASCADE,
    hop_number         INTEGER NOT NULL,
    ip_address         VARCHAR(45),
    hostname           VARCHAR(255),
    latitude           REAL,
    longitude          REAL,
    city               VARCHAR(100),
    country            VARCHAR(100),
    country_code       VARCHAR(2),
    asn                INTEGER,
    asn_organization   VARCHAR(255),
    geoip_interpolated BOOLEAN NOT NULL DEFAULT 0,
    rtt_ms             REAL,
    packet_loss        REAL,
    responded          BOOLEAN NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_trace_hops_trace_id ON trace_hops(trace_id, hop_number);
