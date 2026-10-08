package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

const traceColumns = `id, test_id, source_ip, destination_ip, destination_host, total_hops, total_rtt_ms,
	completed, error_message, started_at, completed_at, created_at`

const hopColumns = `id, trace_id, hop_number, ip_address, hostname, latitude, longitude, city, country,
	country_code, asn, asn_organization, geoip_interpolated, rtt_ms, packet_loss, responded`

func scanTrace(row rowScanner) (*model.Trace, error) {
	var t model.Trace
	err := row.Scan(&t.ID, &t.TestID, &t.SourceIP, &t.DestinationIP, &t.DestinationHost, &t.TotalHops, &t.TotalRTTMs,
		&t.Completed, &t.ErrorMessage, db.NullTime(&t.StartedAt), db.NullTime(&t.CompletedAt), db.Time(&t.CreatedAt))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Hops = []model.TraceHop{}
	return &t, nil
}

// queryTraces lädt Traces samt Hops. Die Hops werden in einer zweiten Abfrage
// geladen, nachdem die erste vollständig gelesen ist (nur eine DB-Verbindung).
func (s *Store) queryTraces(ctx context.Context, query string, args ...any) ([]model.Trace, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	traces := []model.Trace{}
	for rows.Next() {
		t, err := scanTrace(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		traces = append(traces, *t)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(traces) == 0 {
		return traces, err
	}

	index := make(map[int64]int, len(traces))
	ids := make([]any, len(traces))
	for i, t := range traces {
		index[t.ID] = i
		ids[i] = t.ID
	}
	hopRows, err := s.db.QueryContext(ctx,
		`SELECT `+hopColumns+` FROM trace_hops WHERE trace_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)
		 ORDER BY trace_id, hop_number`, ids...)
	if err != nil {
		return nil, err
	}
	defer hopRows.Close()
	for hopRows.Next() {
		var h model.TraceHop
		var traceID int64
		if err := hopRows.Scan(&h.ID, &traceID, &h.HopNumber, &h.IPAddress, &h.Hostname, &h.Latitude, &h.Longitude,
			&h.City, &h.Country, &h.CountryCode, &h.ASN, &h.ASNOrganization, &h.GeoIPInterpolated,
			&h.RTTMs, &h.PacketLoss, &h.Responded); err != nil {
			return nil, err
		}
		t := &traces[index[traceID]]
		t.Hops = append(t.Hops, h)
	}
	return traces, hopRows.Err()
}

func (s *Store) queryTrace(ctx context.Context, query string, args ...any) (*model.Trace, error) {
	traces, err := s.queryTraces(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(traces) == 0 {
		return nil, ErrNotFound
	}
	return &traces[0], nil
}

// TraceByID lädt einen Trace samt Hops.
func (s *Store) TraceByID(ctx context.Context, id int64) (*model.Trace, error) {
	return s.queryTrace(ctx, `SELECT `+traceColumns+` FROM traces WHERE id = ?`, id)
}

// TraceByTestID lädt den jüngsten Trace eines Tests.
func (s *Store) TraceByTestID(ctx context.Context, testID int64) (*model.Trace, error) {
	return s.queryTrace(ctx, `SELECT `+traceColumns+` FROM traces WHERE test_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, testID)
}

// ListTraces liefert die neuesten Traces; onlyCompleted blendet abgebrochene aus.
func (s *Store) ListTraces(ctx context.Context, limit int, onlyCompleted bool) ([]model.Trace, error) {
	where := ""
	if onlyCompleted {
		where = ` WHERE completed = 1`
	}
	return s.queryTraces(ctx, `SELECT `+traceColumns+` FROM traces`+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
}

// TracesByTest liefert alle Traces eines Tests, neueste zuerst.
func (s *Store) TracesByTest(ctx context.Context, testID int64) ([]model.Trace, error) {
	return s.queryTraces(ctx, `SELECT `+traceColumns+` FROM traces WHERE test_id = ? ORDER BY created_at DESC, id DESC`, testID)
}

// CreateTrace speichert einen Trace samt Hops in einer Transaktion und setzt die IDs.
func (s *Store) CreateTrace(ctx context.Context, t *model.Trace) error {
	t.CreatedAt = db.Now()
	t.TotalHops = len(t.Hops)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO traces (test_id, source_ip, destination_ip, destination_host, total_hops, total_rtt_ms,
			completed, error_message, started_at, completed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.TestID, t.SourceIP, t.DestinationIP, t.DestinationHost, t.TotalHops, t.TotalRTTMs,
		t.Completed, t.ErrorMessage, db.FormatNullTime(t.StartedAt), db.FormatNullTime(t.CompletedAt), db.FormatTime(t.CreatedAt))
	if err != nil {
		return err
	}
	if t.ID, err = res.LastInsertId(); err != nil {
		return err
	}

	for i := range t.Hops {
		h := &t.Hops[i]
		res, err := tx.ExecContext(ctx,
			`INSERT INTO trace_hops (trace_id, hop_number, ip_address, hostname, latitude, longitude, city, country,
				country_code, asn, asn_organization, geoip_interpolated, rtt_ms, packet_loss, responded)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, h.HopNumber, h.IPAddress, h.Hostname, h.Latitude, h.Longitude, h.City, h.Country,
			h.CountryCode, h.ASN, h.ASNOrganization, h.GeoIPInterpolated, h.RTTMs, h.PacketLoss, h.Responded)
		if err != nil {
			return err
		}
		if h.ID, err = res.LastInsertId(); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteTrace löscht einen Trace; die Hops folgen per ON DELETE CASCADE.
func (s *Store) DeleteTrace(ctx context.Context, id int64) error {
	return affectedOne(s.db.ExecContext(ctx, `DELETE FROM traces WHERE id = ?`, id))
}

// DeleteTracesByTest löscht alle Traces eines Tests.
func (s *Store) DeleteTracesByTest(ctx context.Context, testID int64) error {
	return affectedOne(s.db.ExecContext(ctx, `DELETE FROM traces WHERE test_id = ?`, testID))
}
