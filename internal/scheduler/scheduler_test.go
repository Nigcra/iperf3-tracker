package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/trace"
)

// Eine "Minute" dauert im Test 40 ms. iperf3 und traceroute fehlen bewusst:
// Tests und Traces schlagen sofort fehl, werden aber gespeichert.
const testMinute = 40 * time.Millisecond

type env struct {
	store *store.Store
	sched *Scheduler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("DB schließen: %v", err)
		}
	})
	st := store.New(conn)

	runner := iperf.NewRunner(st, filepath.Join(t.TempDir(), "kein-iperf3"))
	t.Cleanup(runner.Stop)
	sched := New(st, runner, trace.NewTracer(nil, filepath.Join(t.TempDir(), "kein-traceroute")))
	sched.minute = testMinute
	t.Cleanup(sched.Stop)
	return &env{store: st, sched: sched}
}

func (e *env) server(t *testing.T, name string, mod func(*model.ServerSettings)) *model.Server {
	t.Helper()
	sv := &model.Server{ServerSettings: model.DefaultServerSettings()}
	sv.Name, sv.Host = name, "127.0.0.1"
	sv.ScheduleEnabled, sv.ScheduleIntervalMinutes = true, 1
	if mod != nil {
		mod(&sv.ServerSettings)
	}
	if err := e.store.CreateServer(context.Background(), sv); err != nil {
		t.Fatal(err)
	}
	return sv
}

func (e *env) tests(t *testing.T, sv *model.Server) []model.Test {
	t.Helper()
	list, err := e.store.ListTests(context.Background(), store.TestFilter{ServerID: &sv.ID, Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestSchedulerRunsDueServers(t *testing.T) {
	e := newEnv(t)
	plain := e.server(t, "plain", nil)
	udp := e.server(t, "udp", func(s *model.ServerSettings) {
		s.DefaultProtocol, s.DefaultDuration, s.DefaultParallel = model.ProtocolUDP, 7, 3
		s.DefaultUDPBandwidthMbps = new(float64)
		*s.DefaultUDPBandwidthMbps = 250
		s.AutoTraceEnabled = true
	})
	off := e.server(t, "ohne Zeitplan", func(s *model.ServerSettings) { s.ScheduleEnabled = false })
	disabled := e.server(t, "deaktiviert", func(s *model.ServerSettings) { s.Enabled = false })

	if err := e.sched.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !e.sched.Running() {
		t.Fatal("Scheduler läuft nicht")
	}
	// Der erste Lauf erfolgt erst nach einem Intervall.
	if n := len(e.tests(t, plain)); n != 0 {
		t.Errorf("sofortiger Lauf nach Start: %d Tests", n)
	}

	time.Sleep(6 * testMinute)
	if n := len(e.tests(t, plain)); n < 2 {
		t.Errorf("plain: %d Tests nach 6 Intervallen", n)
	}
	if n := len(e.tests(t, off)) + len(e.tests(t, disabled)); n != 0 {
		t.Errorf("nicht eingeplante Server liefen: %d Tests", n)
	}

	// Geplante Tests nutzen die Vorgaben des Servers, inkl. UDP-Bandbreite.
	ut := e.tests(t, udp)
	if len(ut) == 0 {
		t.Fatal("udp: keine Tests")
	}
	got := ut[len(ut)-1]
	if got.Protocol != model.ProtocolUDP || got.Duration != 7 || got.ParallelStreams != 3 ||
		got.UDPBandwidthMbps == nil || *got.UDPBandwidthMbps != 250 {
		t.Errorf("Testparameter: %+v", got)
	}

	// Auto-Trace: Nach dem Test wird ein Trace mit Test-Verknüpfung gespeichert.
	deadline := time.Now().Add(2 * time.Second)
	for {
		traces, err := e.store.TracesByTest(context.Background(), got.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(traces) == 1 && traces[0].DestinationHost == "127.0.0.1" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("kein Auto-Trace für Test %d", got.ID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSchedulerUpdateAndRemove(t *testing.T) {
	e := newEnv(t)
	sv := e.server(t, "a", nil)
	if err := e.sched.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Zeitplan abschalten: keine weiteren Tests.
	sv.ScheduleEnabled = false
	if e.sched.Update(sv) {
		t.Error("Update meldet Einplanung trotz abgeschaltetem Zeitplan")
	}
	time.Sleep(4 * testMinute)
	if n := len(e.tests(t, sv)); n != 0 {
		t.Errorf("nach Abschalten: %d Tests", n)
	}

	// Wieder einschalten, dann entfernen.
	sv.ScheduleEnabled = true
	if !e.sched.Update(sv) {
		t.Fatal("Update plant nicht ein")
	}
	time.Sleep(4 * testMinute)
	e.sched.Remove(sv.ID)
	time.Sleep(testMinute) // ein evtl. gerade laufender Durchlauf endet
	n := len(e.tests(t, sv))
	if n == 0 {
		t.Fatal("nach Einschalten keine Tests")
	}
	time.Sleep(4 * testMinute)
	if m := len(e.tests(t, sv)); m != n {
		t.Errorf("nach Remove weitere Tests: %d → %d", n, m)
	}
}

func TestSchedulerKeepsUnchangedInterval(t *testing.T) {
	e := newEnv(t)
	sv := e.server(t, "a", func(s *model.ServerSettings) { s.ScheduleIntervalMinutes = 3 })
	if err := e.sched.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Wiederholtes Speichern ohne Intervalländerung setzt den Countdown nicht zurück.
	for i := 0; i < 5; i++ {
		time.Sleep(testMinute)
		e.sched.Update(sv)
	}
	if n := len(e.tests(t, sv)); n == 0 {
		t.Error("Countdown wurde durch Update zurückgesetzt")
	}
}

func TestSchedulerNotStarted(t *testing.T) {
	e := newEnv(t)
	sv := e.server(t, "a", nil)
	if e.sched.Update(sv) || e.sched.Running() {
		t.Error("ungestarteter Scheduler darf nichts einplanen")
	}
	e.sched.Remove(sv.ID)
	e.sched.Stop()
}
