// Command iperf3-tracker führt zeit- und benutzergesteuerte iperf3-Tests aus,
// speichert die Ergebnisse in SQLite und stellt API und Oberfläche über HTTP bereit.
//
// Aufruf:
//
//	iperf3-tracker [-config config.yaml] [-version] [-no-tui]
//	iperf3-tracker service install|uninstall|start|stop|restart|status [-config pfad]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/kardianos/service"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/config"
	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/iperf"
	"iperf3-tracker/internal/scheduler"
	"iperf3-tracker/internal/store"
	"iperf3-tracker/internal/svc"
	"iperf3-tracker/internal/trace"
	"iperf3-tracker/internal/version"
	"iperf3-tracker/internal/web"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "service-run":
			if err := runService(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "Dienstfehler:", err)
				os.Exit(1)
			}
			return
		case "service":
			handleServiceCommand(os.Args[2:])
			return
		}
	}

	cfgPath := flag.String("config", "config.yaml", "Pfad zur Konfigurationsdatei")
	showVersion := flag.Bool("version", false, "Version ausgeben und beenden")
	flag.Bool("no-tui", false, "Ohne Terminal-Oberfläche starten (ohne Wirkung: der iperf3-Tracker hat keine TUI)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s (Build %s, %s/%s)\n", version.Name, version.Version, version.BuildDate, runtime.GOOS, runtime.GOARCH)
		return
	}

	stop := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		close(stop)
	}()
	if err := run(*cfgPath, os.Stderr, stop); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
}

// newLogger richtet slog nach log.level und log.format ein.
func newLogger(out io.Writer, cfg config.LogConfig) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.SlogLevel()}
	if cfg.Format == "json" {
		return slog.New(slog.NewJSONHandler(out, opts))
	}
	return slog.New(slog.NewTextHandler(out, opts))
}

// run lädt die Konfiguration, startet alle Dienste und blockiert, bis stop
// geschlossen wird oder der HTTP-Server scheitert.
func run(cfgPath string, logOut io.Writer, stop <-chan struct{}) error {
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

	slog.SetDefault(newLogger(logOut, cfg.Log))
	if created {
		slog.Info("Neue Konfiguration erstellt", "path", cfgPath)
	}
	for _, w := range cfg.Warnings {
		slog.Warn(w)
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
	slog.Info("Datenbank geöffnet", "path", cfg.Storage.Path)

	if !svc.Interactive() {
		// Im Dienstbetrieb sieht niemand die Konsole: das Startpasswort
		// zusätzlich neben die Konfiguration schreiben.
		web.InitialPasswordHook = func(username, password string) {
			p := filepath.Join(cfg.Dir, "initial-admin-password.txt")
			text := fmt.Sprintf("Benutzer: %s\nPasswort: %s\n\nDas Passwort muss bei der ersten Anmeldung geändert werden.\nDiese Datei danach löschen.\n", username, password)
			if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
				slog.Error("Startpasswort konnte nicht gespeichert werden", "path", p, "error", err)
				return
			}
			slog.Warn("Startpasswort zusätzlich gespeichert", "path", p)
		}
	}
	if _, err := web.EnsureAdmin(context.Background(), st); err != nil {
		return err
	}
	if err := web.FlagLegacyDefaultPassword(context.Background(), st); err != nil {
		return fmt.Errorf("Admin-Konto konnte nicht geprüft werden: %w", err)
	}
	if n, err := st.FailUnfinishedTests(context.Background(), "Abgebrochen: Dienst wurde neu gestartet"); err != nil {
		return fmt.Errorf("Offene Tests konnten nicht bereinigt werden: %w", err)
	} else if n > 0 {
		slog.Warn("Unterbrochene Tests als fehlgeschlagen markiert", "count", n)
	}

	// --- iperf3 ---
	runner := iperf.NewRunner(st, iperf.FindBinary(cfg.Iperf.Path))
	defer runner.Stop()
	runner.Prepare(cfg.Iperf.Path, cfg.Iperf.AutoInstall)

	// --- GeoIP für Trace-Standorte ---
	geo, geoPath, err := trace.OpenGeoIP(cfg.GeoIP.Path, "/var/lib/GeoIP/GeoLite2-City.mmdb")
	if err != nil {
		slog.Warn("GeoIP-Datenbank nicht verfügbar – Traces ohne Standorte", "path", cfg.GeoIP.Path, "error", err)
	} else {
		slog.Info("GeoIP-Datenbank geladen", "path", geoPath)
		defer geo.Close()
	}
	tracer := trace.NewTracer(geo)

	// --- Scheduler ---
	sched := scheduler.New(st, runner, tracer)
	if cfg.Scheduler.Enabled {
		if err := sched.Start(context.Background()); err != nil {
			return fmt.Errorf("Scheduler konnte nicht gestartet werden: %w", err)
		}
		defer sched.Stop()
	} else {
		slog.Info("Scheduler per Konfiguration deaktiviert")
	}

	// --- HTTP-Server ---
	if cfg.Map.TileURL != "" {
		slog.Info("Peering-Map nutzt Kachelserver", "tile_url", cfg.Map.TileURL)
	}
	srv := &http.Server{
		Addr: cfg.Web.Listen,
		Handler: web.NewServer(web.Deps{
			Store:     st,
			Tokens:    auth.NewTokens(cfg.Auth.SecretKey),
			Runner:    runner,
			Tracer:    tracer,
			Scheduler: sched,
			Map: web.MapSettings{
				TileURL:         cfg.Map.TileURL,
				TileAttribution: cfg.Map.TileAttribution,
				TileOrigin:      cfg.Map.TileOrigin(),
			},
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", cfg.Web.Listen)
	if err != nil {
		return fmt.Errorf("HTTP-Server: %w", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	slog.Info(version.Name+" läuft", "address", cfg.Web.Listen, "version", version.Version, "build", version.BuildDate)
	if host, port, err := net.SplitHostPort(cfg.Web.Listen); err == nil {
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "localhost"
		}
		slog.Info("Oberfläche: http://" + net.JoinHostPort(host, port))
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP-Server: %w", err)
		}
	case <-stop:
		slog.Info("Beende …")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// ---------- Dienstbetrieb ----------

// program bindet run an den Dienstmanager (kardianos/service).
type program struct {
	cfgPath string
	logOut  io.Writer
	stop    chan struct{}
	done    chan struct{}
}

func (p *program) Start(service.Service) error {
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		if err := run(p.cfgPath, p.logOut, p.stop); err != nil {
			slog.Error("Dienst beendet", "error", err)
			fmt.Fprintln(p.logOut, "Fehler:", err)
			os.Exit(1)
		}
	}()
	return nil
}

func (p *program) Stop(service.Service) error {
	close(p.stop)
	select {
	case <-p.done:
	case <-time.After(15 * time.Second):
	}
	return nil
}

// runService läuft unter dem Dienstmanager (Argument "service-run").
func runService(args []string) error {
	fs := flag.NewFlagSet("service-run", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Pfad zur Konfigurationsdatei")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path, err := serviceConfigPath(*cfgPath)
	if err != nil {
		return err
	}
	// Windows-Dienste starten in System32 und haben keine Konsole: ins
	// Verzeichnis der Konfiguration wechseln und dort ins Logfile schreiben.
	var logOut io.Writer = os.Stderr
	if runtime.GOOS == "windows" && !service.Interactive() {
		f, err := os.OpenFile(filepath.Join(filepath.Dir(path), "iperf3-tracker.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err == nil {
			defer f.Close()
			logOut = f
		}
	}
	_ = os.Chdir(filepath.Dir(path))
	s, err := svc.New(&program{cfgPath: path, logOut: logOut}, path)
	if err != nil {
		return err
	}
	return s.Run()
}

// serviceConfigPath liefert den absoluten Konfigurationspfad für den Dienst;
// ohne Angabe config.yaml neben der Binary.
func serviceConfigPath(p string) (string, error) {
	if p == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		return filepath.Join(filepath.Dir(exe), "config.yaml"), nil
	}
	return filepath.Abs(p)
}

// handleServiceCommand verarbeitet
//
//	iperf3-tracker service install|uninstall|start|stop|restart|status [-config pfad]
func handleServiceCommand(args []string) {
	usage := func() {
		fmt.Println("Verwendung: iperf3-tracker service <install|uninstall|start|stop|restart|status> [-config pfad]")
		os.Exit(2)
	}
	if len(args) == 0 {
		usage()
	}
	cmd := args[0]
	fs := flag.NewFlagSet("service", flag.ExitOnError)
	cfgFlag := fs.String("config", "", "Pfad zur Konfigurationsdatei (Standard: config.yaml neben der Binary)")
	_ = fs.Parse(args[1:])

	var err error
	switch cmd {
	case "install":
		var path string
		if path, err = serviceConfigPath(*cfgFlag); err == nil {
			if err = svc.Install(path); err == nil {
				fmt.Printf("Dienst %q installiert (%s), Konfiguration: %s\n", svc.Name, svc.Platform(), path)
			}
		}
	case "uninstall":
		err = svc.Uninstall()
	case "start":
		err = svc.Start()
	case "stop":
		err = svc.Stop()
	case "restart":
		err = svc.Restart()
	case "status":
		var st string
		if st, err = svc.Status(); err == nil {
			fmt.Println("Dienststatus:", st)
		}
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Dienstfehler:", err)
		os.Exit(1)
	}
	if cmd != "status" && cmd != "install" {
		fmt.Printf("Dienst-Aktion '%s' erfolgreich.\n", cmd)
	}
}
