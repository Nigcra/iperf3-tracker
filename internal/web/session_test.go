package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/scheduler"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/trace"
)

// browser ist ein HTTP-Client mit Cookie-Speicher, wie ihn die Oberfläche nutzt.
type browser struct {
	t    *testing.T
	srv  *httptest.Server
	c    *http.Client
	csrf string
}

func newBrowser(t *testing.T, srv *httptest.Server) *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{t: t, srv: srv, c: &http.Client{Jar: jar}}
}

func (b *browser) do(method, path, body string, header map[string]string, out any) (int, http.Header) {
	b.t.Helper()
	req, err := http.NewRequest(method, b.srv.URL+path, strings.NewReader(body))
	if err != nil {
		b.t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := b.c.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode != http.StatusNoContent {
		json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, resp.Header
}

func (b *browser) login(user, pass string) map[string]any {
	b.t.Helper()
	var out map[string]any
	code, hdr := b.do("POST", "/api/auth/login", `{"username":"`+user+`","password":"`+pass+`"}`, nil, &out)
	if code != http.StatusOK {
		b.t.Fatalf("Login: %d %v", code, out)
	}
	sc := hdr.Get("Set-Cookie")
	if !strings.Contains(sc, sessionCookie+"=") || !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "SameSite=Lax") || strings.Contains(sc, "Secure") {
		b.t.Errorf("Set-Cookie: %s", sc)
	}
	b.csrf, _ = out["csrf_token"].(string)
	if b.csrf == "" {
		b.t.Fatal("csrf_token fehlt")
	}
	return out
}

func TestCookieSessionAndCSRF(t *testing.T) {
	srv := newTestServer(t)
	b := newBrowser(t, srv)
	b.login("admin", "admin123")

	// Lesen mit Cookie genügt; /auth/me liefert das CSRF-Token im Header.
	var me map[string]any
	code, hdr := b.do("GET", "/api/auth/me", "", nil, &me)
	if code != http.StatusOK || me["username"] != "admin" || hdr.Get(csrfHeader) != b.csrf {
		t.Fatalf("/me: %d %v csrf=%q", code, me, hdr.Get(csrfHeader))
	}
	// Ändernde Anfrage ohne bzw. mit falschem CSRF-Token: 403.
	body := `{"name":"S1","host":"h1"}`
	if code, _ := b.do("POST", "/api/servers", body, nil, nil); code != http.StatusForbidden {
		t.Errorf("ohne CSRF: %d", code)
	}
	if code, _ := b.do("POST", "/api/servers", body, map[string]string{csrfHeader: "falsch"}, nil); code != http.StatusForbidden {
		t.Errorf("falsches CSRF: %d", code)
	}
	if code, _ := b.do("POST", "/api/servers", body, map[string]string{csrfHeader: b.csrf}, nil); code != http.StatusCreated && code != http.StatusOK {
		t.Errorf("mit CSRF: %d", code)
	}
	// Fremde Herkunft wird abgelehnt – auch beim Login.
	if code, _ := b.do("POST", "/api/servers", `{"name":"S2","host":"h2"}`, map[string]string{csrfHeader: b.csrf, "Sec-Fetch-Site": "cross-site"}, nil); code != http.StatusForbidden {
		t.Errorf("cross-site: %d", code)
	}
	if code, _ := b.do("POST", "/api/auth/login", `{"username":"admin","password":"admin123"}`, map[string]string{"Origin": "https://evil.example"}, nil); code != http.StatusForbidden {
		t.Errorf("Login von fremder Origin: %d", code)
	}

	// Abmelden löscht das Cookie.
	if code, _ := b.do("POST", "/api/auth/logout", "", nil, nil); code != http.StatusNoContent {
		t.Errorf("logout: %d", code)
	}
	if code, _ := b.do("GET", "/api/auth/me", "", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("nach logout: %d", code)
	}
}

func TestSecureCookieBehindTLSProxy(t *testing.T) {
	srv := newTestServer(t)
	b := newBrowser(t, srv)
	_, hdr := b.do("POST", "/api/auth/login", `{"username":"admin","password":"admin123"}`, map[string]string{"X-Forwarded-Proto": "https"}, nil)
	if sc := hdr.Get("Set-Cookie"); !strings.HasPrefix(sc, sessionCookieSecure+"=") || !strings.Contains(sc, "Secure") {
		t.Errorf("Set-Cookie unter TLS: %s", sc)
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self'", "style-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP ohne %q: %s", want, csp)
		}
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "http") {
		t.Errorf("CSP zu offen: %s", csp)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff fehlt")
	}
}

// newServerWithStore startet einen Server auf einer leeren Datenbank.
func newServerWithStore(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	st := store.New(conn)
	runner := iperf.NewRunner(st, filepath.Join(t.TempDir(), "kein-iperf3"))
	t.Cleanup(runner.Stop)
	tracer := trace.NewTracer(nil, filepath.Join(t.TempDir(), "kein-traceroute"))
	srv := httptest.NewServer(NewServer(Deps{
		Store: st, Tokens: auth.NewTokens("test-secret"), Runner: runner, Tracer: tracer,
		Scheduler: scheduler.New(st, runner, tracer),
	}).Handler())
	t.Cleanup(srv.Close)
	return srv, st
}

func TestFirstAdminMustChangePassword(t *testing.T) {
	srv, st := newServerWithStore(t)
	var password string
	InitialPasswordHook = func(_, p string) { password = p }
	t.Cleanup(func() { InitialPasswordHook = nil })

	created, err := EnsureAdmin(context.Background(), st)
	if err != nil || !created || len(password) < 16 {
		t.Fatalf("EnsureAdmin: %v %v %q", created, err, password)
	}
	if again, _ := EnsureAdmin(context.Background(), st); again {
		t.Error("zweiter Admin angelegt")
	}
	expectStatus(t, srv, "POST", "/api/auth/login", "", `{"username":"admin","password":"admin123"}`, http.StatusUnauthorized)

	tok := login(t, srv, "admin", password)
	var me map[string]any
	if call(t, srv, "GET", "/api/auth/me", tok, "", &me); me["must_change_password"] != true {
		t.Fatalf("/me: %v", me)
	}
	// Alles außer /me und Passwortwechsel ist gesperrt.
	expectStatus(t, srv, "GET", "/api/servers", tok, "", http.StatusForbidden)
	expectStatus(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"`+password+`","new_password":"`+password+`"}`, http.StatusUnprocessableEntity)
	expectStatus(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"`+password+`","new_password":"neuesPasswort"}`, http.StatusOK)
	expectStatus(t, srv, "GET", "/api/servers", tok, "", http.StatusOK)
	expectStatus(t, srv, "POST", "/api/auth/init-admin", "", "", http.StatusBadRequest)
}

func TestInitAdminDoesNotRevealPassword(t *testing.T) {
	srv, _ := newServerWithStore(t)
	var out map[string]any
	if code := call(t, srv, "POST", "/api/auth/init-admin", "", "", &out); code != http.StatusOK {
		t.Fatalf("init-admin: %d %v", code, out)
	}
	if _, ok := out["password"]; ok {
		t.Errorf("Passwort in der Antwort: %v", out)
	}
}

func TestLegacyBcryptRehashAndDefaultPassword(t *testing.T) {
	srv, st := newServerWithStore(t)
	ctx := context.Background()
	h, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, &model.User{Username: "admin", Email: "a@x", HashedPassword: string(h), IsActive: true, IsAdmin: true}); err != nil {
		t.Fatal(err)
	}
	if err := FlagLegacyDefaultPassword(ctx, st); err != nil {
		t.Fatal(err)
	}
	tok := login(t, srv, "admin", "admin123")
	u, _ := st.UserByUsername(ctx, "admin")
	if !strings.HasPrefix(u.HashedPassword, "$argon2id$") || !u.MustChangePassword {
		t.Errorf("nach Login: hash=%.12s must=%v", u.HashedPassword, u.MustChangePassword)
	}
	expectStatus(t, srv, "GET", "/api/tests", tok, "", http.StatusForbidden)
	// Umgehashtes Passwort funktioniert weiter.
	login(t, srv, "admin", "admin123")
}

func TestLiveTestsStream(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/tests/live/stream", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type %q", resp.Header.Get("Content-Type"))
	}
	sc := bufio.NewScanner(resp.Body)
	next := func() map[string]any {
		t.Helper()
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				var ev map[string]any
				if err := json.Unmarshal([]byte(data), &ev); err != nil {
					t.Fatal(err)
				}
				return ev
			}
		}
		t.Fatalf("Strom beendet: %v", sc.Err())
		return nil
	}
	if ev := next(); ev["type"] != "live" || len(ev["tests"].([]any)) != 0 {
		t.Fatalf("erstes Event: %v", ev)
	}

	// Ein Test erscheint im Strom (er scheitert mangels iperf3 sofort und
	// bleibt mit Status failed noch kurz sichtbar).
	var sv map[string]any
	call(t, srv, "POST", "/api/servers", tok, `{"name":"S","host":"127.0.0.1"}`, &sv)
	var test map[string]any
	if code := call(t, srv, "POST", "/api/tests/run", tok, `{"server_id":`+itoa(int64(sv["id"].(float64)))+`}`, &test); code != http.StatusCreated {
		t.Fatalf("run: %d %v", code, test)
	}
	for {
		ev := next()
		tests := ev["tests"].([]any)
		if len(tests) == 0 {
			continue
		}
		lt := tests[0].(map[string]any)
		if lt["test_id"] != test["id"] || lt["server_name"] != "S" {
			t.Fatalf("Eintrag: %v", lt)
		}
		if lt["status"] == "failed" {
			break
		}
	}
}
