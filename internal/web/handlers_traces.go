package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"unicode/utf8"

	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/trace"
)

const msgTraceNotFound = "Trace nicht gefunden"

// testTraces verhindert, dass für denselben Test mehrere Traces gleichzeitig laufen.
type testTraces struct {
	mu      sync.Mutex
	running map[int64]bool
}

func (tt *testTraces) start(testID int64) bool {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	if tt.running[testID] {
		return false
	}
	tt.running[testID] = true
	return true
}

func (tt *testTraces) done(testID int64) {
	tt.mu.Lock()
	delete(tt.running, testID)
	tt.mu.Unlock()
}

// handleStartTestTrace startet im Hintergrund einen Trace zum Server eines Tests.
func (s *Server) handleStartTestTrace(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "test_id")
	if !ok {
		return
	}
	t, err := s.store.TestByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, msgTestNotFound)
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	sv, err := s.store.ServerByID(r.Context(), t.ServerID)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if _, err := s.store.TraceByTestID(r.Context(), id); err == nil {
		writeError(w, r, http.StatusBadRequest, "Für diesen Test existiert bereits ein Trace")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeInternal(w, r, err)
		return
	}
	if !s.testTraces.start(id) {
		writeError(w, r, http.StatusBadRequest, "Für diesen Test läuft bereits ein Trace")
		return
	}

	go func() {
		defer s.testTraces.done(id)
		tr := s.tracer.Run(context.Background(), sv.Host, trace.DefaultOptions(), nil)
		tr.TestID = &id
		if err := s.store.CreateTrace(context.Background(), tr); err != nil {
			slog.Error("Trace konnte nicht gespeichert werden", "test", id, "error", err)
			return
		}
		slog.Info("Trace abgeschlossen", "test", id, "hops", tr.TotalHops, "completed", tr.Completed)
	}()
	writeJSON(w, r, http.StatusAccepted, map[string]any{"message": "Trace gestartet", "test_id": id})
}

func (s *Server) handleGetTestTrace(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "test_id")
	if !ok {
		return
	}
	s.writeTrace(w, r, func(ctx context.Context) (*model.Trace, error) { return s.store.TraceByTestID(ctx, id) })
}

func (s *Server) handleDeleteTestTrace(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "test_id")
	if !ok {
		return
	}
	s.deleteTrace(w, r, s.store.DeleteTracesByTest(r.Context(), id))
}

// handleCreateTrace führt einen Trace ohne Test synchron aus (bis zu 90 s).
func (s *Server) handleCreateTrace(w http.ResponseWriter, r *http.Request, _ *model.User) {
	def := trace.DefaultOptions()
	req := struct {
		Destination string `json:"destination"`
		MaxHops     int    `json:"max_hops"`
		Timeout     int    `json:"timeout"`
		Count       int    `json:"count"`
	}{MaxHops: def.MaxHops, Timeout: def.WaitSec, Count: def.Probes}
	if !decodeJSON(w, r, &req) {
		return
	}
	var msg string
	switch n := utf8.RuneCountInString(req.Destination); {
	case n < 1 || n > 255:
		msg = "Ziel muss 1–255 Zeichen lang sein"
	case req.MaxHops < 1 || req.MaxHops > 64:
		msg = "max_hops muss zwischen 1 und 64 liegen"
	case req.Timeout < 1 || req.Timeout > 10:
		msg = "timeout muss zwischen 1 und 10 Sekunden liegen"
	case req.Count < 1 || req.Count > 10:
		msg = "count muss zwischen 1 und 10 liegen"
	}
	if msg != "" {
		writeError(w, r, http.StatusUnprocessableEntity, msg)
		return
	}

	tr := s.tracer.Run(r.Context(), req.Destination, trace.Options{MaxHops: req.MaxHops, WaitSec: req.Timeout, Probes: req.Count}, nil)
	if err := s.store.CreateTrace(r.Context(), tr); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, tr)
}

func (s *Server) handleGetTrace(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "trace_id")
	if !ok {
		return
	}
	s.writeTrace(w, r, func(ctx context.Context) (*model.Trace, error) { return s.store.TraceByID(ctx, id) })
}

func (s *Server) handleListTraces(w http.ResponseWriter, r *http.Request, _ *model.User) {
	limit, err := queryInt(r, "limit", 10, 1, 1000)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}
	traces, err := s.store.ListTraces(r.Context(), limit)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, traces)
}

func (s *Server) handleTracesByTest(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "test_id")
	if !ok {
		return
	}
	traces, err := s.store.TracesByTest(r.Context(), id)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, traces)
}

func (s *Server) handleDeleteTrace(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "trace_id")
	if !ok {
		return
	}
	s.deleteTrace(w, r, s.store.DeleteTrace(r.Context(), id))
}

func (s *Server) writeTrace(w http.ResponseWriter, r *http.Request, load func(context.Context) (*model.Trace, error)) {
	tr, err := load(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, msgTraceNotFound)
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, tr)
}

func (s *Server) deleteTrace(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, msgTraceNotFound)
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"message": "Trace gelöscht"})
}
