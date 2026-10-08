package web

import (
	"errors"
	"net/http"
	"time"

	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

// handleDashboardStats liefert die Übersicht. „Heute“ bezieht sich auf die
// lokale Zeitzone des Servers.
func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request, _ *model.User) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	st, err := s.store.DashboardStats(r.Context(), todayStart)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleServerStatsList(w http.ResponseWriter, r *http.Request, _ *model.User) {
	list, err := s.store.ServerStats(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleServerStats(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "server_id")
	if !ok {
		return
	}
	st, err := s.store.ServerStatsByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgServerNotFound)
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}
