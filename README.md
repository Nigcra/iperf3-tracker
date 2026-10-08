# iperf3-Tracker

Misst regelmäßig die Bandbreite zu iperf3-Servern, speichert die Ergebnisse
historisch und zeigt den Netzwerkpfad auf einer Karte. Eine einzelne
Go-Binary liefert API und Oberfläche aus – ohne weitere Laufzeitumgebung oder
Webserver.

![Dashboard](_screenshots/dashboard.png)

## Funktionen

- **Dashboard** – Kennzahlen, Download-/Upload-Verlauf je Server mit
  Schwellwerten, Live-Fortschritt laufender Tests
- **Tests** – Test mit beliebigen Parametern starten (TCP/UDP, Download,
  Upload, bidirektional, parallele Streams, UDP-Zielbandbreite), Live-Anzeige,
  Testverlauf mit Details und iperf3-Rohausgabe
- **Zeitplan** – Tests je Server im eingestellten Intervall, optional mit
  anschließender Traceroute (Auto-Trace)
- **Peering-Map** – Traceroute live auf der Karte, Standorte aus der
  GeoLite2-Datenbank (fehlende Standorte werden aus Nachbar-Hops geschätzt),
  Pfad- und Hop-Tabelle auch für private Netze
- **Server-Profile** – eigene oder öffentliche iperf3-Server (Auswahlliste)
- **Administration** – Benutzer, Bereinigung alter Daten, Datenbank-Statistik
- Hell/Dunkel, mobil nutzbar, Oberfläche auf Deutsch

| Tests | Peering-Map | Server (hell) |
|---|---|---|
| ![Tests](_screenshots/tests.png) | ![Peering-Map](_screenshots/peering-map.png) | ![Server](_screenshots/server-hell.png) |

## Voraussetzungen

- **iperf3 ab Version 3.17** (wegen `--json-stream`). Fehlt iperf3, bietet die
  Oberfläche Administratoren nach der Anmeldung die Installation an – unter
  Windows per winget, unter Linux über den vorhandenen Paketmanager (apt-get,
  dnf, yum, zypper, apk, pacman; ohne Root über `sudo -n`). Ohne Nachfrage beim
  Start installieren: `iperf.auto_install: true` (z. B. für Server ohne
  Oberfläche). Manuell:
  - Windows: `winget install ar51an.iPerf3`
  - Linux: Paket `iperf3` der Distribution, sofern mindestens 3.17
    (z. B. Debian 13: `apt install iperf3`)
- **tracert** (Windows, vorinstalliert) bzw. **traceroute** (Linux)
- Zum Bauen: **Go 1.24** oder neuer

## Schnellstart (Windows)

```cmd
start.cmd
```

Baut `_release\iperf3-tracker.exe` beim ersten Aufruf, startet den Dienst und
öffnet http://localhost:8000. Nach Code-Änderungen neu bauen mit
`start.cmd neu` oder `build.cmd release`.

Erste Anmeldung: **`admin` / `admin123`** – danach über das Benutzermenü
(Personen-Symbol oben rechts) **„Passwort ändern“**.

Die Binary lässt sich auch direkt per Doppelklick starten. `config.yaml` und
die Datenbank (`data\`) werden beim ersten Start neben der Binary angelegt;
`build.cmd` legt die GeoIP-Datenbank in `_release\geoip\` ab.

## Docker

```bash
docker compose up -d --build
```

Ein Container mit iperf3 3.18 und traceroute (Debian trixie); Konfiguration
und Datenbank liegen im Volume `iperf-data`. Oberfläche unter
`http://<host>:8000`.

> Die Docker-Variante ist vorbereitet, aber noch nicht getestet.

## Konfiguration

`config.yaml` (siehe [config.example.yaml](config.example.yaml)); einzelne
Werte lassen sich per Umgebungsvariable überschreiben:

| Einstellung | Umgebungsvariable | Standard |
|---|---|---|
| `web.listen` | `LISTEN_ADDR` | `0.0.0.0:8000` |
| `storage.path` | `DB_PATH` | `data/iperf3-tracker.db` |
| `auth.secret_key` | `SECRET_KEY` | beim ersten Start zufällig erzeugt |
| `scheduler.enabled` | `SCHEDULER_ENABLED` | `true` |
| `iperf.path` | `IPERF3_PATH` | automatisch (PATH, winget-Pfad) |
| `iperf.auto_install` | `IPERF3_AUTO_INSTALL` | `false` (Oberfläche fragt nach) |
| `geoip.path` | `GEOIP_PATH` | `geoip/GeoLite2-City.mmdb` |
| `log.level` | `LOG_LEVEL` | `info` |

Relative Pfade beziehen sich auf das Verzeichnis der `config.yaml`.

## Entwicklung

```cmd
go test ./...
go run ./cmd/iperf3-tracker
```

| Verzeichnis | Inhalt |
|---|---|
| `cmd/iperf3-tracker` | Einstiegspunkt |
| `internal/web` | HTTP-API, Live-Trace (SSE), eingebettete Oberfläche (`static/`) |
| `internal/iperf` | iperf3-Ausführung und Auswertung von `--json-stream` |
| `internal/trace` | tracert/traceroute, GeoIP, Standort-Interpolation |
| `internal/scheduler` | Zeitplan je Server, Auto-Trace |
| `internal/store`, `internal/db` | SQLite (reiner Go-Treiber, kein CGO) |
| `geoip/` | GeoLite2-City-Datenbank |

Die Oberfläche folgt der Designsprache des Spherifyer und lädt Chart.js und
Leaflet per CDN; die Karte nutzt OpenStreetMap-Kacheln.

## GeoIP-Hinweis

Enthält GeoLite2-Daten von MaxMind, verfügbar unter
https://www.maxmind.com.
