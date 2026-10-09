package i18n

import (
	"net/http/httptest"
	"testing"
)

func TestTranslate(t *testing.T) {
	cases := []struct{ de, en string }{
		{"Server nicht gefunden", "Server not found"},
		{"Server 7 nicht gefunden", "Server 7 not found"},
		{"Server Deutschland ist deaktiviert", "Server Deutschland is disabled"}, // Name bleibt
		{`Parameter "limit" muss eine Zahl zwischen 1 und 1000 sein`, `Parameter "limit" must be a number between 1 and 1000`},
		{"12 Trace(s) und 140 Hop(s) gelöscht", "12 trace(s) and 140 hop(s) deleted"},
		{"Installation fehlgeschlagen: winget ist nicht verfügbar – iperf3 bitte manuell installieren",
			"Installation failed: winget is not available – please install iperf3 manually"},
		{"iperf3 nicht installiert – kein unterstützter Paketmanager gefunden (apt-get, dnf, yum, zypper, apk, pacman)",
			"iperf3 not installed – no supported package manager found (apt-get, dnf, yum, zypper, apk, pacman)"},
		{"iperf3 kann hier nicht automatisch installiert werden: iperf3 nicht installiert",
			"iperf3 cannot be installed automatically here: iperf3 not installed"},
		{"tracert konnte nicht gestartet werden: exec: not found", "tracert could not be started: exec: not found"},
		{"Zeitüberschreitung nach 1m30s", "Timed out after 1m30s"},
		{"Passwort muss mindestens 10 Zeichen lang sein", "Password must be at least 10 characters long"},
		{"Zu viele Fehlversuche – bitte später erneut versuchen", "Too many failed attempts – please try again later"},
		{"Öffentlicher iperf3-Server von AT&T in Virginia", "Public iperf3 server by AT&T in Virginia"},
		// Meldungen von iperf3 selbst bleiben unverändert.
		{"unable to connect to server: Connection refused", "unable to connect to server: Connection refused"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Translate(c.de, EN); got != c.en {
			t.Errorf("Translate(%q)\n got %q\nwant %q", c.de, got, c.en)
		}
		if got := Translate(c.de, DE); got != c.de {
			t.Errorf("Translate(%q, DE) = %q", c.de, got)
		}
	}
}

func TestFromRequest(t *testing.T) {
	cases := []struct{ url, header, want string }{
		{"/", "", DE},
		{"/", "en-US,en;q=0.9", EN},
		{"/", "de-DE,de;q=0.9,en;q=0.8", DE},
		{"/", "fr", EN},
		{"/?lang=en", "de", EN},
		{"/?lang=de", "en", DE},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", c.url, nil)
		if c.header != "" {
			r.Header.Set("Accept-Language", c.header)
		}
		if got := FromRequest(r); got != c.want {
			t.Errorf("%s %q: %s, want %s", c.url, c.header, got, c.want)
		}
	}
}
