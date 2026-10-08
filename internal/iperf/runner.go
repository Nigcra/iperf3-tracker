// Package iperf führt iperf3-Tests aus, wertet deren JSON-Stream-Ausgabe aus
// und hält den Live-Status laufender Tests vor.
package iperf

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

// liveRetention ist die Zeit, die der Live-Status nach Testende abrufbar
// bleibt, damit die Oberfläche die Endwerte anzeigen kann.
const liveRetention = 10 * time.Second

// LiveStatus ist der Zwischenstand eines wartenden oder laufenden Tests.
type LiveStatus struct {
	Status              model.TestStatus
	Progress            int
	ElapsedSeconds      int
	TotalSeconds        int
	CurrentDownloadMbps float64
	CurrentUploadMbps   float64
}

// Runner führt Tests nacheinander aus – es läuft immer nur ein iperf3 gleichzeitig.
type Runner struct {
	store   *store.Store
	command []string // iperf3-Binary, ggf. mit festen Vorab-Argumenten (Tests)
	sem     chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	mu   sync.RWMutex
	live map[int64]*LiveStatus
}

// NewRunner erstellt einen Runner. command ist der iperf3-Aufruf, in der
// Regel nur der Pfad zur Binary.
func NewRunner(st *store.Store, command ...string) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		store:   st,
		command: command,
		sem:     make(chan struct{}, 1),
		ctx:     ctx,
		cancel:  cancel,
		live:    map[int64]*LiveStatus{},
	}
}

// Submit reiht einen gespeicherten, wartenden Test zur Ausführung ein.
func (r *Runner) Submit(t *model.Test) {
	r.setLive(t.ID, &LiveStatus{Status: model.StatusPending, TotalSeconds: t.Duration})
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.process(t.ID)
	}()
}

// Stop bricht laufende und wartende Tests ab und wartet, bis deren Status gespeichert ist.
func (r *Runner) Stop() {
	r.cancel()
	r.wg.Wait()
}

// Live liefert den Live-Status eines Tests, solange er wartet, läuft oder
// gerade beendet wurde.
func (r *Runner) Live(id int64) (LiveStatus, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.live[id]
	if !ok {
		return LiveStatus{}, false
	}
	return *l, true
}

func (r *Runner) setLive(id int64, l *LiveStatus) {
	r.mu.Lock()
	r.live[id] = l
	r.mu.Unlock()
}

func (r *Runner) updateLive(id int64, f func(*LiveStatus)) {
	r.mu.Lock()
	if l, ok := r.live[id]; ok {
		f(l)
	}
	r.mu.Unlock()
}

func (r *Runner) deleteLive(id int64) {
	r.mu.Lock()
	delete(r.live, id)
	r.mu.Unlock()
}

// process wartet auf den Ausführungs-Slot, führt den Test aus und speichert das Ergebnis.
func (r *Runner) process(id int64) {
	// Datenbankzugriffe laufen bewusst ohne r.ctx, damit ein Abbruch beim
	// Beenden noch gespeichert wird.
	ctx := context.Background()

	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-r.ctx.Done():
		r.finish(ctx, id, nil, model.TestResult{}, "Abgebrochen: Dienst wurde beendet", "")
		return
	}

	t, err := r.store.TestByID(ctx, id)
	if err != nil {
		// Test wurde zwischenzeitlich gelöscht (oder DB-Fehler): nichts mehr zu tun.
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("Test konnte nicht geladen werden", "test", id, "fehler", err)
		}
		r.deleteLive(id)
		return
	}
	sv, err := r.store.ServerByID(ctx, t.ServerID)
	if err != nil {
		r.finish(ctx, id, t, model.TestResult{}, "Server nicht gefunden", "")
		return
	}

	if err := r.store.StartTest(ctx, id, db.Now()); err != nil {
		slog.Error("Test konnte nicht gestartet werden", "test", id, "fehler", err)
	}
	r.updateLive(id, func(l *LiveStatus) { l.Status = model.StatusRunning })
	slog.Info("Starte iperf3-Test", "test", id, "server", sv.Name, "richtung", t.Direction, "protokoll", t.Protocol)

	res, errMsg, raw := r.execute(t, sv)
	r.finish(ctx, id, t, res, errMsg, raw)
}

// finish speichert das Ergebnis, setzt den Live-Status auf den Endstand und
// entfernt ihn nach liveRetention.
func (r *Runner) finish(ctx context.Context, id int64, t *model.Test, res model.TestResult, errMsg, raw string) {
	status := model.StatusCompleted
	var errPtr, rawPtr *string
	if errMsg != "" {
		status = model.StatusFailed
		errPtr = &errMsg
		slog.Warn("iperf3-Test fehlgeschlagen", "test", id, "fehler", errMsg)
	} else {
		slog.Info("iperf3-Test abgeschlossen", "test", id)
	}
	if raw != "" {
		rawPtr = &raw
	}
	if err := r.store.FinishTest(ctx, id, status, db.Now(), res, errPtr, rawPtr); err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Error("Testergebnis konnte nicht gespeichert werden", "test", id, "fehler", err)
	}

	r.updateLive(id, func(l *LiveStatus) {
		l.Status = status
		if status == model.StatusCompleted {
			l.Progress = 100
			if t != nil {
				l.ElapsedSeconds = t.Duration
			}
			if res.DownloadBandwidthMbps != nil {
				l.CurrentDownloadMbps = *res.DownloadBandwidthMbps
			}
			if res.UploadBandwidthMbps != nil {
				l.CurrentUploadMbps = *res.UploadBandwidthMbps
			}
		}
	})
	time.AfterFunc(liveRetention, func() { r.deleteLive(id) })
}

// execute startet iperf3 und liefert Messwerte, ggf. eine Fehlermeldung und die Rohausgabe.
func (r *Runner) execute(t *model.Test, sv *model.Server) (res model.TestResult, errMsg, raw string) {
	timeout := time.Duration(t.Duration)*time.Second + 30*time.Second
	ctx, cancel := context.WithTimeout(r.ctx, timeout)
	defer cancel()

	args := append(append([]string{}, r.command[1:]...), Args(sv.Host, sv.Port, t)...)
	cmd := exec.CommandContext(ctx, r.command[0], args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return res, err.Error(), ""
	}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return res, "iperf3 nicht gefunden – bitte installieren oder iperf.path konfigurieren", ""
		}
		return res, "iperf3 konnte nicht gestartet werden: " + err.Error(), ""
	}

	stopProgress := r.trackProgress(t.ID, t.Duration)
	defer stopProgress()

	var out strings.Builder
	var end *endData
	var iperfErr string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		line := sc.Bytes()
		out.Write(line)
		out.WriteByte('\n')

		var ev event
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		switch ev.Event {
		case "interval":
			var d intervalData
			if json.Unmarshal(ev.Data, &d) == nil {
				rates := intervalRates(d, t.Direction)
				r.updateLive(t.ID, func(l *LiveStatus) {
					if rates.DownloadMbps != nil {
						l.CurrentDownloadMbps = *rates.DownloadMbps
					}
					if rates.UploadMbps != nil {
						l.CurrentUploadMbps = *rates.UploadMbps
					}
				})
			}
		case "end":
			var d endData
			if json.Unmarshal(ev.Data, &d) == nil {
				end = &d
			}
		case "error":
			json.Unmarshal(ev.Data, &iperfErr)
		}
	}
	waitErr := cmd.Wait()

	raw = out.String()
	if raw == "" {
		raw = stderr.String()
	}
	switch {
	case r.ctx.Err() != nil:
		return res, "Abgebrochen: Dienst wurde beendet", raw
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return res, fmt.Sprintf("Zeitüberschreitung nach %s", timeout), raw
	case iperfErr != "":
		return res, iperfErr, raw
	case waitErr != nil:
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return res, msg, raw
		}
		return res, "iperf3 beendet mit Fehler: " + waitErr.Error(), raw
	case end == nil:
		return res, "iperf3 lieferte keine Ergebnisse", raw
	}
	res, ok := endResult(*end, t.Direction)
	if !ok {
		return res, fmt.Sprintf("iperf3 lieferte keine Ergebnisse für die Richtung %s", t.Direction), raw
	}
	return res, "", raw
}

// trackProgress aktualisiert Fortschritt und Laufzeit alle 500 ms. Die
// zurückgegebene Funktion beendet die Aktualisierung.
func (r *Runner) trackProgress(id int64, duration int) (stop func()) {
	start := time.Now()
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				elapsed := time.Since(start).Seconds()
				r.updateLive(id, func(l *LiveStatus) {
					l.ElapsedSeconds = int(elapsed)
					l.Progress = min(100, int(elapsed/float64(duration)*100))
				})
			}
		}
	}()
	return func() { close(done) }
}

// Args baut die iperf3-Argumente für einen Test. Download nutzt -R (der
// Server sendet), Upload den Normalmodus (der Client sendet). Im
// Python-Backend war diese Zuordnung vertauscht.
func Args(host string, port int, t *model.Test) []string {
	args := []string{
		"-c", host,
		"-p", strconv.Itoa(port),
		"-t", strconv.Itoa(t.Duration),
		"-P", strconv.Itoa(t.ParallelStreams),
		"-i", "1",
		"--json-stream",
		"--forceflush",
	}
	if t.Protocol == model.ProtocolUDP {
		args = append(args, "-u")
	}
	switch t.Direction {
	case model.DirectionDownload:
		args = append(args, "-R")
	case model.DirectionBidirectional:
		args = append(args, "--bidir")
	}
	return args
}
