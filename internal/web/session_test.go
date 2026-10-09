package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

	// Abmelden löscht das Cookie und sperrt das Token: Auch ein zuvor
	// kopiertes Cookie gilt danach nicht mehr.
	stolen := b.cookie()
	if code, _ := b.do("POST", "/api/auth/logout", "", nil, nil); code != http.StatusNoContent {
		t.Errorf("logout: %d", code)
	}
	if code, _ := b.do("GET", "/api/auth/me", "", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("nach logout: %d", code)
	}
	if code, _ := cookieRequest(t, srv, "GET", "/api/auth/me", stolen); code != http.StatusUnauthorized {
		t.Errorf("kopiertes Cookie nach logout: %d", code)
	}
}

// cookie liefert den Wert des Sitzungs-Cookies im Cookie-Speicher.
func (b *browser) cookie() string {
	b.t.Helper()
	u, _ := url.Parse(b.srv.URL)
	for _, c := range b.c.Jar.Cookies(u) {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	b.t.Fatal("kein Sitzungs-Cookie")
	return ""
}

// cookieRequest schickt eine Anfrage mit dem Sitzungs-Cookie token (ohne
// Cookie-Speicher) und liefert Status und Antwort-Header.
func cookieRequest(t *testing.T, srv *httptest.Server, method, path, token string) (int, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header
}

// signToken erstellt ein Sitzungs-Token wie der Server, aber mit frei
// wählbaren Zeitpunkten (für Verlängerung und Ablauf).
func signToken(t *testing.T, user, sessionID string, iat, exp time.Time) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user, "ver": 0, "jti": sessionID, "iat": iat.Unix(), "exp": exp.Unix(),
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// TestSlidingSession: Ist mehr als die Hälfte der Laufzeit verstrichen, stellt
// der Server bei Aktivität ein neues Cookie mit voller Laufzeit aus – mit
// derselben Sitzung, also gleichem CSRF-Token. Frische Cookies und
// Bearer-Tokens werden nicht verlängert, abgelaufene abgewiesen.
func TestSlidingSession(t *testing.T) {
	srv := newTestServer(t)
	ttl := auth.DefaultSessionTTL
	now := time.Now()
	old := signToken(t, "admin", "sitzung-alt", now.Add(-ttl/2-time.Minute), now.Add(ttl/2-time.Minute))

	code, hdr := cookieRequest(t, srv, "GET", "/api/auth/me", old)
	if code != http.StatusOK {
		t.Fatalf("/me mit älterem Cookie: %d", code)
	}
	var renewed *http.Cookie
	for _, c := range (&http.Response{Header: hdr}).Cookies() {
		if c.Name == sessionCookie {
			renewed = c
		}
	}
	if renewed == nil || renewed.Value == old || renewed.MaxAge != int(ttl/time.Second) || !renewed.HttpOnly {
		t.Fatalf("keine Verlängerung: %v", hdr.Values("Set-Cookie"))
	}
	claims, err := auth.NewTokens(testSecret, 0).Parse(renewed.Value)
	if err != nil || claims.SessionID != "sitzung-alt" || claims.ExpiresAt.Before(now.Add(ttl-time.Minute)) {
		t.Fatalf("verlängertes Token: %+v %v", claims, err)
	}
	csrf := auth.NewTokens(testSecret, 0).CSRF("sitzung-alt")
	if hdr.Get(csrfHeader) != csrf {
		t.Errorf("CSRF-Token nach Verlängerung geändert: %q", hdr.Get(csrfHeader))
	}
	// Das verlängerte Cookie funktioniert; da es frisch ist, wird es nicht erneut verlängert.
	if code, hdr := cookieRequest(t, srv, "GET", "/api/auth/me", renewed.Value); code != http.StatusOK || hdr.Get("Set-Cookie") != "" {
		t.Errorf("frisches Cookie: %d, Set-Cookie %q", code, hdr.Get("Set-Cookie"))
	}
	// Bearer-Tokens behalten ihre feste Laufzeit.
	req, _ := http.NewRequest("GET", srv.URL+"/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+old)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Set-Cookie") != "" {
		t.Errorf("Bearer: %d, Set-Cookie %q", resp.StatusCode, resp.Header.Get("Set-Cookie"))
	}
	// Ohne Aktivität innerhalb der Laufzeit ist die Sitzung beendet.
	expired := signToken(t, "admin", "sitzung-abgelaufen", now.Add(-ttl-time.Minute), now.Add(-time.Minute))
	if code, _ := cookieRequest(t, srv, "GET", "/api/auth/me", expired); code != http.StatusUnauthorized {
		t.Errorf("abgelaufenes Cookie: %d", code)
	}
	// Tokens früherer Versionen ohne Sitzungskennung gelten nicht mehr.
	if code, _ := cookieRequest(t, srv, "GET", "/api/auth/me", signToken(t, "admin", "", now, now.Add(time.Hour))); code != http.StatusUnauthorized {
		t.Errorf("Token ohne jti: %d", code)
	}
}

// TestCookiePasswordChangeKeepsSession: Im Browser läuft die Sitzung nach dem
// Passwortwechsel mit neuem Cookie und unverändertem CSRF-Token weiter; ein
// zuvor kopiertes Cookie ist beendet.
func TestCookiePasswordChangeKeepsSession(t *testing.T) {
	srv := newTestServer(t)
	b := newBrowser(t, srv)
	b.login("admin", "admin123")
	before := b.cookie()
	code, _ := b.do("POST", "/api/auth/change-password", `{"current_password":"admin123","new_password":"neuesPasswort"}`, map[string]string{csrfHeader: b.csrf}, nil)
	if code != http.StatusOK {
		t.Fatalf("change-password: %d", code)
	}
	if b.cookie() == before {
		t.Fatal("Cookie nicht erneuert")
	}
	if code, _ := b.do("POST", "/api/servers", `{"name":"S1","host":"h1"}`, map[string]string{csrfHeader: b.csrf}, nil); code != http.StatusCreated && code != http.StatusOK {
		t.Errorf("nach Passwortwechsel mit bisherigem CSRF-Token: %d", code)
	}
	if code, _ := cookieRequest(t, srv, "GET", "/api/auth/me", before); code != http.StatusUnauthorized {
		t.Errorf("altes Cookie nach Passwortwechsel: %d", code)
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
		Store: st, Tokens: auth.NewTokens(testSecret, 0), Runner: runner, Tracer: tracer,
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
	var changed map[string]any
	if code := call(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"`+password+`","new_password":"neuesPasswort"}`, &changed); code != http.StatusOK {
		t.Fatalf("change-password: %d %v", code, changed)
	}
	// Das Token mit dem Startpasswort ist beendet, die Sitzung läuft mit dem neuen weiter.
	expectStatus(t, srv, "GET", "/api/servers", tok, "", http.StatusUnauthorized)
	expectStatus(t, srv, "GET", "/api/servers", changed["access_token"].(string), "", http.StatusOK)
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

// TestAuthStatusInitialPending: /api/auth/status ist ohne Anmeldung erreichbar
// und meldet initial_pending nur, solange der Admin das beim ersten Start
// erzeugte Passwort noch nicht geändert hat – danach nie wieder.
func TestAuthStatusInitialPending(t *testing.T) {
	status := func(srv *httptest.Server) map[string]any {
		t.Helper()
		var out map[string]any
		if code := call(t, srv, "GET", "/api/auth/status", "", "", &out); code != http.StatusOK {
			t.Fatalf("/api/auth/status: Status %d, %v", code, out)
		}
		if out["auth_enabled"] != true || len(out) != 2 {
			t.Fatalf("/api/auth/status: %v", out)
		}
		return out
	}
	ctx := context.Background()

	srv, st := newServerWithStore(t)
	if got := status(srv)["initial_pending"]; got != false {
		t.Errorf("ohne Benutzer: initial_pending = %v", got)
	}
	var password string
	InitialPasswordHook = func(_, p string) { password = p }
	t.Cleanup(func() { InitialPasswordHook = nil })
	if created, err := EnsureAdmin(ctx, st); err != nil || !created {
		t.Fatalf("EnsureAdmin: %v %v", created, err)
	}
	if got := status(srv)["initial_pending"]; got != true {
		t.Errorf("nach dem ersten Start: initial_pending = %v", got)
	}
	// Anmelden allein ändert nichts, erst der Passwortwechsel.
	tok := login(t, srv, "admin", password)
	if got := status(srv)["initial_pending"]; got != true {
		t.Errorf("nach der Anmeldung: initial_pending = %v", got)
	}
	expectStatus(t, srv, "POST", "/api/auth/change-password", tok, `{"current_password":"`+password+`","new_password":"neuesPasswort"}`, http.StatusOK)
	if got := status(srv)["initial_pending"]; got != false {
		t.Errorf("nach dem Passwortwechsel: initial_pending = %v", got)
	}
	// Auch nach einem Neustart (FlagLegacyDefaultPassword) bleibt der Hinweis aus.
	if err := FlagLegacyDefaultPassword(ctx, st); err != nil {
		t.Fatal(err)
	}
	if got := status(srv)["initial_pending"]; got != false {
		t.Errorf("nach Neustart: initial_pending = %v", got)
	}

	// Früheres Standardpasswort: Wechsel erzwungen, aber kein erzeugtes Passwort.
	legacySrv, legacySt := newServerWithStore(t)
	h, _ := bcrypt.GenerateFromPassword([]byte(auth.LegacyDefaultPassword), bcrypt.MinCost)
	if err := legacySt.CreateUser(ctx, &model.User{Username: "admin", Email: "a@x", HashedPassword: string(h), IsActive: true, IsAdmin: true}); err != nil {
		t.Fatal(err)
	}
	if err := FlagLegacyDefaultPassword(ctx, legacySt); err != nil {
		t.Fatal(err)
	}
	if got := status(legacySrv)["initial_pending"]; got != false {
		t.Errorf("früheres Standardpasswort: initial_pending = %v", got)
	}
	// Nach dem Umhashen auf Argon2id bleibt es dabei.
	login(t, legacySrv, "admin", auth.LegacyDefaultPassword)
	if got := status(legacySrv)["initial_pending"]; got != false {
		t.Errorf("früheres Standardpasswort nach Umhashen: initial_pending = %v", got)
	}

	// Vorhandener Admin ohne Wechselzwang.
	if got := status(newTestServer(t))["initial_pending"]; got != false {
		t.Errorf("bestehender Admin: initial_pending = %v", got)
	}
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
