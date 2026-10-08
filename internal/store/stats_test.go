package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return New(conn)
}

func f64(v float64) *float64 { return &v }

func TestStats(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	mk := func(name string, enabled bool) *model.Server {
		sv := &model.Server{ServerSettings: model.DefaultServerSettings()}
		sv.Name, sv.Host, sv.Enabled = name, name, enabled
		if err := st.CreateServer(ctx, sv); err != nil {
			t.Fatal(err)
		}
		return sv
	}
	a, b, _ := mk("a", true), mk("b", true), mk("leer", false)

	run := func(sv *model.Server, status model.TestStatus, res model.TestResult) {
		test := &model.Test{ServerID: sv.ID, Protocol: model.ProtocolTCP, Direction: model.DirectionBidirectional, Duration: 1, ParallelStreams: 1}
		if err := st.CreateTest(ctx, test); err != nil {
			t.Fatal(err)
		}
		if status != model.StatusPending {
			if err := st.FinishTest(ctx, test.ID, status, db.Now(), res, nil, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	run(a, model.StatusCompleted, model.TestResult{DownloadBandwidthMbps: f64(100), UploadBandwidthMbps: f64(10), DownloadJitterMs: f64(1), UploadJitterMs: f64(3)})
	run(a, model.StatusCompleted, model.TestResult{DownloadBandwidthMbps: f64(200), DownloadPacketLossPercent: f64(2)})
	// Fehlgeschlagene und wartende Tests fließen nicht in die Mittelwerte ein.
	run(a, model.StatusFailed, model.TestResult{DownloadBandwidthMbps: f64(9999)})
	run(a, model.StatusPending, model.TestResult{})
	run(b, model.StatusCompleted, model.TestResult{UploadBandwidthMbps: f64(50)})

	d, err := st.DashboardStats(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalServers != 3 || d.ActiveServers != 2 || d.TotalTests != 5 || d.TestsToday != 5 || d.LastTestAt == nil {
		t.Errorf("Dashboard: %+v", d)
	}
	if !eqF(d.AvgDownloadMbps, 150) || !eqF(d.AvgUploadMbps, 30) {
		t.Errorf("Dashboard-Mittelwerte: down %v up %v", *d.AvgDownloadMbps, *d.AvgUploadMbps)
	}
	if d, _ := st.DashboardStats(ctx, time.Now().Add(time.Hour)); d.TestsToday != 0 {
		t.Errorf("tests_today nach Tagesbeginn in der Zukunft: %d", d.TestsToday)
	}

	list, err := st.ServerStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("%d Server-Statistiken", len(list))
	}
	sa := list[0]
	if sa.ServerName != "a" || sa.TotalTests != 4 || sa.SuccessfulTests != 2 || sa.FailedTests != 1 {
		t.Errorf("Server a: %+v", sa)
	}
	if !eqF(sa.AvgDownloadMbps, 150) || !eqF(sa.AvgUploadMbps, 10) || !eqF(sa.AvgJitterMs, 2) || !eqF(sa.AvgPacketLossPercent, 2) {
		t.Errorf("Server a Mittelwerte: %+v", sa)
	}
	empty := list[2]
	if empty.TotalTests != 0 || empty.AvgDownloadMbps != nil || empty.AvgJitterMs != nil || empty.LastTestAt != nil {
		t.Errorf("Server ohne Tests: %+v", empty)
	}

	one, err := st.ServerStatsByID(ctx, b.ID)
	if err != nil || one.ServerName != "b" || one.AvgDownloadMbps != nil || !eqF(one.AvgUploadMbps, 50) {
		t.Errorf("ServerStatsByID: %+v, %v", one, err)
	}
	if _, err := st.ServerStatsByID(ctx, 999); err != ErrNotFound {
		t.Errorf("unbekannter Server: %v", err)
	}
}

func eqF(got *float64, want float64) bool { return got != nil && *got == want }
