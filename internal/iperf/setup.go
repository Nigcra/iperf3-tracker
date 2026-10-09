package iperf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// installTimeout begrenzt eine Installation.
const installTimeout = 10 * time.Minute

var (
	// ErrInstallRunning meldet, dass bereits eine Installation läuft.
	ErrInstallRunning = errors.New("Installation läuft bereits")
	// ErrNotInstallable meldet, dass iperf3 hier nicht installiert werden kann
	// (bereits vorhanden, Pfad fest konfiguriert oder kein Paketmanager).
	ErrNotInstallable = errors.New("iperf3 kann hier nicht automatisch installiert werden")
)

// Status beschreibt die iperf3-Installation, die der Runner verwendet.
type Status struct {
	Path       string `json:"path"`
	Version    string `json:"version,omitempty"`
	Available  bool   `json:"available"`  // vorhanden und neu genug
	Installing bool   `json:"installing"` // Installation läuft
	// Installable ist true, wenn iperf3 fehlt und über den Paketmanager
	// installiert werden kann; InstallCommand nennt die Befehle dafür.
	Installable    bool   `json:"installable"`
	InstallCommand string `json:"install_command,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Austauschbar für Tests: Ermittlung der Installationsbefehle und Installation.
var (
	installPlan = func() ([][]string, error) {
		return installCommands(runtime.GOOS, exec.LookPath, os.Geteuid() == 0)
	}
	installFunc = Install
)

// Status liefert den aktuellen iperf3-Status.
func (r *Runner) Status() Status {
	r.cmdMu.RLock()
	defer r.cmdMu.RUnlock()
	return r.status
}

func (r *Runner) commandLine() []string {
	r.cmdMu.RLock()
	defer r.cmdMu.RUnlock()
	return r.command
}

// refresh prüft binary, übernimmt es und ermittelt, ob iperf3 installierbar ist.
func (r *Runner) refresh(binary string) error {
	r.cmdMu.RLock()
	configured := r.configured
	r.cmdMu.RUnlock()

	version, err := CheckVersion(binary)
	st := Status{Path: binary, Version: version, Available: err == nil}
	if err != nil {
		st.Error = err.Error()
		if errors.Is(err, ErrNotInstalled) && configured == "" {
			if cmds, planErr := installPlan(); planErr == nil {
				st.Installable = true
				parts := make([]string, len(cmds))
				for i, c := range cmds {
					parts[i] = strings.Join(c, " ")
				}
				st.InstallCommand = strings.Join(parts, " && ")
			} else {
				st.Error += " – " + planErr.Error()
			}
		}
	}
	r.cmdMu.Lock()
	r.command = []string{binary}
	r.status = st
	r.cmdMu.Unlock()
	return err
}

// Prepare sucht iperf3 und prüft die Version. Fehlt iperf3, wird es nur bei
// autoInstall sofort installiert; sonst bleibt die Installation der
// Oberfläche überlassen (InstallAsync nach Bestätigung durch einen Admin).
// Eine zu alte Version wird nur gemeldet, ein konfigurierter Pfad nie ersetzt.
func (r *Runner) Prepare(configured string, autoInstall bool) {
	r.cmdMu.Lock()
	r.configured = configured
	r.cmdMu.Unlock()

	binary := FindBinary(configured)
	err := r.refresh(binary)
	if err == nil {
		slog.Info("iperf3 gefunden", "path", binary, "version", r.Status().Version)
		return
	}
	if autoInstall && r.Status().Installable {
		slog.Info("iperf3 nicht gefunden – starte automatische Installation")
		r.InstallAsync()
		return
	}
	slog.Warn("iperf3 nicht nutzbar – Tests werden fehlschlagen", "path", binary, "error", err,
		"installable", r.Status().Installable)
}

// InstallAsync installiert iperf3 im Hintergrund über den Paketmanager und
// verwendet es anschließend ohne Neustart.
func (r *Runner) InstallAsync() error {
	r.cmdMu.Lock()
	switch {
	case r.status.Installing:
		r.cmdMu.Unlock()
		return ErrInstallRunning
	case !r.status.Installable:
		r.cmdMu.Unlock()
		return ErrNotInstallable
	}
	r.status.Installing = true
	r.status.Error = ""
	r.cmdMu.Unlock()
	slog.Info("Installiere iperf3", "command", r.Status().InstallCommand)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ctx, cancel := context.WithTimeout(r.ctx, installTimeout)
		defer cancel()
		installErr := installFunc(ctx)

		// Auch nach einem Fehler neu suchen: winget meldet z. B. einen Fehler,
		// wenn das Paket bereits installiert ist.
		binary := FindBinary("")
		if err := r.refresh(binary); err != nil {
			switch {
			case installErr != nil:
				err = fmt.Errorf("Installation fehlgeschlagen: %w", installErr)
			case errors.Is(err, ErrNotInstalled):
				err = errors.New("Installation laut Paketmanager erfolgreich, iperf3 wurde aber nicht gefunden – " +
					"ggf. den Dienst neu starten (PATH) oder iperf.path setzen")
			}
			r.cmdMu.Lock()
			r.status.Error = err.Error()
			r.cmdMu.Unlock()
			slog.Error("iperf3 konnte nicht installiert werden", "error", err)
			return
		}
		slog.Info("iperf3 installiert", "path", binary, "version", r.Status().Version)
	}()
	return nil
}
