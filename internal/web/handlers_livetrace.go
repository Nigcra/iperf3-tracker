package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"

	"iperf3-tracker/internal/i18n"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/trace"
)

// sseKeepAlive ist der Abstand der Keep-Alive-Kommentare, damit Proxys lange
// Timeout-Hops nicht als tote Verbindung abbrechen.
const sseKeepAlive = 15 * time.Second

// sseStream schreibt Server-Sent Events; send und ping sind nebenläufig nutzbar.
type sseStream struct {
	mu sync.Mutex
	w  http.ResponseWriter
	rc *http.ResponseController
}

func newSSEStream(w http.ResponseWriter) *sseStream {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // nginx: nicht puffern
	w.WriteHeader(http.StatusOK)
	return &sseStream{w: w, rc: http.NewResponseController(w)}
}

// send schreibt v als JSON in ein data-Event (ohne Event-Namen).
func (s *sseStream) send(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.write("data: " + string(data) + "\n\n")
}

func (s *sseStream) ping() { s.write(": ping\n\n") }

func (s *sseStream) write(chunk string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprint(s.w, chunk)
	s.rc.Flush()
}

// handleLiveTrace streamt eine Routenverfolgung Hop für Hop:
// start → hop … → interpolation_complete → complete, im Fehlerfall error.
// Angemeldet wird über das Sitzungs-Cookie (EventSource sendet es mit) oder
// ein Bearer-Token (API-Clients). Auch Anmeldefehler werden als error-Event gemeldet, weil EventSource
// den HTTP-Status nicht auswerten kann. Schließt der Client die Verbindung,
// wird die Verfolgung abgebrochen und nicht gespeichert.
func (s *Server) handleLiveTrace(w http.ResponseWriter, r *http.Request) {
	destination := r.PathValue("destination")
	stream := newSSEStream(w)
	lang := i18n.FromRequest(r)
	fail := func(msg string) {
		stream.send(map[string]string{"type": "error", "message": i18n.Translate(msg, lang)})
	}

	if _, _, err := s.userForRequest(r, false); err != nil {
		if !errors.Is(err, errUnauthorized) && !errors.Is(err, errInactive) && !errors.Is(err, errMustChangePass) {
			slog.Error("Live-Trace: Anmeldung nicht prüfbar", "error", err)
			err = errors.New("Interner Serverfehler")
		}
		fail(err.Error())
		return
	}
	if n := utf8.RuneCountInString(destination); n < 1 || n > 255 {
		fail("Ziel muss 1–255 Zeichen lang sein")
		return
	}

	ctx := r.Context()
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		t := time.NewTicker(sseKeepAlive)
		defer t.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				stream.ping()
			}
		}
	}()

	stream.send(map[string]string{"type": "start", "destination": destination})
	tr := s.tracer.Run(ctx, destination, trace.DefaultOptions(), func(h model.TraceHop) {
		stream.send(map[string]any{"type": "hop", "data": h})
	})

	if ctx.Err() != nil {
		slog.Info("Live-Trace vom Client abgebrochen", "destination", destination)
		return
	}
	if len(tr.Hops) == 0 && tr.ErrorMessage != nil {
		fail(*tr.ErrorMessage)
		return
	}
	stream.send(map[string]any{"type": "interpolation_complete", "hops": tr.Hops})

	// Speichern vor "complete": Die Oberfläche lädt danach die Historie neu.
	if err := s.store.CreateTrace(context.WithoutCancel(ctx), tr); err != nil {
		slog.Error("Live-Trace konnte nicht gespeichert werden", "destination", destination, "error", err)
		fail("Trace konnte nicht gespeichert werden")
		return
	}
	stream.send(map[string]any{"type": "complete", "trace_id": tr.ID})
}
