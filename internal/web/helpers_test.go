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
	"iperf3-tracker/internal/store"
)

// newTestServer startet die API auf einer frischen Datenbank mit Standard-Admin.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	st := store.New(conn)
	admin, err := auth.DefaultAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(context.Background(), admin); err != nil {
		t.Fatal(err)
	}

	// Nicht vorhandene Binary: Tests schlagen sofort und deterministisch fehl.
	// Die eigentliche Ausführung testet das Paket iperf.
	runner := iperf.NewRunner(st, filepath.Join(t.TempDir(), "kein-iperf3"))
	t.Cleanup(runner.Stop)

	srv := httptest.NewServer(NewServer(st, auth.NewTokens("test-secret"), runner).Handler())
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
