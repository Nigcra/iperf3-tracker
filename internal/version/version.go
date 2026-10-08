// Package version hält Versionskennung und die zur Build-Zeit injizierte
// Build-Zeit. BuildDate wird über -ldflags
// "-X iperf3-tracker/internal/version.BuildDate=..." gesetzt (siehe build.cmd)
// und erlaubt es, zweifelsfrei festzustellen, welches Binary tatsächlich läuft.
package version

// Name ist der Anzeigename der Anwendung.
const Name = "iperf3-Tracker"

// Version ist die Anwendungsversion. 2.x kennzeichnet die Go-Portierung.
const Version = "2.0.0-dev"

// BuildDate ist der zur Build-Zeit gesetzte Zeitstempel (Format 2006-01-02 15:04).
// "dev" bei ungestempelten Builds (z. B. direkter `go run`).
var BuildDate = "dev"
