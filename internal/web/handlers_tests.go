package web

import (
	"errors"
	"fmt"
	"net/http"

	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

const msgTestNotFound = "Test nicht gefunden"

func (s *Server) handleListTests(w http.ResponseWriter, r *http.Request, _ *model.User) {
	var f store.TestFilter
	var err error
	fail := func(err error) { writeError(w, http.StatusUnprocessableEntity, err.Error()) }

	if f.ServerID, err = queryInt64(r, "server_id"); err != nil {
		fail(err)
		return
	}
	if v := r.URL.Query().Get("status"); v != "" {
		st := model.TestStatus(v)
		if !st.Valid() {
			fail(fmt.Errorf("Parameter \"status\" muss pending, running, completed oder failed sein"))
			return
		}
		f.Status = &st
	}
	if f.From, err = queryTime(r, "from_date"); err != nil {
		fail(err)
		return
	}
	if f.To, err = queryTime(r, "to_date"); err != nil {
		fail(err)
		return
	}
	if f.Skip, f.Limit, err = paging(r, 100, 1000); err != nil {
		fail(err)
		return
	}

	tests, err := s.store.ListTests(r.Context(), f)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tests)
}

func (s *Server) handleGetTest(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "test_id")
	if !ok {
		return
	}
	t, err := s.store.TestDetailByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgTestNotFound)
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleRunTest(w http.ResponseWriter, r *http.Request, _ *model.User) {
	// Vorgabewerte wie im bisherigen Schema TestCreate.
	req := struct {
		ServerID        int64           `json:"server_id"`
		Protocol        model.Protocol  `json:"protocol"`
		Direction       model.Direction `json:"direction"`
		Duration        int             `json:"duration"`
		ParallelStreams int             `json:"parallel_streams"`
		// Optional; ohne Angabe gilt bei UDP die Vorgabe des Servers.
		UDPBandwidthMbps *float64 `json:"udp_bandwidth_mbps"`
	}{Protocol: model.ProtocolTCP, Direction: model.DirectionDownload, Duration: 10, ParallelStreams: 1}
	if !decodeJSON(w, r, &req) {
		return
	}
	switch {
	case !req.Protocol.Valid():
		writeError(w, http.StatusUnprocessableEntity, "Protokoll muss tcp oder udp sein")
		return
	case !req.Direction.Valid():
		writeError(w, http.StatusUnprocessableEntity, "Richtung muss download, upload oder bidirectional sein")
		return
	case req.Duration < 1 || req.Duration > 300:
		writeError(w, http.StatusUnprocessableEntity, "Testdauer muss zwischen 1 und 300 Sekunden liegen")
		return
	case req.ParallelStreams < 1 || req.ParallelStreams > 128:
		writeError(w, http.StatusUnprocessableEntity, "Parallele Streams müssen zwischen 1 und 128 liegen")
		return
	}
	if msg := model.ValidateUDPBandwidth(req.UDPBandwidthMbps); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}

	sv, err := s.store.ServerByID(r.Context(), req.ServerID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Server %d nicht gefunden", req.ServerID))
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if !sv.Enabled {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Server %s ist deaktiviert", sv.Name))
		return
	}

	t := &model.Test{
		ServerID:        sv.ID,
		Protocol:        req.Protocol,
		Direction:       req.Direction,
		Duration:        req.Duration,
		ParallelStreams: req.ParallelStreams,
	}
	if t.Protocol == model.ProtocolUDP {
		t.UDPBandwidthMbps = req.UDPBandwidthMbps
		if t.UDPBandwidthMbps == nil {
			t.UDPBandwidthMbps = sv.DefaultUDPBandwidthMbps
		}
	}
	if err := s.store.CreateTest(r.Context(), t); err != nil {
		writeInternal(w, r, err)
		return
	}
	s.runner.Submit(t)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleDeleteTest(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "test_id")
	if !ok {
		return
	}
	if err := s.store.DeleteTest(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgTestNotFound)
		return
	} else if err != nil {
		writeInternal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleLatestTest liefert den jüngsten erfolgreichen Test eines Servers oder null.
func (s *Server) handleLatestTest(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "server_id")
	if !ok {
		return
	}
	t, err := s.store.LatestCompletedTest(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// handleTestLive liefert den Live-Status für die Fortschrittsanzeige. Solange
// der Runner einen Live-Eintrag hält (bis 10 s nach Testende), gilt der Test
// als laufend – die Oberfläche zeigt so noch die Endwerte an.
func (s *Server) handleTestLive(w http.ResponseWriter, r *http.Request, _ *model.User) {
	id, ok := pathID(w, r, "test_id")
	if !ok {
		return
	}
	t, err := s.store.TestByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgTestNotFound)
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

	resp := map[string]any{"test_id": t.ID, "server_name": sv.Name}
	if l, ok := s.runner.Live(id); ok {
		resp["is_running"] = true
		resp["status"] = l.Status
		resp["progress"] = l.Progress
		resp["elapsed_seconds"] = l.ElapsedSeconds
		resp["total_seconds"] = l.TotalSeconds
		resp["current_download_mbps"] = l.CurrentDownloadMbps
		resp["current_upload_mbps"] = l.CurrentUploadMbps
	} else {
		progress := 0
		if t.Status == model.StatusCompleted {
			progress = 100
		}
		resp["is_running"] = t.Status == model.StatusRunning || t.Status == model.StatusPending
		resp["status"] = t.Status
		resp["progress"] = progress
		resp["elapsed_seconds"] = 0
		resp["total_seconds"] = t.Duration
		resp["current_download_mbps"] = valueOrZero(t.DownloadBandwidthMbps)
		resp["current_upload_mbps"] = valueOrZero(t.UploadBandwidthMbps)
	}
	writeJSON(w, http.StatusOK, resp)
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
