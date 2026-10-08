// Package model enthält die Datenstrukturen der Anwendung. Die JSON-Tags
// entsprechen 1:1 den Feldnamen der bisherigen Python-API (snake_case);
// optionale Felder sind Pointer und werden als null serialisiert.
package model

import "time"

// User ist ein Benutzerkonto.
type User struct {
	ID             int64      `json:"id"`
	Username       string     `json:"username"`
	Email          string     `json:"email"`
	HashedPassword string     `json:"-"`
	IsActive       bool       `json:"is_active"`
	IsAdmin        bool       `json:"is_admin"`
	CreatedAt      time.Time  `json:"created_at"`
	LastLogin      *time.Time `json:"last_login"`
}

// Protocol ist das Transportprotokoll eines iperf3-Tests.
type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
)

// Valid meldet, ob p ein bekannter Wert ist.
func (p Protocol) Valid() bool { return p == ProtocolTCP || p == ProtocolUDP }

// Direction ist die Messrichtung eines iperf3-Tests.
type Direction string

const (
	DirectionDownload      Direction = "download"
	DirectionUpload        Direction = "upload"
	DirectionBidirectional Direction = "bidirectional"
)

// Valid meldet, ob d ein bekannter Wert ist.
func (d Direction) Valid() bool {
	return d == DirectionDownload || d == DirectionUpload || d == DirectionBidirectional
}

// ServerSettings sind die vom Benutzer pflegbaren Felder eines Server-Profils.
// Requests werden nur in diesen Teil dekodiert, sodass ID und Zeitstempel
// nicht von außen überschrieben werden können.
type ServerSettings struct {
	Name                    string    `json:"name"`
	Host                    string    `json:"host"`
	Port                    int       `json:"port"`
	Description             *string   `json:"description"`
	Enabled                 bool      `json:"enabled"`
	DefaultDuration         int       `json:"default_duration"`
	DefaultParallel         int       `json:"default_parallel"`
	DefaultNumStreams       int       `json:"default_num_streams"`
	DefaultProtocol         Protocol  `json:"default_protocol"`
	DefaultDirection        Direction `json:"default_direction"`
	ScheduleEnabled         bool      `json:"schedule_enabled"`
	ScheduleIntervalMinutes int       `json:"schedule_interval_minutes"`
	AutoTraceEnabled        bool      `json:"auto_trace_enabled"`
}

// DefaultServerSettings liefert die Vorgabewerte für ein neues Server-Profil.
func DefaultServerSettings() ServerSettings {
	return ServerSettings{
		Port:                    5201,
		Enabled:                 true,
		DefaultDuration:         10,
		DefaultParallel:         1,
		DefaultNumStreams:       1,
		DefaultProtocol:         ProtocolTCP,
		DefaultDirection:        DirectionDownload,
		ScheduleIntervalMinutes: 30,
	}
}

// Validate prüft die Wertebereiche wie das bisherige Pydantic-Schema und
// liefert eine Fehlermeldung oder "".
func (s ServerSettings) Validate() string {
	inRange := func(v, lo, hi int) bool { return v >= lo && v <= hi }
	switch {
	case !inRange(len([]rune(s.Name)), 1, 100):
		return "Name muss 1–100 Zeichen lang sein"
	case !inRange(len([]rune(s.Host)), 1, 255):
		return "Host muss 1–255 Zeichen lang sein"
	case !inRange(s.Port, 1, 65535):
		return "Port muss zwischen 1 und 65535 liegen"
	case !inRange(s.DefaultDuration, 1, 300):
		return "Testdauer muss zwischen 1 und 300 Sekunden liegen"
	case !inRange(s.DefaultParallel, 1, 128):
		return "Parallele Streams müssen zwischen 1 und 128 liegen"
	case !inRange(s.DefaultNumStreams, 1, 128):
		return "Anzahl Streams muss zwischen 1 und 128 liegen"
	case !s.DefaultProtocol.Valid():
		return "Protokoll muss tcp oder udp sein"
	case !s.DefaultDirection.Valid():
		return "Richtung muss download, upload oder bidirectional sein"
	case s.ScheduleIntervalMinutes < 1:
		return "Intervall muss mindestens 1 Minute betragen"
	}
	return ""
}

// TestStatus ist der Ausführungsstatus eines Tests.
type TestStatus string

const (
	StatusPending   TestStatus = "pending"
	StatusRunning   TestStatus = "running"
	StatusCompleted TestStatus = "completed"
	StatusFailed    TestStatus = "failed"
)

// Valid meldet, ob s ein bekannter Wert ist.
func (s TestStatus) Valid() bool {
	return s == StatusPending || s == StatusRunning || s == StatusCompleted || s == StatusFailed
}

// TestResult sind die Messwerte eines abgeschlossenen Tests.
type TestResult struct {
	DownloadBandwidthMbps     *float64 `json:"download_bandwidth_mbps"`
	DownloadBytes             *int64   `json:"download_bytes"`
	DownloadJitterMs          *float64 `json:"download_jitter_ms"`
	DownloadPacketLossPercent *float64 `json:"download_packet_loss_percent"`
	UploadBandwidthMbps       *float64 `json:"upload_bandwidth_mbps"`
	UploadBytes               *int64   `json:"upload_bytes"`
	UploadJitterMs            *float64 `json:"upload_jitter_ms"`
	UploadPacketLossPercent   *float64 `json:"upload_packet_loss_percent"`
	Retransmits               *int64   `json:"retransmits"`
	CPUPercent                *float64 `json:"cpu_percent"`
}

// Test ist ein iperf3-Testlauf. Die Rohausgabe wird nur in der Detailansicht
// geliefert (siehe TestDetail).
type Test struct {
	ID              int64      `json:"id"`
	ServerID        int64      `json:"server_id"`
	Protocol        Protocol   `json:"protocol"`
	Direction       Direction  `json:"direction"`
	Duration        int        `json:"duration"`
	ParallelStreams int        `json:"parallel_streams"`
	Status          TestStatus `json:"status"`
	StartedAt       *time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	TestResult
	ErrorMessage *string   `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

// TestDetail ist ein Test inklusive Server-Profil und Rohausgabe.
type TestDetail struct {
	Test
	Server    Server  `json:"server"`
	RawOutput *string `json:"raw_output"`
}

// Server ist ein iperf3-Server-Profil.
type Server struct {
	ID int64 `json:"id"`
	ServerSettings
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
