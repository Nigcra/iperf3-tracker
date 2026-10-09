package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"iperf3-tracker/internal/model"
)

// liveTestsInterval ist der Abstand, in dem der Server den Live-Status prüft;
// gesendet wird nur bei Änderungen.
const liveTestsInterval = 500 * time.Millisecond

// liveTest ist ein Eintrag im Live-Strom (Felder wie GET /api/tests/{id}/live).
type liveTest struct {
	TestID              int64            `json:"test_id"`
	ServerID            int64            `json:"server_id"`
	ServerName          string           `json:"server_name"`
	IsRunning           bool             `json:"is_running"`
	Status              model.TestStatus `json:"status"`
	Progress            int              `json:"progress"`
	ElapsedSeconds      int              `json:"elapsed_seconds"`
	TotalSeconds        int              `json:"total_seconds"`
	CurrentDownloadMbps float64          `json:"current_download_mbps"`
	CurrentUploadMbps   float64          `json:"current_upload_mbps"`
}

// handleLiveTestsStream sendet per Server-Sent Events den Live-Status aller
// wartenden, laufenden und gerade beendeten Tests:
// {"type":"live","tests":[…]} – sofort beim Verbinden und danach bei jeder
// Änderung. Ein beendeter Test bleibt noch 10 s mit seinen Endwerten in der
// Liste und fällt dann heraus. Ersetzt das Abfragen von /api/tests/{id}/live.
func (s *Server) handleLiveTestsStream(w http.ResponseWriter, r *http.Request, _ *model.User) {
	stream := newSSEStream(w)
	ctx := r.Context()
	names := map[int64]string{}

	var last []byte
	send := func() {
		data, err := json.Marshal(map[string]any{"type": "live", "tests": s.liveTests(ctx, names)})
		if err != nil || string(data) == string(last) {
			return
		}
		last = data
		stream.write("data: " + string(data) + "\n\n")
	}
	send()

	tick := time.NewTicker(liveTestsInterval)
	defer tick.Stop()
	ping := time.NewTicker(sseKeepAlive)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			send()
		case <-ping.C:
			stream.ping()
		}
	}
}

// liveTests baut die Liste für den Live-Strom; names puffert Servernamen.
func (s *Server) liveTests(ctx context.Context, names map[int64]string) []liveTest {
	all := s.runner.LiveAll()
	out := make([]liveTest, 0, len(all))
	for id, l := range all {
		name, ok := names[l.ServerID]
		if !ok {
			if sv, err := s.store.ServerByID(ctx, l.ServerID); err == nil {
				name = sv.Name
				names[l.ServerID] = name
			}
		}
		out = append(out, liveTest{
			TestID: id, ServerID: l.ServerID, ServerName: name, IsRunning: true,
			Status: l.Status, Progress: l.Progress, ElapsedSeconds: l.ElapsedSeconds, TotalSeconds: l.TotalSeconds,
			CurrentDownloadMbps: l.CurrentDownloadMbps, CurrentUploadMbps: l.CurrentUploadMbps,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TestID < out[j].TestID })
	return out
}
