package iperf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// Mindestversion für --json-stream.
const minMajor, minMinor = 3, 17

// ErrNotInstalled meldet, dass keine ausführbare iperf3-Binary gefunden wurde.
var ErrNotInstalled = errors.New("iperf3 nicht installiert")

// FindBinary liefert den Pfad zu iperf3: den konfigurierten Pfad, sonst die
// Suche im PATH, unter Windows zusätzlich die Ablageorte von winget (Benutzer-
// und Maschineninstallation) und übliche manuelle Installationsorte.
func FindBinary(configured string) string {
	if configured != "" {
		return configured
	}
	if p, err := exec.LookPath("iperf3"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("LOCALAPPDATA") + `\Microsoft\WinGet`, os.Getenv("ProgramFiles") + `\WinGet`} {
			if p := filepath.Join(root, "Links", "iperf3.exe"); fileExists(p) {
				return p
			}
			if m, _ := filepath.Glob(filepath.Join(root, "Packages", "ar51an.iPerf3_*", "iperf3.exe")); len(m) > 0 {
				return m[0]
			}
		}
		for _, p := range []string{`C:\Program Files\iperf3\iperf3.exe`, `C:\iperf3\iperf3.exe`} {
			if fileExists(p) {
				return p
			}
		}
	}
	return "iperf3"
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

var versionRe = regexp.MustCompile(`iperf (\d+)\.(\d+)`)

// CheckVersion ermittelt die iperf3-Version und prüft, ob sie --json-stream
// unterstützt. Die Version wird auch im Fehlerfall geliefert, sofern bekannt.
// Fehlt die Binary, ist der Fehler ErrNotInstalled.
func CheckVersion(binary string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "--version").Output()
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return "", ErrNotInstalled
	}
	if err != nil {
		return "", fmt.Errorf("iperf3 nicht ausführbar (%s): %w", binary, err)
	}
	m := versionRe.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("iperf3-Version nicht erkennbar")
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	version := m[1] + "." + m[2]
	if major < minMajor || (major == minMajor && minor < minMinor) {
		return version, fmt.Errorf("iperf3 %s ist zu alt – benötigt wird mindestens %d.%d (--json-stream)", version, minMajor, minMinor)
	}
	return version, nil
}
