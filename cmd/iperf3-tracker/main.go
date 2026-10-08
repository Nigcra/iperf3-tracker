// Command iperf3-tracker führt zeit- und benutzergesteuerte iperf3-Tests aus,
// speichert die Ergebnisse in SQLite und stellt API und Oberfläche über HTTP bereit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/config"
	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/trace"
	"iperf3-tracker/internal/version"
	"iperf3-tracker/internal/web"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "Pfad zur Konfigurationsdatei")
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	// --- Konfiguration laden oder beim ersten Start anlegen ---
	var (
		cfg     *config.Config
		created bool
		err     error
	)
	if _, statErr := os.Stat(cfgPath); os.IsNotExist(statErr) {
		created = true
		cfg, err = config.CreateDefault(cfgPath)
	} else {
		cfg, err = config.Load(cfgPath)
	}
	if err != nil {
		return err
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.Log.SlogLevel()})))
	if created {
		slog.Info("Neue Konfiguration erstellt", "pfad", cfgPath)
	}
	if cfg.Auth.SecretGenerated {
		slog.Warn("auth.secret_key nicht gesetzt – temporärer Schlüssel erzeugt, Anmeldungen werden beim Neustart ungültig")
	}

	// --- Datenbank ---
	conn, err := db.Open(cfg.Storage.Path)
	if err != nil {
		return err
	}
	defer conn.Close()
	st := store.New(conn)
	slog.Info("Datenbank geöffnet", "pfad", cfg.Storage.Path)

	if err := ensureDefaultAdmin(st); err != nil {
		return err
	}
	if n, err := st.FailUnfinishedTests(context.Background(), "Abgebrochen: Dienst wurde neu gestartet"); err != nil {
		return fmt.Errorf("Offene Tests konnten nicht bereinigt werden: %w", err)
	} else if n > 0 {
		slog.Warn("Unterbrochene Tests als fehlgeschlagen markiert", "anzahl", n)
	}

	// --- iperf3 ---
	iperfBin := iperf.FindBinary(cfg.Iperf.Path)
	if v, err := iperf.CheckVersion(iperfBin); err != nil {
		slog.Warn("iperf3 nicht nutzbar – Tests werden fehlschlagen", "pfad", iperfBin, "fehler", err)
	} else {
		slog.Info("iperf3 gefunden", "pfad", iperfBin, "version", v)
	}
	runner := iperf.NewRunner(st, iperfBin)
	defer runner.Stop()

	// --- GeoIP für Trace-Standorte ---
	// backend/geoip ist der bisherige Ablageort (Python-Backend, bis Phase 10).
	geo, geoPath, err := trace.OpenGeoIP(cfg.GeoIP.Path,
		filepath.Join(cfg.Dir, "backend", "geoip", "GeoLite2-City.mmdb"),
		"/var/lib/GeoIP/GeoLite2-City.mmdb")
	if err != nil {
		slog.Warn("GeoIP-Datenbank nicht verfügbar – Traces ohne Standorte", "pfad", cfg.GeoIP.Path, "fehler", err)
	} else {
		slog.Info("GeoIP-Datenbank geladen", "pfad", geoPath)
		defer geo.Close()
	}
	tracer := trace.NewTracer(geo)

	// --- HTTP-Server ---
	srv := &http.Server{
		Addr:              cfg.Web.Listen,
		Handler:           web.NewServer(st, auth.NewTokens(cfg.Auth.SecretKey), runner, tracer).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info(version.Name+" läuft", "adresse", cfg.Web.Listen, "version", version.Version, "build", version.BuildDate)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP-Server: %w", err)
		}
	case <-sig:
		slog.Info("Beende …")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// ensureDefaultAdmin legt den Standard-Admin an, solange kein Benutzer existiert.
func ensureDefaultAdmin(st *store.Store) error {
	ctx := context.Background()
	n, err := st.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("Benutzer konnten nicht gezählt werden: %w", err)
	}
	if n > 0 {
		return nil
	}
	admin, err := auth.DefaultAdmin()
	if err != nil {
		return err
	}
	if err := st.CreateUser(ctx, admin); err != nil {
		return fmt.Errorf("Standard-Admin konnte nicht angelegt werden: %w", err)
	}
	slog.Warn("Standard-Admin angelegt – Passwort bitte umgehend ändern",
		"benutzer", auth.DefaultAdminUsername, "passwort", auth.DefaultAdminPassword)
	return nil
}
