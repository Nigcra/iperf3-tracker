package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/scheduler"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/trace"
)

// newTestServer startet die API auf einer frischen Datenbank mit Standard-Admin.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWithTracer(t, nil)
}

// newTestServerWithTracer erlaubt einen eigenen Tracer; nil = ohne traceroute-Binary.
func newTestServerWithTracer(t *testing.T, tracer *trace.Tracer) *httptest.Server {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	st := store.New(conn)
	admin, err := auth.NewAdmin("admin", auth.DefaultAdminEmail, "admin123", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(context.Background(), admin); err != nil {
		t.Fatal(err)
	}

	// Nicht vorhandene Binaries: Tests und Traces schlagen sofort und
	// deterministisch fehl. Die eigentliche Ausführung testen die Pakete iperf
	// und trace.
	runner := iperf.NewRunner(st, filepath.Join(t.TempDir(), "kein-iperf3"))
	t.Cleanup(runner.Stop)

	if tracer == nil {
		tracer = trace.NewTracer(nil, filepath.Join(t.TempDir(), "kein-traceroute"))
	}
	// Scheduler wird nicht gestartet (wie bei scheduler.enabled: false).
	srv := httptest.NewServer(NewServer(Deps{
		Store:     st,
		Tokens:    auth.NewTokens("test-secret"),
		Runner:    runner,
		Tracer:    tracer,
		Scheduler: scheduler.New(st, runner, tracer),
	}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

// call führt einen Request aus, dekodiert die Antwort nach out (falls nicht
// nil und ein Body vorhanden ist) und liefert den Statuscode.
func call(t *testing.T, srv *httptest.Server, method, path, token, body string, out any) int {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: Antwort nicht lesbar: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func login(t *testing.T, srv *httptest.Server, username, password string) string {
	t.Helper()
	var out map[string]any
	code := call(t, srv, "POST", "/api/auth/login", "", `{"username":"`+username+`","password":"`+password+`"}`, &out)
	if code != http.StatusOK {
		t.Fatalf("Login %s: Status %d, %v", username, code, out)
	}
	if out["token_type"] != "bearer" {
		t.Fatalf("token_type = %v", out["token_type"])
	}
	return out["access_token"].(string)
}

// expectStatus prüft nur den Statuscode eines Requests.
func expectStatus(t *testing.T, srv *httptest.Server, method, path, token, body string, want int) {
	t.Helper()
	if got := call(t, srv, method, path, token, body, nil); got != want {
		t.Errorf("%s %s: Status %d, erwartet %d", method, path, got, want)
	}
}
