// Package svc verwaltet den Betriebssystemdienst (Windows-Dienst, systemd,
// launchd) über github.com/kardianos/service – nach dem Muster des
// Spherifyer.
//
// Der installierte Dienst startet die Binary mit dem Argument "service-run"
// und dem bei der Installation festgelegten Konfigurationspfad.
package svc

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kardianos/service"
)

// Name ist der Bezeichner des Dienstes.
const Name = "iperf3-tracker"

// config liefert die Dienstdefinition. cfgPath ist nur für die Installation
// relevant; Start/Stopp/Status finden den Dienst über den Namen.
func config(cfgPath string) *service.Config {
	wd := ""
	if exe, err := os.Executable(); err == nil {
		wd = filepath.Dir(exe)
	}
	args := []string{"service-run"}
	if cfgPath != "" {
		args = append(args, "-config", cfgPath)
	}
	cfg := &service.Config{
		Name:             Name,
		DisplayName:      "iperf3-Tracker",
		Description:      "iperf3-Tracker – zeitgesteuerte iperf3-Tests mit Weboberfläche.",
		Arguments:        args,
		WorkingDirectory: wd,
		Option:           service.KeyValue{},
	}
	// systemd: nach dem Netzwerk starten (unter Windows wären das Dienstnamen).
	if runtime.GOOS == "linux" {
		cfg.Dependencies = []string{"After=network-online.target", "Wants=network-online.target"}
	}
	// Optionales Dienstkonto aus der Umgebung (wie Spherifyer).
	if user := os.Getenv("SERVICE_ACCOUNT_USER"); user != "" {
		cfg.UserName = user
		if pw := os.Getenv("SERVICE_ACCOUNT_PASSWORD"); pw != "" {
			cfg.Option["Password"] = pw
		}
	}
	return cfg
}

// New baut den Dienst für prg (Aufruf mit "service-run").
func New(prg service.Interface, cfgPath string) (service.Service, error) {
	return service.New(prg, config(cfgPath))
}

// Interactive meldet, ob das Programm in einer Konsole läuft (und nicht unter
// dem Dienstmanager).
func Interactive() bool { return service.Interactive() }

func control(action, cfgPath string) error {
	s, err := service.New(noopProgram{}, config(cfgPath))
	if err != nil {
		return err
	}
	return service.Control(s, action)
}

// Install registriert den Dienst; cfgPath wird als -config übergeben.
func Install(cfgPath string) error { return control("install", cfgPath) }

// Uninstall entfernt den Dienst.
func Uninstall() error { return control("uninstall", "") }

// Start startet den installierten Dienst.
func Start() error { return control("start", "") }

// Stop stoppt den Dienst.
func Stop() error { return control("stop", "") }

// Restart startet den Dienst neu.
func Restart() error { return control("restart", "") }

// Status meldet "läuft", "gestoppt", "nicht installiert" oder "unbekannt".
func Status() (string, error) {
	s, err := service.New(noopProgram{}, config(""))
	if err != nil {
		return "", err
	}
	st, err := s.Status()
	if err != nil {
		if errors.Is(err, service.ErrNotInstalled) {
			return "nicht installiert", nil
		}
		return "", err
	}
	switch st {
	case service.StatusRunning:
		return "läuft", nil
	case service.StatusStopped:
		return "gestoppt", nil
	default:
		return "unbekannt", nil
	}
}

// Platform liefert das Dienstsystem, z. B. "windows-service" oder "linux-systemd".
func Platform() string { return service.Platform() }

type noopProgram struct{}

func (noopProgram) Start(service.Service) error { return nil }
func (noopProgram) Stop(service.Service) error  { return nil }
