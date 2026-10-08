package web

import (
	"net/http"
	"testing"

	"iperf3-tracker/internal/auth"
)

func TestAuthFlow(t *testing.T) {
	srv := newTestServer(t)

	expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`, http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/auth/me", "", "", http.StatusUnauthorized)
	expectStatus(t, srv, "POST", "/api/auth/init-admin", "", "", http.StatusBadRequest)

	admin := login(t, srv, "admin", "admin123")
	var me map[string]any
	if code := call(t, srv, "GET", "/api/auth/me", admin, "", &me); code != http.StatusOK ||
		me["username"] != "admin" || me["is_admin"] != true || me["last_login"] == nil {
		t.Fatalf("/me: Status %d, %v", code, me)
	}

	var bob map[string]any
	if code := call(t, srv, "POST", "/api/auth/register", admin, `{"username":"bob","email":"bob@example.org","password":"geheim1"}`, &bob); code != http.StatusOK ||
		bob["is_admin"] != false || bob["last_login"] != nil {
		t.Fatalf("register: Status %d, %v", code, bob)
	}
	expectStatus(t, srv, "POST", "/api/auth/register", admin, `{"username":"bob","email":"x@example.org","password":"geheim1"}`, http.StatusBadRequest)
	expectStatus(t, srv, "POST", "/api/auth/register", admin, `{"username":"al","email":"al@example.org","password":"geheim1"}`, http.StatusUnprocessableEntity)

	user := login(t, srv, "bob", "geheim1")
	expectStatus(t, srv, "GET", "/api/auth/users", user, "", http.StatusForbidden)

	var users []map[string]any
	if code := call(t, srv, "GET", "/api/auth/users", admin, "", &users); code != http.StatusOK || len(users) != 2 {
		t.Fatalf("Benutzerliste: Status %d, %d Einträge", code, len(users))
	}

	expectStatus(t, srv, "DELETE", "/api/auth/users/1", admin, "", http.StatusBadRequest)
	expectStatus(t, srv, "DELETE", "/api/auth/users/99", admin, "", http.StatusNotFound)
	expectStatus(t, srv, "DELETE", "/api/auth/users/2", admin, "", http.StatusOK)
	expectStatus(t, srv, "GET", "/api/auth/me", user, "", http.StatusUnauthorized)
}

func TestForeignTokenRejected(t *testing.T) {
	srv := newTestServer(t)
	foreign, err := auth.NewTokens("anderer-schluessel").Create("admin")
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, srv, "GET", "/api/auth/me", foreign, "", http.StatusUnauthorized)
}
