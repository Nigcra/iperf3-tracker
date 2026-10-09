package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListenDefaults(t *testing.T) {
	// Ohne web.listen: nur lokal.
	cfg, err := Load(writeConfig(t, "log:\n  level: info\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Listen != DefaultListen || DefaultListen != "127.0.0.1:8000" {
		t.Errorf("Listen = %q", cfg.Web.Listen)
	}
	// Die beim ersten Start erzeugte Datei bleibt beim bisherigen Verhalten.
	created, err := CreateDefault(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if created.Web.Listen != "0.0.0.0:8000" || created.Auth.SecretKey == "" || created.Auth.SecretGenerated {
		t.Errorf("CreateDefault: %+v", created.Web)
	}
	if created.Log.Format != "text" || created.Map.TileURL != "" {
		t.Errorf("CreateDefault: log %+v, map %+v", created.Log, created.Map)
	}
}

func TestEnvOverrides(t *testing.T) {
	env := map[string]string{
		"IPERF3_WEB_LISTEN":        "0.0.0.0:9000",
		"IPERF3_SCHEDULER_ENABLED": "false",
		"IPERF3_LOG_FORMAT":        "json",
		"IPERF3_MAP_TILE_URL":      "https://tiles.example.org/{z}/{x}/{y}.png",
		// Veraltet: gilt, solange der neue Name fehlt …
		"DB_PATH": "/data/x.db",
		// … und wird ignoriert, wenn er gesetzt ist.
		"LOG_LEVEL":        "debug",
		"IPERF3_LOG_LEVEL": "warn",
	}
	c := defaults()
	if err := c.applyEnv(func(k string) (string, bool) { v, ok := env[k]; return v, ok }); err != nil {
		t.Fatal(err)
	}
	if c.Web.Listen != "0.0.0.0:9000" || c.Scheduler.Enabled || c.Log.Format != "json" || c.Storage.Path != "/data/x.db" || c.Log.Level != "warn" {
		t.Errorf("Ergebnis: %+v", c)
	}
	if c.Map.TileURL == "" || c.Map.TileOrigin() != "https://tiles.example.org" {
		t.Errorf("TileOrigin = %q", c.Map.TileOrigin())
	}
	if len(c.Warnings) != 2 || !strings.Contains(strings.Join(c.Warnings, "\n"), "IPERF3_STORAGE_PATH") {
		t.Errorf("Warnungen: %q", c.Warnings)
	}

	bad := defaults()
	if err := bad.applyEnv(func(k string) (string, bool) {
		return "vielleicht", k == "IPERF3_SCHEDULER_ENABLED"
	}); err == nil {
		t.Error("ungültiger bool wurde akzeptiert")
	}
}

func TestEnvKeys(t *testing.T) {
	keys := strings.Join(EnvKeys(), ",")
	for _, want := range []string{"IPERF3_WEB_LISTEN", "IPERF3_STORAGE_PATH", "IPERF3_AUTH_SECRET_KEY", "IPERF3_IPERF_AUTO_INSTALL", "IPERF3_MAP_TILE_URL", "IPERF3_LOG_FORMAT", "IPERF3_AUTH_SESSION_TTL"} {
		if !strings.Contains(keys, want) {
			t.Errorf("%s fehlt in %s", want, keys)
		}
	}
}

func TestValidate(t *testing.T) {
	for _, content := range []string{
		"log:\n  format: xml\n",
		"map:\n  tile_url: \"ftp://x/{z}/{x}/{y}\"\n",
		"map:\n  tile_url: \"https://x/tiles.png\"\n",
	} {
		if _, err := Load(writeConfig(t, content)); err == nil {
			t.Errorf("%q: kein Fehler", content)
		}
	}
	cfg, err := Load(writeConfig(t, "map:\n  tile_url: \"https://{s}.tile.example.org/{z}/{x}/{y}.png\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Map.TileOrigin(); got != "https://*.tile.example.org" {
		t.Errorf("TileOrigin = %q", got)
	}
}

func TestSessionTTL(t *testing.T) {
	cfg, err := Load(writeConfig(t, "log:\n  level: info\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d, err := cfg.Auth.SessionDuration(); err != nil || d != 12*time.Hour {
		t.Errorf("Standard: %v %v", d, err)
	}
	created, err := CreateDefault(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil || created.Auth.SessionTTL != "12h" {
		t.Errorf("CreateDefault: %q %v", created.Auth.SessionTTL, err)
	}
	cfg, err = Load(writeConfig(t, "auth:\n  session_ttl: \"30m\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := cfg.Auth.SessionDuration(); d != 30*time.Minute {
		t.Errorf("aus Datei: %v", d)
	}
	c := defaults()
	if err := c.applyEnv(func(k string) (string, bool) { return "8h", k == "IPERF3_AUTH_SESSION_TTL" }); err != nil {
		t.Fatal(err)
	}
	if d, _ := c.Auth.SessionDuration(); d != 8*time.Hour {
		t.Errorf("aus Umgebung: %v", d)
	}
	for _, bad := range []string{"zwölf", "30s", "-1h"} {
		if _, err := Load(writeConfig(t, "auth:\n  session_ttl: \""+bad+"\"\n")); err == nil {
			t.Errorf("session_ttl %q akzeptiert", bad)
		}
	}
}
