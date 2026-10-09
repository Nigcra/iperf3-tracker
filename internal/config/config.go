// Package config lädt die Anwendungskonfiguration aus config.yaml. Jeder
// Schlüssel lässt sich per Umgebungsvariable IPERF3_<ABSCHNITT>_<SCHLÜSSEL>
// überschreiben (z. B. IPERF3_WEB_LISTEN, IPERF3_STORAGE_PATH); die früheren
// Namen ohne Präfix gelten weiter als veraltete Aliase (siehe env.go).
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultListen ist die Adresse, wenn web.listen weder in der Datei noch per
// Umgebung gesetzt ist: nur lokal erreichbar. Die beim ersten Start erzeugte
// config.yaml und das Docker-Image setzen ausdrücklich 0.0.0.0:8000.
const DefaultListen = "127.0.0.1:8000"

// Config bündelt die gesamte Anwendungskonfiguration.
type Config struct {
	Web       WebConfig       `yaml:"web"`
	Storage   StorageConfig   `yaml:"storage"`
	Auth      AuthConfig      `yaml:"auth"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Iperf     IperfConfig     `yaml:"iperf"`
	GeoIP     GeoIPConfig     `yaml:"geoip"`
	Map       MapConfig       `yaml:"map"`
	Log       LogConfig       `yaml:"log"`

	// Dir ist das Verzeichnis der Config-Datei; relative Pfade beziehen sich darauf.
	Dir string `yaml:"-"`
	// Warnings sammelt Hinweise beim Laden (z. B. veraltete Umgebungsvariablen),
	// die erst nach dem Einrichten des Loggers ausgegeben werden können.
	Warnings []string `yaml:"-"`
}

// WebConfig konfiguriert den HTTP-Server.
type WebConfig struct {
	Listen string `yaml:"listen"`
}

// StorageConfig konfiguriert die SQLite-Datenbank.
type StorageConfig struct {
	Path string `yaml:"path"`
}

// AuthConfig konfiguriert die Anmeldung.
type AuthConfig struct {
	// SecretKey signiert die Sitzungen (JWT, HS256) und leitet die CSRF-Tokens ab.
	SecretKey string `yaml:"secret_key"`
	// SecretGenerated ist true, wenn kein Schlüssel konfiguriert war und für
	// diese Laufzeit ein zufälliger erzeugt wurde.
	SecretGenerated bool `yaml:"-"`
}

// SchedulerConfig steuert die zeitgesteuerten Tests.
type SchedulerConfig struct {
	Enabled bool `yaml:"enabled"`
}

// IperfConfig konfiguriert die iperf3-Binary.
type IperfConfig struct {
	// Path zur iperf3-Binary; leer = automatisch suchen (PATH, unter Windows
	// zusätzlich übliche Installationsorte).
	Path string `yaml:"path"`
	// AutoInstall installiert iperf3 beim Start ohne Nachfrage über den
	// Paketmanager, falls es fehlt. Standard ist aus: dann fragt die Oberfläche
	// einen Admin, bevor installiert wird (sinnvoll ohne Oberfläche, z. B. Docker).
	AutoInstall bool `yaml:"auto_install"`
}

// GeoIPConfig konfiguriert die GeoLite2-City-Datenbank für Trace-Standorte.
type GeoIPConfig struct {
	Path string `yaml:"path"`
}

// MapConfig konfiguriert den Kartenhintergrund der Peering-Map.
type MapConfig struct {
	// TileURL ist eine Kachel-URL mit {z}, {x} und {y} (z. B. eines eigenen
	// Kachelservers). Leer = eingebettete Weltkarte (Natural Earth), ohne
	// jeden Abruf von fremden Servern.
	TileURL string `yaml:"tile_url"`
	// TileAttribution ist der Quellenhinweis für die Kacheln (HTML erlaubt).
	// Leer = OpenStreetMap-Hinweis.
	TileAttribution string `yaml:"tile_attribution"`
}

// tilePlaceholders ersetzt die Leaflet-Platzhalter für die URL-Prüfung.
var tilePlaceholders = strings.NewReplacer("{s}", "a", "{z}", "0", "{x}", "0", "{y}", "0", "{r}", "")

// TileOrigin liefert Schema und Host der Kachel-URL (für die CSP) oder "".
func (m MapConfig) TileOrigin() string {
	if m.TileURL == "" {
		return ""
	}
	u, err := url.Parse(tilePlaceholders.Replace(m.TileURL))
	if err != nil || u.Host == "" {
		return ""
	}
	// Platzhalter für Subdomains ({s}.tile.example.org) als Wildcard erlauben.
	host := u.Host
	if strings.HasPrefix(m.TileURL, u.Scheme+"://{s}.") {
		host = "*." + strings.TrimPrefix(host, "a.")
	}
	return u.Scheme + "://" + host
}

// LogConfig steuert die Protokollierung.
type LogConfig struct {
	// Level: debug, info, warn oder error.
	Level string `yaml:"level"`
	// Format: text (Standard) oder json.
	Format string `yaml:"format"`
}

// SlogLevel liefert das konfigurierte Log-Level (debug, info, warn, error).
func (l LogConfig) SlogLevel() slog.Level {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(l.Level)); err != nil {
		return slog.LevelInfo
	}
	return lvl
}

// defaultYAML ist der Inhalt einer neu erzeugten config.yaml; %s ist der
// zufällig erzeugte Token-Schlüssel.
const defaultYAML = `# iperf3-Tracker – configuration
# Every key can be overridden with an environment variable
# IPERF3_<SECTION>_<KEY>, e.g. IPERF3_WEB_LISTEN, IPERF3_STORAGE_PATH,
# IPERF3_AUTH_SECRET_KEY, IPERF3_SCHEDULER_ENABLED, IPERF3_LOG_LEVEL.

web:
  # 0.0.0.0 = reachable from the network. Without this key the service only
  # listens on 127.0.0.1:8000.
  listen: "0.0.0.0:8000"

storage:
  path: "data/iperf3-tracker.db"

auth:
  # Key used to sign login sessions. Generated randomly on first start;
  # changing it invalidates all existing logins.
  secret_key: "%s"

scheduler:
  enabled: true

iperf:
  # Path to the iperf3 binary (version 3.17 or newer); empty = auto-detect.
  path: ""
  # Install iperf3 on startup without asking if it is missing (winget or package manager).
  # Default: off – the web interface asks an administrator before installing.
  auto_install: false

geoip:
  # GeoLite2-City database for the locations on the map (relative to this file).
  path: "geoip/GeoLite2-City.mmdb"

map:
  # Empty = embedded offline world map (Natural Earth), no external requests.
  # Optional own tile server, e.g. "https://tiles.example.org/{z}/{x}/{y}.png"
  tile_url: ""
  # Attribution shown for the tiles (HTML allowed); empty = OpenStreetMap notice.
  tile_attribution: ""

log:
  # debug | info | warn | error
  level: "info"
  # text | json
  format: "text"
`

// CreateDefault schreibt eine Standard-config.yaml mit frisch erzeugtem
// Token-Schlüssel nach path und gibt die geparste Konfiguration zurück.
func CreateDefault(path string) (*Config, error) {
	content := fmt.Sprintf(defaultYAML, newSecret())
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("Standard-Konfiguration konnte nicht erstellt werden (%s): %w", path, err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return nil, fmt.Errorf("Standard-Konfiguration konnte nicht erstellt werden (%s): %w", path, err)
	}
	return Load(path)
}

// Load lädt die Konfiguration aus einer YAML-Datei und wendet anschließend
// die Umgebungsvariablen an.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Konfiguration konnte nicht gelesen werden: %w", err)
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("Konfiguration ist ungültig: %w", err)
	}
	if err := cfg.applyEnv(os.LookupEnv); err != nil {
		return nil, err
	}

	// Relative Pfade relativ zum Verzeichnis der Config-Datei auflösen.
	cfg.Dir = filepath.Dir(filepath.Clean(path))
	for _, p := range []*string{&cfg.Storage.Path, &cfg.GeoIP.Path} {
		if !filepath.IsAbs(*p) {
			*p = filepath.Join(cfg.Dir, *p)
		}
	}

	if cfg.Auth.SecretKey == "" {
		cfg.Auth.SecretKey = newSecret()
		cfg.Auth.SecretGenerated = true
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Web:       WebConfig{Listen: DefaultListen},
		Storage:   StorageConfig{Path: "data/iperf3-tracker.db"},
		GeoIP:     GeoIPConfig{Path: "geoip/GeoLite2-City.mmdb"},
		Scheduler: SchedulerConfig{Enabled: true},
		Log:       LogConfig{Level: "info", Format: "text"},
	}
}

func (c *Config) validate() error {
	if c.Web.Listen == "" {
		return fmt.Errorf("web.listen darf nicht leer sein")
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(c.Log.Level)); err != nil {
		return fmt.Errorf("log.level ist ungültig: %q", c.Log.Level)
	}
	switch c.Log.Format {
	case "", "text", "json":
	default:
		return fmt.Errorf("log.format muss text oder json sein: %q", c.Log.Format)
	}
	if c.Map.TileURL != "" {
		u, err := url.Parse(tilePlaceholders.Replace(c.Map.TileURL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") ||
			!strings.Contains(c.Map.TileURL, "{z}") || !strings.Contains(c.Map.TileURL, "{x}") || !strings.Contains(c.Map.TileURL, "{y}") {
			return fmt.Errorf("map.tile_url muss eine http(s)-URL mit {z}, {x} und {y} sein: %q", c.Map.TileURL)
		}
	}
	return nil
}

func newSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("Zufallsgenerator nicht verfügbar: " + err.Error())
	}
	return hex.EncodeToString(b)
}
