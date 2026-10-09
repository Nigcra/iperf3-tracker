package web

import (
	"net/http"
	"strings"
	"testing"

	"iperf3-tracker/internal/auth"
)

func TestAuthFlow(t *testing.T) {
	srv := newTestServer(t)

	expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`, http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/auth/me", "", "", http.StatusUnauthorized)

	admin := login(t, srv, "admin", "admin123")
	var me map[string]any
	if code := call(t, srv, "GET", "/api/auth/me", admin, "", &me); code != http.StatusOK ||
		me["username"] != "admin" || me["is_admin"] != true || me["last_login"] == nil {
		t.Fatalf("/me: Status %d, %v", code, me)
	}
	if _, ok := me["token_version"]; ok {
		t.Errorf("/me verrät die Token-Version: %v", me)
	}

	var bob map[string]any
	if code := call(t, srv, "POST", "/api/auth/register", admin, `{"username":"bob","email":"bob@example.org","password":"geheimPasswort1"}`, &bob); code != http.StatusOK ||
		bob["is_admin"] != false || bob["last_login"] != nil {
		t.Fatalf("register: Status %d, %v", code, bob)
	}
	expectStatus(t, srv, "POST", "/api/auth/register", admin, `{"username":"bob","email":"x@example.org","password":"geheimPasswort1"}`, http.StatusBadRequest)
	expectStatus(t, srv, "POST", "/api/auth/register", admin, `{"username":"al","email":"al@example.org","password":"geheimPasswort1"}`, http.StatusUnprocessableEntity)

	user := login(t, srv, "bob", "geheimPasswort1")
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

// TestInitAdminRemoved: Den ersten Admin legt EnsureAdmin beim Start an; der
// frühere öffentliche Endpunkt existiert nicht mehr.
func TestInitAdminRemoved(t *testing.T) {
	srv, _ := newServerWithStore(t)
	expectStatus(t, srv, "POST", "/api/auth/init-admin", "", "", http.StatusNotFound)
}

func TestForeignTokenRejected(t *testing.T) {
	srv := newTestServer(t)
	foreign, _, err := auth.NewTokens("anderer-schluessel", 0).Create("admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, srv, "GET", "/api/auth/me", foreign, "", http.StatusUnauthorized)
}

// TestPasswordMinLength: Neue Passwörter brauchen mindestens 10 Zeichen
// (Zeichen, nicht Byte); bestehende kürzere wie admin123 bleiben gültig.
func TestPasswordMinLength(t *testing.T) {
	srv := newTestServer(t)
	admin := login(t, srv, "admin", "admin123")

	var out map[string]any
	if code := call(t, srv, "POST", "/api/auth/register", admin, `{"username":"neun","email":"n@example.org","password":"123456789"}`, &out); code != http.StatusUnprocessableEntity ||
		out["detail"] != "Passwort muss mindestens 10 Zeichen lang sein" {
		t.Errorf("9 Zeichen: Status %d, %v", code, out)
	}
	expectStatus(t, srv, "POST", "/api/auth/register", admin, `{"username":"zehn","email":"z@example.org","password":"1234567890"}`, http.StatusOK)
	// 10 Zeichen mit Umlauten sind mehr als 10 Byte, aber gültig; 9 Umlaute nicht.
	expectStatus(t, srv, "POST", "/api/auth/register", admin, `{"username":"umlaut9","email":"u9@example.org","password":"äöüäöüäöü"}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/auth/register", admin, `{"username":"umlaut10","email":"u10@example.org","password":"äöüäöüäöüß"}`, http.StatusOK)

	expectStatus(t, srv, "POST", "/api/auth/change-password", admin, `{"current_password":"admin123","new_password":"123456789"}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/auth/change-password", admin, `{"current_password":"admin123","new_password":"`+strings.Repeat("x", 257)+`"}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/auth/change-password", admin, `{"current_password":"admin123","new_password":"1234567890"}`, http.StatusOK)
}

// TestChangePasswordEndsOtherSessions: Nach dem Passwortwechsel gelten alle
// bisherigen Tokens des Benutzers nicht mehr; die eigene Sitzung läuft mit
// dem neuen Token aus der Antwort weiter. Andere Benutzer bleiben angemeldet.
func TestChangePasswordEndsOtherSessions(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")
	other := login(t, srv, "admin", "admin123")
	expectStatus(t, srv, "POST", "/api/auth/register", tok, `{"username":"bob","email":"bob@example.org","password":"geheimPasswort1"}`, http.StatusOK)
	bob := login(t, srv, "bob", "geheimPasswort1")

	expectStatus(t, srv, "POST", "/api/auth/change-password", "", `{}`, http.StatusUnauthorized)
	expectStatus(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"falsch","new_password":"neuesPasswort"}`, http.StatusBadRequest)
	expectStatus(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"admin123","new_password":"kurz"}`, http.StatusUnprocessableEntity)
	// Fehlgeschlagene Versuche beenden keine Sitzung.
	expectStatus(t, srv, "GET", "/api/auth/me", other, "", http.StatusOK)

	var out map[string]any
	if code := call(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"admin123","new_password":"neuesPasswort"}`, &out); code != http.StatusOK {
		t.Fatalf("change-password: %d %v", code, out)
	}
	fresh, _ := out["access_token"].(string)
	if fresh == "" || fresh == tok || out["csrf_token"] == "" {
		t.Fatalf("Antwort ohne neues Token: %v", out)
	}

	expectStatus(t, srv, "GET", "/api/auth/me", tok, "", http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/auth/me", other, "", http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/auth/me", fresh, "", http.StatusOK)
	expectStatus(t, srv, "GET", "/api/auth/me", bob, "", http.StatusOK)

	expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"admin123"}`, http.StatusUnauthorized)
	login(t, srv, "admin", "neuesPasswort")
}

// TestLogoutRevokesBearer: Abmelden sperrt das Token auch für API-Clients,
// andere Sitzungen desselben Benutzers bleiben bestehen.
func TestLogoutRevokesBearer(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")
	other := login(t, srv, "admin", "admin123")

	expectStatus(t, srv, "POST", "/api/auth/logout", tok, "", http.StatusNoContent)
	expectStatus(t, srv, "GET", "/api/auth/me", tok, "", http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/servers", tok, "", http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/auth/me", other, "", http.StatusOK)
	// Abmelden ohne oder mit ungültigem Token ist harmlos.
	expectStatus(t, srv, "POST", "/api/auth/logout", "", "", http.StatusNoContent)
	expectStatus(t, srv, "POST", "/api/auth/logout", "kein-token", "", http.StatusNoContent)
}

// TestLogoutAll beendet alle Sitzungen des Benutzers, nicht die anderer.
func TestLogoutAll(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")
	other := login(t, srv, "admin", "admin123")
	expectStatus(t, srv, "POST", "/api/auth/register", tok, `{"username":"bob","email":"bob@example.org","password":"geheimPasswort1"}`, http.StatusOK)
	bob := login(t, srv, "bob", "geheimPasswort1")

	expectStatus(t, srv, "POST", "/api/auth/logout-all", "", "", http.StatusUnauthorized)
	expectStatus(t, srv, "POST", "/api/auth/logout-all", tok, "", http.StatusNoContent)
	expectStatus(t, srv, "GET", "/api/auth/me", tok, "", http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/auth/me", other, "", http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/auth/me", bob, "", http.StatusOK)
	// Neue Anmeldung funktioniert wieder.
	expectStatus(t, srv, "GET", "/api/auth/me", login(t, srv, "admin", "admin123"), "", http.StatusOK)
}

// TestLoginRateLimit: Nach 10 Fehlversuchen je Client-Adresse wird jede
// Anmeldung mit 429 abgewiesen – auch mit richtigem Passwort und auch bei
// gefälschtem X-Forwarded-For.
func TestLoginRateLimit(t *testing.T) {
	srv := newTestServer(t)
	for i := 0; i < auth.LoginMaxFailures; i++ {
		expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`, http.StatusUnauthorized)
	}
	req, _ := http.NewRequest("POST", srv.URL+"/api/auth/login", strings.NewReader(`{"username":"admin","password":"admin123"}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.Header.Set("Accept-Language", "en")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("nach %d Fehlversuchen: Status %d, Retry-After %q", auth.LoginMaxFailures, resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	var out map[string]any
	if code := call(t, srv, "POST", "/api/auth/login", "", `{"username":"unbekannt","password":"x"}`, &out); code != http.StatusTooManyRequests ||
		out["detail"] != "Zu viele Fehlversuche – bitte später erneut versuchen" {
		t.Errorf("gesperrt: Status %d, %v", code, out)
	}
}

// TestChangePasswordRateLimit: Ein falsches aktuelles Passwort zählt als
// Fehlversuch; danach sind Passwortwechsel und Login gesperrt.
func TestChangePasswordRateLimit(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")
	for i := 0; i < auth.LoginMaxFailures; i++ {
		expectStatus(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"falsch","new_password":"neuesPasswort"}`, http.StatusBadRequest)
	}
	expectStatus(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"admin123","new_password":"neuesPasswort"}`, http.StatusTooManyRequests)
	expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"admin123"}`, http.StatusTooManyRequests)
	// Die bestehende Sitzung bleibt nutzbar.
	expectStatus(t, srv, "GET", "/api/auth/me", tok, "", http.StatusOK)
}

// TestLoginUnknownUserChecksHash: Auch für unbekannte Benutzer wird ein
// Argon2id-Hash mit den aktuellen Parametern geprüft (gleiche Antwortzeit).
func TestLoginUnknownUserChecksHash(t *testing.T) {
	srv := newTestServer(t)
	var checked []string
	orig := checkPassword
	checkPassword = func(hash, password string) bool {
		checked = append(checked, hash)
		return orig(hash, password)
	}
	t.Cleanup(func() { checkPassword = orig })

	expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"niemand","password":"admin123"}`, http.StatusUnauthorized)
	if len(checked) != 1 || checked[0] != auth.DummyHash() ||
		!strings.HasPrefix(checked[0], "$argon2id$") || auth.NeedsRehash(checked[0]) {
		t.Fatalf("unbekannter Benutzer: geprüfte Hashes %q", checked)
	}
	// Gegenprobe: vorhandener Benutzer, falsches Passwort – genau ein Vergleich mit seinem Hash.
	checked = nil
	expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`, http.StatusUnauthorized)
	if len(checked) != 1 || checked[0] == auth.DummyHash() || !strings.HasPrefix(checked[0], "$argon2id$") {
		t.Fatalf("vorhandener Benutzer: geprüfte Hashes %q", checked)
	}
}
