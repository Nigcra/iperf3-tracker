package store

import (
	"context"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

// DeleteTests löscht Tests, optional nur vor before und/oder eines Servers;
// ohne Einschränkung alle. Zugehörige Traces folgen per ON DELETE CASCADE.
func (s *Store) DeleteTests(ctx context.Context, before *time.Time, serverID *int64) (int64, error) {
	query := `DELETE FROM tests WHERE 1 = 1`
	args := []any{}
	if before != nil {
		query += ` AND created_at < ?`
		args = append(args, db.FormatTime(*before))
	}
	if serverID != nil {
		query += ` AND server_id = ?`
		args = append(args, *serverID)
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteTraces löscht Traces, optional nur vor before; ohne Einschränkung
// alle. Liefert die Anzahl gelöschter Traces und Hops.
func (s *Store) DeleteTraces(ctx context.Context, before *time.Time) (traces, hops int64, err error) {
	where, args := "", []any{}
	if before != nil {
		where, args = ` WHERE created_at < ?`, []any{db.FormatTime(*before)}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM trace_hops WHERE trace_id IN (SELECT id FROM traces`+where+`)`, args...).Scan(&hops); err != nil {
		return 0, 0, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM traces`+where, args...)
	if err != nil {
		return 0, 0, err
	}
	if traces, err = res.RowsAffected(); err != nil {
		return 0, 0, err
	}
	return traces, hops, tx.Commit()
}

// DatabaseStats liefert die Anzahl der Datensätze und die Zeitspanne der Tests.
func (s *Store) DatabaseStats(ctx context.Context) (*model.DatabaseStats, error) {
	var st model.DatabaseStats
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM tests),
			(SELECT COUNT(*) FROM servers),
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM traces),
			(SELECT COUNT(*) FROM trace_hops),
			(SELECT MIN(created_at) FROM tests),
			(SELECT MAX(created_at) FROM tests)`,
	).Scan(&st.TotalTests, &st.TotalServers, &st.TotalUsers, &st.TotalTraces, &st.TotalHops,
		db.NullTime(&st.OldestTest), db.NullTime(&st.NewestTest))
	if err != nil {
		return nil, err
	}
	return &st, nil
}
