package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestLocalizedErrors prüft, dass Meldungen der API der Sprache der Anfrage folgen.
func TestLocalizedErrors(t *testing.T) {
	srv := newTestServer(t)
	tok := login(t, srv, "admin", "admin123")

	get := func(method, path, token, body, lang string) (int, map[string]any, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if lang != "" {
			req.Header.Set("Accept-Language", lang)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out, res.Header
	}

	cases := []struct {
		method, path, token, body, lang, want string
	}{
		{"POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`, "", "Benutzername oder Passwort falsch"},
		{"POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`, "en-US,en;q=0.9", "Wrong username or password"},
		{"POST", "/api/auth/login", "", `{"username":"admin","password":"falsch"}`, "de-DE", "Benutzername oder Passwort falsch"},
		{"GET", "/api/servers/99", tok, "", "en", "Server not found"},
		{"GET", "/api/tests?limit=0", tok, "", "en", `Parameter "limit" must be a number between 1 and 1000`},
		{"POST", "/api/servers", tok, `{"name":"x","host":"h","port":0}`, "en", "Port must be between 1 and 65535"},
	}
	for _, c := range cases {
		_, out, hdr := get(c.method, c.path, c.token, c.body, c.lang)
		if out["detail"] != c.want {
			t.Errorf("%s %s [%s]: detail %q, want %q", c.method, c.path, c.lang, out["detail"], c.want)
		}
		if want := map[bool]string{true: "en", false: "de"}[strings.HasPrefix(c.lang, "en")]; hdr.Get("Content-Language") != want {
			t.Errorf("%s %s [%s]: Content-Language %q", c.method, c.path, c.lang, hdr.Get("Content-Language"))
		}
	}

	// Ort und Beschreibung öffentlicher Server; Nutzerdaten bleiben unverändert.
	req, _ := http.NewRequest("GET", srv.URL+"/api/public-servers", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept-Language", "en")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var public []map[string]any
	json.NewDecoder(res.Body).Decode(&public)
	for _, p := range public {
		if loc := p["location"].(string); strings.Contains(loc, "Deutschland") || strings.Contains(loc, "Frankreich") {
			t.Errorf("location not translated: %q", loc)
		}
		if desc := p["description"].(string); strings.HasPrefix(desc, "Öffentlich") {
			t.Errorf("description not translated: %q", desc)
		}
	}
}
