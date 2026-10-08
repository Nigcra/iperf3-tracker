package store

import (
	"context"
	"database/sql"
	"errors"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

const serverColumns = `id, name, host, port, description, enabled,
	default_duration, default_parallel, default_num_streams, default_protocol, default_direction,
	default_udp_bandwidth_mbps, schedule_enabled, schedule_interval_minutes, auto_trace_enabled, created_at, updated_at`

func scanServer(row rowScanner) (*model.Server, error) {
	var sv model.Server
	err := row.Scan(&sv.ID, &sv.Name, &sv.Host, &sv.Port, &sv.Description, &sv.Enabled,
		&sv.DefaultDuration, &sv.DefaultParallel, &sv.DefaultNumStreams, &sv.DefaultProtocol, &sv.DefaultDirection,
		&sv.DefaultUDPBandwidthMbps, &sv.ScheduleEnabled, &sv.ScheduleIntervalMinutes, &sv.AutoTraceEnabled,
		db.Time(&sv.CreatedAt), db.Time(&sv.UpdatedAt))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sv, nil
}

// ServerByID lädt ein Server-Profil anhand seiner ID.
func (s *Store) ServerByID(ctx context.Context, id int64) (*model.Server, error) {
	return scanServer(s.db.QueryRowContext(ctx, `SELECT `+serverColumns+` FROM servers WHERE id = ?`, id))
}

// ListServers liefert Server-Profile, neueste zuerst. enabled filtert optional.
func (s *Store) ListServers(ctx context.Context, enabled *bool, skip, limit int) ([]model.Server, error) {
	query := `SELECT ` + serverColumns + ` FROM servers`
	args := []any{}
	if enabled != nil {
		query += ` WHERE enabled = ?`
		args = append(args, *enabled)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, skip)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := []model.Server{}
	for rows.Next() {
		sv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, *sv)
	}
	return servers, rows.Err()
}

// CreateServer speichert sv und setzt ID und Zeitstempel. Ein bereits
// vergebener Name ergibt ErrDuplicate.
func (s *Store) CreateServer(ctx context.Context, sv *model.Server) error {
	sv.CreatedAt = db.Now()
	sv.UpdatedAt = sv.CreatedAt
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO servers (name, host, port, description, enabled,
			default_duration, default_parallel, default_num_streams, default_protocol, default_direction,
			default_udp_bandwidth_mbps, schedule_enabled, schedule_interval_minutes, auto_trace_enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sv.Name, sv.Host, sv.Port, sv.Description, sv.Enabled,
		sv.DefaultDuration, sv.DefaultParallel, sv.DefaultNumStreams, sv.DefaultProtocol, sv.DefaultDirection,
		sv.DefaultUDPBandwidthMbps, sv.ScheduleEnabled, sv.ScheduleIntervalMinutes, sv.AutoTraceEnabled,
		db.FormatTime(sv.CreatedAt), db.FormatTime(sv.UpdatedAt))
	if err != nil {
		return mapUnique(err)
	}
	sv.ID, err = res.LastInsertId()
	return err
}

// UpdateServer schreibt alle pflegbaren Felder von sv und setzt UpdatedAt.
func (s *Store) UpdateServer(ctx context.Context, sv *model.Server) error {
	sv.UpdatedAt = db.Now()
	err := affectedOne(s.db.ExecContext(ctx,
		`UPDATE servers SET name = ?, host = ?, port = ?, description = ?, enabled = ?,
			default_duration = ?, default_parallel = ?, default_num_streams = ?,
			default_protocol = ?, default_direction = ?, default_udp_bandwidth_mbps = ?,
			schedule_enabled = ?, schedule_interval_minutes = ?, auto_trace_enabled = ?, updated_at = ?
		 WHERE id = ?`,
		sv.Name, sv.Host, sv.Port, sv.Description, sv.Enabled,
		sv.DefaultDuration, sv.DefaultParallel, sv.DefaultNumStreams,
		sv.DefaultProtocol, sv.DefaultDirection, sv.DefaultUDPBandwidthMbps,
		sv.ScheduleEnabled, sv.ScheduleIntervalMinutes, sv.AutoTraceEnabled, db.FormatTime(sv.UpdatedAt),
		sv.ID))
	return mapUnique(err)
}

// DeleteServer löscht ein Server-Profil; Tests und Traces folgen per ON DELETE CASCADE.
func (s *Store) DeleteServer(ctx context.Context, id int64) error {
	return affectedOne(s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id))
}
