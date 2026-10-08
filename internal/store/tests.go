package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

// testColumns enthält alle Spalten außer raw_output, das nur die Detailansicht braucht.
const testColumns = `id, server_id, protocol, direction, duration, parallel_streams, status,
	started_at, completed_at,
	download_bandwidth_mbps, download_bytes, download_jitter_ms, download_packet_loss_percent,
	upload_bandwidth_mbps, upload_bytes, upload_jitter_ms, upload_packet_loss_percent,
	retransmits, cpu_percent, error_message, created_at`

func scanTest(row rowScanner, extra ...any) (*model.Test, error) {
	var t model.Test
	r := &t.TestResult
	dest := append([]any{&t.ID, &t.ServerID, &t.Protocol, &t.Direction, &t.Duration, &t.ParallelStreams, &t.Status,
		db.NullTime(&t.StartedAt), db.NullTime(&t.CompletedAt),
		&r.DownloadBandwidthMbps, &r.DownloadBytes, &r.DownloadJitterMs, &r.DownloadPacketLossPercent,
		&r.UploadBandwidthMbps, &r.UploadBytes, &r.UploadJitterMs, &r.UploadPacketLossPercent,
		&r.Retransmits, &r.CPUPercent, &t.ErrorMessage, db.Time(&t.CreatedAt)}, extra...)
	err := row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) queryTests(ctx context.Context, query string, args ...any) ([]model.Test, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tests := []model.Test{}
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return nil, err
		}
		tests = append(tests, *t)
	}
	return tests, rows.Err()
}

// TestFilter schränkt ListTests ein; nil-Felder filtern nicht.
type TestFilter struct {
	ServerID *int64
	Status   *model.TestStatus
	From, To *time.Time
	Skip     int
	Limit    int
}

// ListTests liefert Tests, neueste zuerst.
func (s *Store) ListTests(ctx context.Context, f TestFilter) ([]model.Test, error) {
	query := `SELECT ` + testColumns + ` FROM tests WHERE 1 = 1`
	args := []any{}
	if f.ServerID != nil {
		query += ` AND server_id = ?`
		args = append(args, *f.ServerID)
	}
	if f.Status != nil {
		query += ` AND status = ?`
		args = append(args, *f.Status)
	}
	if f.From != nil {
		query += ` AND created_at >= ?`
		args = append(args, db.FormatTime(*f.From))
	}
	if f.To != nil {
		query += ` AND created_at <= ?`
		args = append(args, db.FormatTime(*f.To))
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Skip)
	return s.queryTests(ctx, query, args...)
}

// TestByID lädt einen Test (ohne Rohausgabe).
func (s *Store) TestByID(ctx context.Context, id int64) (*model.Test, error) {
	return scanTest(s.db.QueryRowContext(ctx, `SELECT `+testColumns+` FROM tests WHERE id = ?`, id))
}

// TestDetailByID lädt einen Test inklusive Server-Profil und Rohausgabe.
func (s *Store) TestDetailByID(ctx context.Context, id int64) (*model.TestDetail, error) {
	var raw *string
	t, err := scanTest(s.db.QueryRowContext(ctx, `SELECT `+testColumns+`, raw_output FROM tests WHERE id = ?`, id), &raw)
	if err != nil {
		return nil, err
	}
	sv, err := s.ServerByID(ctx, t.ServerID)
	if err != nil {
		return nil, err
	}
	return &model.TestDetail{Test: *t, Server: *sv, RawOutput: raw}, nil
}

// LatestCompletedTest liefert den jüngsten erfolgreichen Test eines Servers.
func (s *Store) LatestCompletedTest(ctx context.Context, serverID int64) (*model.Test, error) {
	return scanTest(s.db.QueryRowContext(ctx,
		`SELECT `+testColumns+` FROM tests WHERE server_id = ? AND status = ?
		 ORDER BY created_at DESC, id DESC LIMIT 1`, serverID, model.StatusCompleted))
}

// CreateTest speichert t als wartenden Test und setzt ID und CreatedAt.
func (s *Store) CreateTest(ctx context.Context, t *model.Test) error {
	t.Status = model.StatusPending
	t.CreatedAt = db.Now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tests (server_id, protocol, direction, duration, parallel_streams, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ServerID, t.Protocol, t.Direction, t.Duration, t.ParallelStreams, t.Status, db.FormatTime(t.CreatedAt))
	if err != nil {
		return err
	}
	t.ID, err = res.LastInsertId()
	return err
}

// StartTest markiert einen Test als laufend.
func (s *Store) StartTest(ctx context.Context, id int64, startedAt time.Time) error {
	return affectedOne(s.db.ExecContext(ctx,
		`UPDATE tests SET status = ?, started_at = ? WHERE id = ?`,
		model.StatusRunning, db.FormatTime(startedAt), id))
}

// FinishTest speichert Endstatus, Messwerte, Fehlermeldung und Rohausgabe.
func (s *Store) FinishTest(ctx context.Context, id int64, status model.TestStatus, completedAt time.Time,
	r model.TestResult, errMsg, rawOutput *string) error {
	return affectedOne(s.db.ExecContext(ctx,
		`UPDATE tests SET status = ?, completed_at = ?,
			download_bandwidth_mbps = ?, download_bytes = ?, download_jitter_ms = ?, download_packet_loss_percent = ?,
			upload_bandwidth_mbps = ?, upload_bytes = ?, upload_jitter_ms = ?, upload_packet_loss_percent = ?,
			retransmits = ?, cpu_percent = ?, error_message = ?, raw_output = ?
		 WHERE id = ?`,
		status, db.FormatTime(completedAt),
		r.DownloadBandwidthMbps, r.DownloadBytes, r.DownloadJitterMs, r.DownloadPacketLossPercent,
		r.UploadBandwidthMbps, r.UploadBytes, r.UploadJitterMs, r.UploadPacketLossPercent,
		r.Retransmits, r.CPUPercent, errMsg, rawOutput, id))
}

// FailUnfinishedTests markiert alle wartenden oder laufenden Tests als
// fehlgeschlagen. Wird beim Start aufgerufen, da solche Tests aus einem
// vorherigen, abgebrochenen Lauf stammen.
func (s *Store) FailUnfinishedTests(ctx context.Context, msg string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE tests SET status = ?, completed_at = ?, error_message = ? WHERE status IN (?, ?)`,
		model.StatusFailed, db.FormatTime(db.Now()), msg, model.StatusPending, model.StatusRunning)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteTest löscht einen Test; zugehörige Traces folgen per ON DELETE CASCADE.
func (s *Store) DeleteTest(ctx context.Context, id int64) error {
	return affectedOne(s.db.ExecContext(ctx, `DELETE FROM tests WHERE id = ?`, id))
}
