package iperf

import (
	"context"
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

// FindBinary liefert den Pfad zu iperf3: den konfigurierten Pfad, sonst die
// Suche im PATH, unter Windows zusätzlich übliche Installationsorte.
func FindBinary(configured string) string {
	if configured != "" {
		return configured
	}
	if p, err := exec.LookPath("iperf3"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet", "Packages",
				"ar51an.iPerf3_Microsoft.Winget.Source_8wekyb3d8bbwe", "iperf3.exe"),
			`C:\Program Files\iperf3\iperf3.exe`,
			`C:\iperf3\iperf3.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return "iperf3"
}

var versionRe = regexp.MustCompile(`iperf (\d+)\.(\d+)`)

// CheckVersion ermittelt die iperf3-Version und prüft, ob sie --json-stream
// unterstützt. Die Version wird auch im Fehlerfall geliefert, sofern bekannt.
func CheckVersion(binary string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "--version").Output()
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
