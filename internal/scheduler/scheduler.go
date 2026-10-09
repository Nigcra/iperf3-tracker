// Package scheduler führt Tests je Server im eingestellten Intervall aus und
// startet bei aktiviertem Auto-Trace anschließend eine Routenverfolgung.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/trace"
)

// Scheduler verwaltet einen Zeitplan je Server. Ohne Start sind Update und
// Remove wirkungslos (Scheduler per Konfiguration abgeschaltet).
type Scheduler struct {
	store  *store.Store
	runner *iperf.Runner
	tracer *trace.Tracer
	// minute ist die Einheit von schedule_interval_minutes (in Tests verkürzt).
	minute time.Duration

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	jobs    map[int64]*job
	running bool
}

type job struct {
	interval time.Duration
	cancel   context.CancelFunc
}

// New erstellt einen (noch nicht gestarteten) Scheduler.
func New(st *store.Store, runner *iperf.Runner, tracer *trace.Tracer) *Scheduler {
	return &Scheduler{store: st, runner: runner, tracer: tracer, minute: time.Minute, jobs: map[int64]*job{}}
}

// Start plant alle aktiven Server mit eingeschaltetem Zeitplan ein.
func (s *Scheduler) Start(ctx context.Context) error {
	servers, err := s.store.ListServers(ctx, nil, 0, 1<<31-1)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running = true
	s.mu.Unlock()

	n := 0
	for i := range servers {
		if s.Update(&servers[i]) {
			n++
		}
	}
	slog.Info("Scheduler gestartet", "scheduled_servers", n)
	return nil
}

// Stop beendet alle Zeitpläne und wartet auf laufende Durchläufe. Laufende
// Tests bricht der Runner beim eigenen Stop ab.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.cancel()
	s.jobs = map[int64]*job{}
	s.mu.Unlock()
	s.wg.Wait()
}

// Running meldet, ob der Scheduler läuft.
func (s *Scheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Update gleicht den Zeitplan eines Servers mit seinen Einstellungen ab
// (nach Anlegen oder Ändern). Ein unverändertes Intervall läuft weiter, statt
// neu zu beginnen. Liefert true, wenn der Server danach eingeplant ist.
func (s *Scheduler) Update(sv *model.Server) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return false
	}
	if !sv.Enabled || !sv.ScheduleEnabled {
		s.removeLocked(sv.ID)
		return false
	}
	interval := time.Duration(sv.ScheduleIntervalMinutes) * s.minute
	if j, ok := s.jobs[sv.ID]; ok {
		if j.interval == interval {
			return true
		}
		j.cancel()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	id := sv.ID
	s.jobs[id] = &job{interval: interval, cancel: cancel}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.loop(ctx, id, interval)
	}()
	slog.Info("Zeitplan aktiv", "server", sv.Name, "interval", interval)
	return true
}

// Remove trägt einen Server aus (z. B. nach dem Löschen).
func (s *Scheduler) Remove(serverID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(serverID)
}

func (s *Scheduler) removeLocked(serverID int64) {
	if j, ok := s.jobs[serverID]; ok {
		j.cancel()
		delete(s.jobs, serverID)
		slog.Info("Zeitplan entfernt", "server", serverID)
	}
}

// loop führt den Server im Intervall aus. Der erste Lauf erfolgt nach einem
// Intervall; dauert ein Lauf länger, wird nicht parallel gestartet.
func (s *Scheduler) loop(ctx context.Context, serverID int64, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.runOnce(ctx, serverID)
		}
	}
}

// runOnce startet einen Test mit den Vorgaben des Servers, wartet auf das
// Ergebnis und verfolgt bei aktiviertem Auto-Trace anschließend die Route.
func (s *Scheduler) runOnce(ctx context.Context, serverID int64) {
	sv, err := s.store.ServerByID(ctx, serverID)
	if err != nil || !sv.Enabled || !sv.ScheduleEnabled {
		return
	}
	test := model.NewTestFromDefaults(sv)
	if err := s.store.CreateTest(ctx, test); err != nil {
		slog.Error("Geplanter Test konnte nicht angelegt werden", "server", sv.Name, "error", err)
		return
	}
	slog.Info("Geplanter Test", "server", sv.Name, "test", test.ID)
	select {
	case <-s.runner.Submit(test):
	case <-ctx.Done():
		return
	}

	if !sv.AutoTraceEnabled {
		return
	}
	tr := s.tracer.Run(ctx, sv.Host, trace.DefaultOptions(), nil)
	if ctx.Err() != nil {
		return
	}
	tr.TestID = &test.ID
	if err := s.store.CreateTrace(ctx, tr); err != nil {
		slog.Error("Auto-Trace konnte nicht gespeichert werden", "server", sv.Name, "error", err)
		return
	}
	slog.Info("Auto-Trace abgeschlossen", "server", sv.Name, "test", test.ID, "hops", tr.TotalHops)
}
