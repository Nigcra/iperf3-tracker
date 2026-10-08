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

	srv := httptest.NewServer(NewServer(st, auth.NewTokens("test-secret")).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, srv *httptest.Server, method, path, token, body string) (int, map[string]any) {
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
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out) // Listen-Antworten bleiben nil
	return resp.StatusCode, out
}

func login(t *testing.T, srv *httptest.Server, username, password string) string {
	t.Helper()
	code, out := call(t, srv, "POST", "/api/auth/login", "", `{"username":"`+username+`","password":"`+password+`"}`)
	if code != http.StatusOK {
		t.Fatalf("Login %s: Status %d, %v", username, code, out)
	}
	if out["token_type"] != "bearer" {
		t.Fatalf("token_type = %v", out["token_type"])
	}
	return out["access_token"].(string)
}

func TestAuthFlow(t *testing.T) {
	srv := newTestServer(t)

	if code, _ := call(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`); code != http.StatusUnauthorized {
		t.Errorf("falsches Passwort: Status %d, erwartet 401", code)
	}
	if code, _ := call(t, srv, "GET", "/api/auth/me", "", ""); code != http.StatusUnauthorized {
		t.Errorf("ohne Token: Status %d, erwartet 401", code)
	}
	if code, _ := call(t, srv, "POST", "/api/auth/init-admin", "", ""); code != http.StatusBadRequest {
		t.Errorf("init-admin mit vorhandenen Benutzern: Status %d, erwartet 400", code)
	}

	admin := login(t, srv, "admin", "admin123")
	code, me := call(t, srv, "GET", "/api/auth/me", admin, "")
	if code != http.StatusOK || me["username"] != "admin" || me["is_admin"] != true || me["last_login"] == nil {
		t.Fatalf("/me: Status %d, %v", code, me)
	}

	code, bob := call(t, srv, "POST", "/api/auth/register", admin, `{"username":"bob","email":"bob@example.org","password":"geheim1"}`)
	if code != http.StatusOK || bob["is_admin"] != false || bob["last_login"] != nil {
		t.Fatalf("register: Status %d, %v", code, bob)
	}
	if code, _ := call(t, srv, "POST", "/api/auth/register", admin, `{"username":"bob","email":"x@example.org","password":"geheim1"}`); code != http.StatusBadRequest {
		t.Errorf("doppelter Benutzername: Status %d, erwartet 400", code)
	}
	if code, _ := call(t, srv, "POST", "/api/auth/register", admin, `{"username":"al","email":"al@example.org","password":"geheim1"}`); code != http.StatusUnprocessableEntity {
		t.Errorf("zu kurzer Benutzername: Status %d, erwartet 422", code)
	}

	user := login(t, srv, "bob", "geheim1")
	if code, _ := call(t, srv, "GET", "/api/auth/users", user, ""); code != http.StatusForbidden {
		t.Errorf("Benutzerliste als Nicht-Admin: Status %d, erwartet 403", code)
	}
	if code, _ := call(t, srv, "DELETE", "/api/auth/users/1", admin, ""); code != http.StatusBadRequest {
		t.Errorf("eigenes Konto löschen: Status %d, erwartet 400", code)
	}
	if code, _ := call(t, srv, "DELETE", "/api/auth/users/99", admin, ""); code != http.StatusNotFound {
		t.Errorf("unbekannter Benutzer: Status %d, erwartet 404", code)
	}
	if code, _ := call(t, srv, "DELETE", "/api/auth/users/2", admin, ""); code != http.StatusOK {
		t.Errorf("Benutzer löschen: Status %d, erwartet 200", code)
	}
	if code, _ := call(t, srv, "GET", "/api/auth/me", user, ""); code != http.StatusUnauthorized {
		t.Errorf("Token eines gelöschten Benutzers: Status %d, erwartet 401", code)
	}
}

func TestForeignTokenRejected(t *testing.T) {
	srv := newTestServer(t)
	foreign, err := auth.NewTokens("anderer-schluessel").Create("admin")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := call(t, srv, "GET", "/api/auth/me", foreign, ""); code != http.StatusUnauthorized {
		t.Errorf("fremd signiertes Token: Status %d, erwartet 401", code)
	}
}
