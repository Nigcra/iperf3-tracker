// Package i18n übersetzt Meldungen der API ins Englische.
//
// Ausgangssprache aller Meldungen ist Deutsch – auch der in der Datenbank
// gespeicherten Fehlermeldungen von Tests und Traces. Übersetzt wird erst bei
// der Ausgabe, so erscheinen auch ältere Einträge in der gewählten Sprache.
// Meldungen mit Platzhaltern werden über Muster erkannt. Werte für %v sind
// selbst Meldungen und werden ihrerseits übersetzt, damit verschachtelte
// Meldungen ("Installation fehlgeschlagen: …") vollständig englisch werden;
// Werte für %s (Namen, Hosts) bleiben unverändert.
package i18n

import (
	"net/http"
	"regexp"
	"strings"
)

const (
	DE = "de"
	EN = "en"
)

// FromRequest bestimmt die Sprache aus dem Query-Parameter "lang" (für
// EventSource, das keine eigenen Header setzen kann) oder dem ersten Eintrag
// von Accept-Language. Ohne Angabe bleibt es bei Deutsch.
func FromRequest(r *http.Request) string {
	if l := r.URL.Query().Get("lang"); l != "" {
		return normalize(l)
	}
	if h := r.Header.Get("Accept-Language"); h != "" {
		tag, _, _ := strings.Cut(h, ",")
		tag, _, _ = strings.Cut(tag, ";")
		return normalize(tag)
	}
	return DE
}

func normalize(tag string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), "de") {
		return DE
	}
	return EN
}

// Translate liefert s in der Sprache lang. Unbekannte Texte (etwa
// Meldungen von iperf3 selbst) bleiben unverändert.
func Translate(s, lang string) string {
	if lang == DE || s == "" {
		return s
	}
	return translate(s, 0)
}

func translate(s string, depth int) string {
	if en, ok := exact[s]; ok {
		return en
	}
	if depth > 4 {
		return s
	}
	for _, p := range patterns {
		m := p.re.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		args := m[1:]
		for i, a := range args {
			if p.translateArg[i] {
				args[i] = translate(a, depth+1)
			}
		}
		i := 0
		return verb.ReplaceAllStringFunc(p.en, func(string) string {
			i++
			return args[i-1]
		})
	}
	return s
}

// verb erkennt die Platzhalter in den Mustern.
var verb = regexp.MustCompile(`%[sdqvw]`)

type pattern struct {
	re           *regexp.Regexp
	en           string
	translateArg []bool // nur %v (eingebettete Meldungen) wird mitübersetzt, %s (Namen, Hosts) nicht
}

var (
	exact    = map[string]string{}
	patterns []pattern
)

func init() {
	for _, e := range catalog {
		de, en := e[0], e[1]
		if !verb.MatchString(de) {
			exact[de] = en
			continue
		}
		var re strings.Builder
		var tr []bool
		re.WriteString("^")
		rest := de
		for {
			loc := verb.FindStringIndex(rest)
			if loc == nil {
				re.WriteString(regexp.QuoteMeta(rest))
				break
			}
			re.WriteString(regexp.QuoteMeta(rest[:loc[0]]))
			switch rest[loc[0]+1] {
			case 'd':
				re.WriteString(`(-?\d+)`)
				tr = append(tr, false)
			case 'q':
				re.WriteString(`("[^"]*")`)
				tr = append(tr, false)
			case 's':
				re.WriteString(`(.+?)`)
				tr = append(tr, false)
			default:
				re.WriteString(`(.+?)`)
				tr = append(tr, true)
			}
			rest = rest[loc[1]:]
		}
		re.WriteString("$")
		patterns = append(patterns, pattern{regexp.MustCompile(re.String()), en, tr})
	}
}

// catalog ordnet jeder deutschen Meldung die englische zu. Muster werden in
// dieser Reihenfolge geprüft – allgemeine Muster daher ans Ende.
var catalog = [][2]string{
	// Allgemein
	{"Interner Serverfehler", "Internal server error"},
	{"Ungültiger Request-Body: %s", "Invalid request body: %s"},
	{"Ungültige Zeichenkodierung – erwartet wird UTF-8", "Invalid character encoding – UTF-8 expected"},
	{"Ungültiger Parameter %q", "Invalid parameter %q"},
	{"Parameter %q muss eine Zahl zwischen %d und %d sein", "Parameter %q must be a number between %d and %d"},
	{"Parameter %q muss true oder false sein", "Parameter %q must be true or false"},
	{"Parameter %q muss eine Zahl sein", "Parameter %q must be a number"},
	{"Parameter %q ist kein gültiger Zeitpunkt", "Parameter %q is not a valid point in time"},
	{"Parameter %q darf nicht negativ sein", "Parameter %q must not be negative"},
	{"Parameter %q fehlt", "Parameter %q is missing"},
	{"Parameter %q muss pending, running, completed oder failed sein", "Parameter %q must be pending, running, completed or failed"},
	{"Datensatz nicht gefunden", "Record not found"},
	{"Wert bereits vergeben", "Value already taken"},

	// Anmeldung und Benutzer
	{"Benutzername oder Passwort falsch", "Wrong username or password"},
	{"Benutzer ist deaktiviert", "User is disabled"},
	{"Anmeldedaten konnten nicht geprüft werden", "Could not validate credentials"},
	{"ungültiges Token", "invalid token"},
	{"Keine ausreichende Berechtigung", "Insufficient permissions"},
	{"Benutzername ist bereits vergeben", "Username is already taken"},
	{"E-Mail-Adresse ist bereits registriert", "Email address is already registered"},
	{"Benutzername muss 3–50 Zeichen lang sein", "Username must be 3–50 characters long"},
	{"E-Mail-Adresse muss 3–100 Zeichen lang sein", "Email address must be 3–100 characters long"},
	{"Passwort muss mindestens %d Zeichen lang sein", "Password must be at least %d characters long"},
	{"Passwort darf höchstens 256 Byte lang sein", "Password must be at most 256 bytes long"},
	{"Das neue Passwort muss sich vom bisherigen unterscheiden", "The new password must differ from the current one"},
	{"Passwortwechsel erforderlich", "Password change required"},
	{"CSRF-Prüfung fehlgeschlagen – bitte die Seite neu laden", "CSRF check failed – please reload the page"},
	{"Anfrage von fremder Herkunft abgelehnt", "Cross-origin request rejected"},
	{"Das eigene Konto kann nicht gelöscht werden", "You cannot delete your own account"},
	{"Benutzer nicht gefunden", "User not found"},
	{"Benutzer gelöscht", "User deleted"},
	{"Aktuelles Passwort ist falsch", "Current password is wrong"},
	{"Zu viele Fehlversuche – bitte später erneut versuchen", "Too many failed attempts – please try again later"},
	{"Passwort geändert", "Password changed"},

	// Server
	{"Server nicht gefunden", "Server not found"},
	{"Server %d nicht gefunden", "Server %d not found"},
	{"Server %s ist deaktiviert", "Server %s is disabled"},
	{"Ein Server mit diesem Namen existiert bereits", "A server with this name already exists"},
	{"Name muss 1–100 Zeichen lang sein", "Name must be 1–100 characters long"},
	{"Host muss 1–255 Zeichen lang sein", "Host must be 1–255 characters long"},
	{"Port muss zwischen 1 und 65535 liegen", "Port must be between 1 and 65535"},
	{"Testdauer muss zwischen 1 und 300 Sekunden liegen", "Duration must be between 1 and 300 seconds"},
	{"Parallele Streams müssen zwischen 1 und 128 liegen", "Parallel streams must be between 1 and 128"},
	{"Anzahl Streams muss zwischen 1 und 128 liegen", "Number of streams must be between 1 and 128"},
	{"Protokoll muss tcp oder udp sein", "Protocol must be tcp or udp"},
	{"Richtung muss download, upload oder bidirectional sein", "Direction must be download, upload or bidirectional"},
	{"Intervall muss mindestens 1 Minute betragen", "Interval must be at least 1 minute"},
	{"UDP-Bandbreite muss größer als 0 und höchstens 100000 Mbit/s sein", "UDP bandwidth must be greater than 0 and at most 100000 Mbit/s"},

	// Öffentliche Server
	{"Frankreich", "France"},
	{"Deutschland", "Germany"},
	{"Schweiz", "Switzerland"},
	{"Paris, Frankreich", "Paris, France"},
	{"Hamburg, Deutschland", "Hamburg, Germany"},
	{"Frankfurt, Deutschland", "Frankfurt, Germany"},
	{"Wolfsburg, Deutschland", "Wolfsburg, Germany"},
	{"Taschkent, Usbekistan", "Tashkent, Uzbekistan"},
	{"Öffentlicher iperf3-Server von %s in %s", "Public iperf3 server by %s in %s"},
	{"Öffentlicher Server von %s in %s", "Public server by %s in %s"},
	{"Öffentlicher iperf3-Server von %s", "Public iperf3 server by %s"},
	{"Öffentlicher iperf3-Server in %s", "Public iperf3 server in %s"},
	{"Öffentlicher Testserver von %s", "Public test server by %s"},

	// Tests
	{"Test nicht gefunden", "Test not found"},
	{"%d Test(s) gelöscht", "%d test(s) deleted"},
	{"Bitte days, server_id oder all=true angeben", "Please specify days, server_id or all=true"},
	{"Bitte days oder all=true angeben", "Please specify days or all=true"},
	{"Abgebrochen: Dienst wurde neu gestartet", "Aborted: the service was restarted"},
	{"iperf3 konnte nicht gestartet werden: %v", "iperf3 could not be started: %v"},
	{"iperf3 beendet mit Fehler: %v", "iperf3 exited with an error: %v"},
	{"iperf3 lieferte keine Ergebnisse für die Richtung %s", "iperf3 returned no results for direction %s"},
	{"Zeitüberschreitung nach %s", "Timed out after %s"},

	// iperf3-Installation
	{"iperf3 nicht installiert", "iperf3 not installed"},
	{"iperf3 nicht ausführbar (%s): %v", "iperf3 not executable (%s): %v"},
	{"iperf3-Version nicht erkennbar", "iperf3 version not recognizable"},
	{"iperf3 %s ist zu alt – benötigt wird mindestens %d.%d (--json-stream)", "iperf3 %s is too old – at least %d.%d is required (--json-stream)"},
	{"winget ist nicht verfügbar – iperf3 bitte manuell installieren", "winget is not available – please install iperf3 manually"},
	{"Homebrew ist nicht verfügbar – iperf3 bitte manuell installieren", "Homebrew is not available – please install iperf3 manually"},
	{"für die Installation über %s sind Root-Rechte nötig, sudo ist nicht verfügbar", "installing via %s requires root privileges, sudo is not available"},
	{"kein unterstützter Paketmanager gefunden (apt-get, dnf, yum, zypper, apk, pacman)", "no supported package manager found (apt-get, dnf, yum, zypper, apk, pacman)"},
	{"automatische Installation unter %s nicht unterstützt", "automatic installation is not supported on %s"},
	{"Installation läuft bereits", "Installation already running"},
	{"iperf3 kann hier nicht automatisch installiert werden", "iperf3 cannot be installed automatically here"},
	{"Installation fehlgeschlagen: %v", "Installation failed: %v"},
	{"Installation laut Paketmanager erfolgreich, iperf3 wurde aber nicht gefunden – ggf. den Dienst neu starten (PATH) oder iperf.path setzen",
		"The package manager reports success, but iperf3 was not found – restart the service (PATH) or set iperf.path"},

	// Traces
	{"Trace nicht gefunden", "Trace not found"},
	{"Trace gestartet", "Trace started"},
	{"Trace gelöscht", "Trace deleted"},
	{"Trace konnte nicht gespeichert werden", "Trace could not be saved"},
	{"%d Trace(s) und %d Hop(s) gelöscht", "%d trace(s) and %d hop(s) deleted"},
	{"Für diesen Test existiert bereits ein Trace", "A trace already exists for this test"},
	{"Für diesen Test läuft bereits ein Trace", "A trace is already running for this test"},
	{"Ziel muss 1–255 Zeichen lang sein", "Destination must be 1–255 characters long"},
	{"max_hops muss zwischen 1 und 64 liegen", "max_hops must be between 1 and 64"},
	{"timeout muss zwischen 1 und 10 Sekunden liegen", "timeout must be between 1 and 10 seconds"},
	{"count muss zwischen 1 und 10 liegen", "count must be between 1 and 10"},
	{"Ziel konnte nicht aufgelöst werden: %v", "Could not resolve destination: %v"},
	{"%s nicht gefunden – bitte installieren", "%s not found – please install it"},
	{"%s konnte nicht gestartet werden: %v", "%s could not be started: %v"},
	{"%s beendet mit Fehler: %v", "%s exited with an error: %v"},
	{"Abgebrochen", "Aborted"},
	{"nur IPv4 wird unterstützt", "only IPv4 is supported"},

	// Zusammengesetzte Meldungen (zuletzt prüfen)
	{"%v – %v", "%v – %v"},
	{"%v: %v", "%v: %v"},
}
