package web

import (
	"net/http"
	"testing"
)

func TestStatsRoutes(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	expectStatus(t, srv, "GET", "/api/stats/dashboard", "", "", http.StatusUnauthorized)

	var dash map[string]any
	if code := call(t, srv, "GET", "/api/stats/dashboard", tok, "", &dash); code != http.StatusOK {
		t.Fatalf("dashboard: %d", code)
	}
	for _, k := range []string{"total_servers", "active_servers", "total_tests", "tests_today", "avg_download_mbps", "avg_upload_mbps", "last_test_at"} {
		if _, ok := dash[k]; !ok {
			t.Errorf("dashboard ohne Feld %s", k)
		}
	}

	call(t, srv, "POST", "/api/servers", tok, `{"name":"A","host":"a"}`, nil)
	var list []map[string]any
	if code := call(t, srv, "GET", "/api/stats/servers", tok, "", &list); code != http.StatusOK || len(list) != 1 || list[0]["server_name"] != "A" {
		t.Fatalf("servers: %d, %v", code, list)
	}
	expectStatus(t, srv, "GET", "/api/stats/servers/1", tok, "", http.StatusOK)
	expectStatus(t, srv, "GET", "/api/stats/servers/999", tok, "", http.StatusNotFound)
}

func TestStatusRoute(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")
	expectStatus(t, srv, "GET", "/api/status", "", "", http.StatusUnauthorized)

	var st struct {
		SchedulerRunning bool `json:"scheduler_running"`
		Iperf3           struct {
			Available bool   `json:"available"`
			Path      string `json:"path"`
		} `json:"iperf3"`
	}
	if code := call(t, srv, "GET", "/api/status", tok, "", &st); code != http.StatusOK || st.Iperf3.Path == "" {
		t.Fatalf("status: %d, %+v", code, st)
	}
	// Der Test-Runner hat keine iperf3-Binary.
	if st.Iperf3.Available || st.SchedulerRunning {
		t.Errorf("status: %+v", st)
	}
}
