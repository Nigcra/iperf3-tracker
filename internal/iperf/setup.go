package iperf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// installTimeout begrenzt eine automatische Installation.
const installTimeout = 10 * time.Minute

// Status beschreibt die iperf3-Installation, die der Runner verwendet.
type Status struct {
	Path       string `json:"path"`
	Version    string `json:"version,omitempty"`
	Available  bool   `json:"available"`  // vorhanden und neu genug
	Installing bool   `json:"installing"` // automatische Installation läuft
	Error      string `json:"error,omitempty"`
}

// installFunc installiert iperf3; in Tests ersetzbar.
var installFunc = Install

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

// useBinary prüft binary und übernimmt es samt Status.
func (r *Runner) useBinary(binary string) error {
	version, err := CheckVersion(binary)
	st := Status{Path: binary, Version: version, Available: err == nil}
	if err != nil {
		st.Error = err.Error()
	}
	r.cmdMu.Lock()
	r.command = []string{binary}
	r.status = st
	r.cmdMu.Unlock()
	return err
}

// Prepare sucht iperf3 und prüft die Version. Fehlt iperf3 und ist kein Pfad
// fest konfiguriert, wird es bei autoInstall im Hintergrund über den
// Paketmanager installiert (siehe Install) und anschließend verwendet. Eine zu
// alte Version wird nur gemeldet, nicht ersetzt.
func (r *Runner) Prepare(configured string, autoInstall bool) {
	binary := FindBinary(configured)
	err := r.useBinary(binary)
	switch {
	case err == nil:
		slog.Info("iperf3 gefunden", "pfad", binary, "version", r.Status().Version)
		return
	case !errors.Is(err, ErrNotInstalled) || configured != "" || !autoInstall:
		slog.Warn("iperf3 nicht nutzbar – Tests werden fehlschlagen", "pfad", binary, "fehler", err)
		return
	}

	slog.Info("iperf3 nicht gefunden – starte automatische Installation")
	r.cmdMu.Lock()
	r.status.Installing = true
	r.status.Error = ""
	r.cmdMu.Unlock()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ctx, cancel := context.WithTimeout(r.ctx, installTimeout)
		defer cancel()
		installErr := installFunc(ctx)

		// Auch nach einem Fehler neu suchen: winget meldet z. B. einen Fehler,
		// wenn das Paket bereits installiert ist.
		binary := FindBinary("")
		if err := r.useBinary(binary); err != nil {
			if installErr != nil {
				err = fmt.Errorf("Installation fehlgeschlagen: %w", installErr)
				r.cmdMu.Lock()
				r.status.Error = err.Error()
				r.cmdMu.Unlock()
			}
			slog.Error("iperf3 konnte nicht installiert werden", "fehler", err)
			return
		}
		slog.Info("iperf3 installiert", "pfad", binary, "version", r.Status().Version)
	}()
}
