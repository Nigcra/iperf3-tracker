package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

// DashboardStats liefert die Kennzahlen der Übersicht. todayStart ist der
// Beginn des aktuellen Tages; Tests ab diesem Zeitpunkt zählen als „heute“.
func (s *Store) DashboardStats(ctx context.Context, todayStart time.Time) (*model.DashboardStats, error) {
	var st model.DashboardStats
	err := s.db.QueryRowContext(noCancel(ctx), `
		SELECT
			(SELECT COUNT(*) FROM servers),
			(SELECT COUNT(*) FROM servers WHERE enabled = 1),
			(SELECT COUNT(*) FROM tests),
			(SELECT COUNT(*) FROM tests WHERE created_at >= ?),
			(SELECT AVG(download_bandwidth_mbps) FROM tests WHERE status = ?),
			(SELECT AVG(upload_bandwidth_mbps) FROM tests WHERE status = ?),
			(SELECT MAX(created_at) FROM tests)`,
		db.FormatTime(todayStart), model.StatusCompleted, model.StatusCompleted,
	).Scan(&st.TotalServers, &st.ActiveServers, &st.TotalTests, &st.TestsToday,
		&st.AvgDownloadMbps, &st.AvgUploadMbps, db.NullTime(&st.LastTestAt))
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// serverStatsQuery aggregiert alle Kennzahlen je Server in einer Abfrage.
// Jitter und Paketverlust werden über beide Richtungen gemittelt.
const serverStatsQuery = `
	WITH ok AS (
		SELECT server_id, download_bandwidth_mbps, upload_bandwidth_mbps,
		       download_jitter_ms, upload_jitter_ms,
		       download_packet_loss_percent, upload_packet_loss_percent
		FROM tests WHERE status = 'completed'
	),
	agg AS (
		SELECT server_id,
		       AVG(download_bandwidth_mbps) AS avg_down,
		       AVG(upload_bandwidth_mbps)   AS avg_up,
		       CASE WHEN COUNT(download_jitter_ms) + COUNT(upload_jitter_ms) > 0
		            THEN (TOTAL(download_jitter_ms) + TOTAL(upload_jitter_ms))
		                 / (COUNT(download_jitter_ms) + COUNT(upload_jitter_ms)) END AS avg_jitter,
		       CASE WHEN COUNT(download_packet_loss_percent) + COUNT(upload_packet_loss_percent) > 0
		            THEN (TOTAL(download_packet_loss_percent) + TOTAL(upload_packet_loss_percent))
		                 / (COUNT(download_packet_loss_percent) + COUNT(upload_packet_loss_percent)) END AS avg_loss
		FROM ok GROUP BY server_id
	)
	SELECT s.id, s.name,
	       COUNT(t.id),
	       COUNT(CASE WHEN t.status = 'completed' THEN 1 END),
	       COUNT(CASE WHEN t.status = 'failed' THEN 1 END),
	       agg.avg_down, agg.avg_up, agg.avg_jitter, agg.avg_loss,
	       MAX(t.created_at)
	FROM servers s
	LEFT JOIN tests t ON t.server_id = s.id
	LEFT JOIN agg ON agg.server_id = s.id`

// ServerStats liefert die Kennzahlen aller Server, sortiert nach ID.
func (s *Store) ServerStats(ctx context.Context) ([]model.ServerStats, error) {
	rows, err := s.db.QueryContext(noCancel(ctx), serverStatsQuery+` GROUP BY s.id ORDER BY s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []model.ServerStats{}
	for rows.Next() {
		st, err := scanServerStats(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *st)
	}
	return list, rows.Err()
}

// ServerStatsByID liefert die Kennzahlen eines Servers.
func (s *Store) ServerStatsByID(ctx context.Context, id int64) (*model.ServerStats, error) {
	return scanServerStats(s.db.QueryRowContext(noCancel(ctx), serverStatsQuery+` WHERE s.id = ? GROUP BY s.id`, id))
}

func scanServerStats(row rowScanner) (*model.ServerStats, error) {
	var st model.ServerStats
	err := row.Scan(&st.ServerID, &st.ServerName, &st.TotalTests, &st.SuccessfulTests, &st.FailedTests,
		&st.AvgDownloadMbps, &st.AvgUploadMbps, &st.AvgJitterMs, &st.AvgPacketLossPercent,
		db.NullTime(&st.LastTestAt))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}
