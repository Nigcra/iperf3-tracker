package web

import (
	"errors"
	"net/http"

	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

const msgServerNotFound = "Server nicht gefunden"
const msgServerDuplicate = "Ein Server mit diesem Namen existiert bereits"

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request, _ *model.User) {
	enabled, err := queryBool(r, "enabled")
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	skip, limit, err := paging(r, 100, 1000)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	servers, err := s.store.ListServers(r.Context(), enabled, skip, limit)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, servers)
}

func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request, _ *model.User) {
	sv, ok := s.loadServer(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, sv)
}

func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request, _ *model.User) {
	sv := &model.Server{ServerSettings: model.DefaultServerSettings()}
	if !decodeJSON(w, r, &sv.ServerSettings) {
		return
	}
	if msg := sv.Validate(); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}
	if err := s.store.CreateServer(r.Context(), sv); errors.Is(err, store.ErrDuplicate) {
		writeError(w, http.StatusBadRequest, msgServerDuplicate)
		return
	} else if err != nil {
		writeInternal(w, r, err)
		return
	}
	// Phase 7: Scheduler für sv einplanen, falls schedule_enabled.
	writeJSON(w, http.StatusCreated, sv)
}

// handleUpdateServer übernimmt nur die im Body enthaltenen Felder (partielles Update).
func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request, _ *model.User) {
	sv, ok := s.loadServer(w, r)
	if !ok {
		return
	}
	if !decodeJSON(w, r, &sv.ServerSettings) {
		return
	}
	if msg := sv.Validate(); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}
	if err := s.store.UpdateServer(r.Context(), sv); errors.Is(err, store.ErrDuplicate) {
		writeError(w, http.StatusBadRequest, msgServerDuplicate)
		return
	} else if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgServerNotFound)
		return
	} else if err != nil {
		writeInternal(w, r, err)
		return
	}
	// Phase 7: Scheduler für sv neu einplanen bzw. austragen.
	writeJSON(w, http.StatusOK, sv)
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "server_id")
	if !ok {
		return
	}
	// Phase 7: Scheduler für id austragen.
	if err := s.store.DeleteServer(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgServerNotFound)
		return
	} else if err != nil {
		writeInternal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loadServer lädt den Server aus dem Pfadparameter server_id; bei Fehlern ist
// die Antwort bereits geschrieben.
func (s *Server) loadServer(w http.ResponseWriter, r *http.Request) (*model.Server, bool) {
	id, ok := pathID(w, r, "server_id")
	if !ok {
		return nil, false
	}
	sv, err := s.store.ServerByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgServerNotFound)
		return nil, false
	}
	if err != nil {
		writeInternal(w, r, err)
		return nil, false
	}
	return sv, true
}
