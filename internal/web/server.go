// Package web stellt die HTTP-API bereit (ab Phase 9 auch die Oberfläche).
// Pfade, JSON-Felder und Statuscodes entsprechen dem bisherigen Python-Backend,
// damit das React-Frontend bis zur neuen Oberfläche unverändert weiterläuft.
package web

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/scheduler"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/trace"
	"iperf3-tracker/internal/version"
)

// Deps sind die Abhängigkeiten des HTTP-Servers.
type Deps struct {
	Store     *store.Store
	Tokens    *auth.Tokens
	Runner    *iperf.Runner
	Tracer    *trace.Tracer
	Scheduler *scheduler.Scheduler
}

// Server bündelt die HTTP-Handler.
type Server struct {
	store     *store.Store
	tokens    *auth.Tokens
	runner    *iperf.Runner
	tracer    *trace.Tracer
	scheduler *scheduler.Scheduler
	mux       *http.ServeMux

	testTraces testTraces
}

// NewServer erstellt den HTTP-Server.
func NewServer(d Deps) *Server {
	s := &Server{
		store:      d.Store,
		tokens:     d.Tokens,
		runner:     d.Runner,
		tracer:     d.Tracer,
		scheduler:  d.Scheduler,
		mux:        http.NewServeMux(),
		testTraces: testTraces{running: map[int64]bool{}},
	}
	s.routes()
	return s
}

// Handler liefert den HTTP-Handler.
func (s *Server) Handler() http.Handler { return withCORS(s.mux) }

func (s *Server) routes() {
	// Bis zur neuen Oberfläche (Phase 9) liefert "/" wie bisher nur Metadaten.
	s.mux.HandleFunc("GET /{$}", s.handleInfo)
	s.mux.HandleFunc("GET /api/info", s.handleInfo)
	s.mux.HandleFunc("GET /health", s.handleHealth)

	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/init-admin", s.handleInitAdmin)
	s.mux.Handle("GET /api/auth/me", s.requireUser(s.handleMe))
	s.mux.Handle("POST /api/auth/register", s.requireAdmin(s.handleRegister))
	s.mux.Handle("GET /api/auth/users", s.requireAdmin(s.handleListUsers))
	s.mux.Handle("DELETE /api/auth/users/{user_id}", s.requireAdmin(s.handleDeleteUser))

	// Im Python-Backend waren diese Routen ohne Anmeldung erreichbar.
	s.mux.Handle("GET /api/servers", s.requireUser(s.handleListServers))
	s.mux.Handle("POST /api/servers", s.requireUser(s.handleCreateServer))
	s.mux.Handle("GET /api/servers/{server_id}", s.requireUser(s.handleGetServer))
	s.mux.Handle("PUT /api/servers/{server_id}", s.requireUser(s.handleUpdateServer))
	s.mux.Handle("DELETE /api/servers/{server_id}", s.requireUser(s.handleDeleteServer))

	s.mux.Handle("GET /api/tests", s.requireUser(s.handleListTests))
	s.mux.Handle("POST /api/tests/run", s.requireUser(s.handleRunTest))
	s.mux.Handle("GET /api/tests/{test_id}", s.requireUser(s.handleGetTest))
	s.mux.Handle("DELETE /api/tests/{test_id}", s.requireUser(s.handleDeleteTest))
	s.mux.Handle("GET /api/tests/{test_id}/live", s.requireUser(s.handleTestLive))
	s.mux.Handle("GET /api/tests/server/{server_id}/latest", s.requireUser(s.handleLatestTest))

	s.mux.Handle("GET /api/stats/dashboard", s.requireUser(s.handleDashboardStats))
	s.mux.Handle("GET /api/stats/servers", s.requireUser(s.handleServerStatsList))
	s.mux.Handle("GET /api/stats/servers/{server_id}", s.requireUser(s.handleServerStats))

	s.mux.Handle("POST /api/tests/{test_id}/trace", s.requireUser(s.handleStartTestTrace))
	s.mux.Handle("GET /api/tests/{test_id}/trace", s.requireUser(s.handleGetTestTrace))
	s.mux.Handle("DELETE /api/tests/{test_id}/trace", s.requireUser(s.handleDeleteTestTrace))
	s.mux.Handle("POST /api/traces", s.requireUser(s.handleCreateTrace))
	s.mux.Handle("GET /api/traces", s.requireUser(s.handleListTraces))
	s.mux.Handle("GET /api/traces/recent", s.requireUser(s.handleRecentTraces))
	s.mux.Handle("GET /api/traces/{trace_id}", s.requireUser(s.handleGetTrace))
	s.mux.Handle("GET /api/traces/test/{test_id}", s.requireUser(s.handleTracesByTest))
	s.mux.Handle("DELETE /api/traces/{trace_id}", s.requireUser(s.handleDeleteTrace))

	// Anmeldung über ?token=, da EventSource keine Header setzen kann.
	s.mux.HandleFunc("GET /api/live-trace/stream/{destination}", s.handleLiveTrace)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":       version.Name,
		"version":    version.Version,
		"build_date": version.BuildDate,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "healthy",
		"scheduler_running": s.scheduler.Running(),
	})
}

// withCORS entspricht der bisherigen CORSMiddleware (alle Origins, Methoden
// und Header, credentials erlaubt). Wird in Phase 10 auf Same-Origin reduziert.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Add("Vary", "Origin")

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if req := r.Header.Get("Access-Control-Request-Headers"); req != "" {
				h.Set("Access-Control-Allow-Headers", req)
			}
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError schreibt eine Fehlerantwort im FastAPI-Format {"detail": "..."}.
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

// writeInternal protokolliert err und antwortet mit 500, ohne Interna preiszugeben.
func writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("Interner Fehler", "methode", r.Method, "pfad", r.URL.Path, "fehler", err)
	writeError(w, http.StatusInternalServerError, "Interner Serverfehler")
}

// decodeJSON liest den Request-Body nach dst; bei Fehlern wird mit 422
// geantwortet (wie FastAPI bei ungültigen Bodies) und false geliefert.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Ungültiger Request-Body: "+err.Error())
		return false
	}
	return true
}
