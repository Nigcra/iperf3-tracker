// Package config lädt die Anwendungskonfiguration aus config.yaml. Einzelne
// Werte lassen sich per Umgebungsvariable überschreiben (für Docker).
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config bündelt die gesamte Anwendungskonfiguration.
type Config struct {
	Web       WebConfig       `yaml:"web"`
	Storage   StorageConfig   `yaml:"storage"`
	Auth      AuthConfig      `yaml:"auth"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Log       LogConfig       `yaml:"log"`
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
	// SecretKey signiert die Login-Tokens (JWT, HS256).
	SecretKey string `yaml:"secret_key"`
	// SecretGenerated ist true, wenn kein Schlüssel konfiguriert war und für
	// diese Laufzeit ein zufälliger erzeugt wurde.
	SecretGenerated bool `yaml:"-"`
}

// SchedulerConfig steuert die zeitgesteuerten Tests.
type SchedulerConfig struct {
	Enabled bool `yaml:"enabled"`
}

// LogConfig steuert die Protokollierung.
type LogConfig struct {
	Level string `yaml:"level"`
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
const defaultYAML = `# iperf3-Tracker – Konfiguration
# Einzelne Werte lassen sich per Umgebungsvariable überschreiben:
#   LISTEN_ADDR, DB_PATH, SECRET_KEY, SCHEDULER_ENABLED, LOG_LEVEL

web:
  listen: "0.0.0.0:8000"

storage:
  path: "data/iperf3-tracker.db"

auth:
  # Schlüssel zum Signieren der Login-Tokens. Beim ersten Start zufällig
  # erzeugt; eine Änderung macht alle bestehenden Anmeldungen ungültig.
  secret_key: "%s"

scheduler:
  enabled: true

log:
  level: "info"
`

// CreateDefault schreibt eine Standard-config.yaml mit frisch erzeugtem
// Token-Schlüssel nach path und gibt die geparste Konfiguration zurück.
func CreateDefault(path string) (*Config, error) {
	content := fmt.Sprintf(defaultYAML, newSecret())
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
	if err := cfg.applyEnv(); err != nil {
		return nil, err
	}

	// Relative Pfade relativ zum Verzeichnis der Config-Datei auflösen.
	if !filepath.IsAbs(cfg.Storage.Path) {
		cfg.Storage.Path = filepath.Join(filepath.Dir(filepath.Clean(path)), cfg.Storage.Path)
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
		Web:       WebConfig{Listen: "0.0.0.0:8000"},
		Storage:   StorageConfig{Path: "data/iperf3-tracker.db"},
		Scheduler: SchedulerConfig{Enabled: true},
		Log:       LogConfig{Level: "info"},
	}
}

func (c *Config) applyEnv() error {
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		c.Web.Listen = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		c.Storage.Path = v
	}
	if v := os.Getenv("SECRET_KEY"); v != "" {
		c.Auth.SecretKey = v
	}
	if v := os.Getenv("SCHEDULER_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("SCHEDULER_ENABLED ist ungültig: %q", v)
		}
		c.Scheduler.Enabled = b
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.Log.Level = v
	}
	return nil
}

func (c *Config) validate() error {
	if c.Web.Listen == "" {
		return fmt.Errorf("web.listen darf nicht leer sein")
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(c.Log.Level)); err != nil {
		return fmt.Errorf("log.level ist ungültig: %q", c.Log.Level)
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
