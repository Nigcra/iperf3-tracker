# Portierungsplan: iperf3-Tracker → Go (inkl. neuer Oberfläche im Spherifyer-Stil)

Ziel: Das bestehende Python/FastAPI-Backend **und** das React-Frontend werden durch **eine** schlanke Go-Anwendung ersetzt. Die Oberfläche wird neu als Vanilla-HTML/JS geschrieben und per `//go:embed` in die Binary eingebettet. Aufbau, Technik und Design folgen dem **Spherifyer** (`..\Spherifyer`): obere Navigationsleiste, Navy-Design mit semantischen CSS-Tokens, Hell/Dunkel-Umschaltung und Chart.js.

> **Stand 2026-10-08 (Branch `go-port`):**
>
> | Phase | Stand |
> |---|---|
> | 0 – Setup | ✅ erledigt: `go.mod`, `build.cmd`, Config (YAML + Env), Schema, `/health`, `/api/info`, CORS |
> | 1 – Auth | ✅ erledigt: bcrypt, JWT, Middleware, alle `/auth/*`-Endpunkte, Standard-Admin, Go-Tests (`internal/web/handlers_auth_test.go`) |
> | 2 – Server-CRUD | ✅ erledigt: `/api/servers` (Liste mit `enabled`/`skip`/`limit`, Detail, Anlegen, partielles Update, Löschen per Cascade), Go-Tests. Scheduler-Aufrufe folgen in Phase 7 |
> | 3 – Tests | ✅ erledigt: `/api/tests/*` (Liste mit Filtern, Detail, Start, Löschen, latest, live), Runner mit Warteschlange und Live-Status, Auswertung per `iperf3 --json-stream`, Fixture- und Runner-Tests, End-to-End gegen lokalen iperf3-Server geprüft |
> | 4 – Statistiken | ✅ erledigt: `/api/stats/dashboard`, `/api/stats/servers[/{id}]` als eine Aggregations-Abfrage statt 7 Abfragen je Server; Store- und HTTP-Tests |
> | 5 – Traceroute + GeoIP | ✅ erledigt: Paket `internal/trace` (tracert/traceroute, Reverse-DNS, GeoLite2, Interpolation), alle Trace-Routen inkl. `/tests/{id}/trace`; Fixture-Tests (echte `tracert`-Ausgabe), End-to-End mit echter GeoIP-Datenbank geprüft |
> | 6 – Live-Trace SSE | ✅ erledigt: `/api/live-trace/stream/{destination}?token=` mit denselben Events wie bisher, End-to-End mit echtem `tracert` geprüft |
> | 7 – Scheduler | ✅ erledigt: Paket `internal/scheduler` (Intervall je Server, Übernahme aus der DB beim Start, Auto-Trace), an Server-CRUD und `/health` angebunden; Tests mit verkürzter Zeiteinheit und Race-Detector, End-to-End geprüft |
> | 8 – Admin + Public Servers | ✅ erledigt: Bereinigung von Tests und Traces, DB-Statistik, öffentliche Serverliste und Suche. **Backend-Parität erreicht:** Alle 30 Aufrufe aus `frontend/src/services/api.ts` sowie der SSE-Stream treffen eine Go-Route (automatisiert geprüft) |
> | 9–10 | offen |
>
> Abweichungen bei der Umsetzung:
> - Das Schema stammt aus den **SQLAlchemy-Modellen** (`models.py`) und nicht aus den Migrationen. Eine frische Python-DB entsteht per `create_all` aus den Modellen. Unterschied zur Migration 002: `traces.test_id` ist nullable und nicht `UNIQUE`.
> - Die Auth-Middleware liegt in `internal/web/middleware.go` statt in `internal/auth/`, weil sie den Store braucht. `auth` bleibt frei von DB-Abhängigkeiten.
> - Das Python-Backend legt den Start-Admin mit dem Passwort `admin` an, `init-admin` dagegen mit `admin123`. Go verwendet einheitlich `admin123`.
> - Zeitstempel werden als TEXT im festen Format `2006-01-02T15:04:05.000000Z` gespeichert. Das Format ist sortierbar, sodass Datumsfilter direkt per Stringvergleich funktionieren.
> - `detail`-Fehlermeldungen sind deutsch. Das React-Frontend wertet nur die Statuscodes aus, und die sind unverändert.
> - `servers`, `tests` und `stats` waren in Python **ohne Anmeldung** erreichbar. In Go verlangen sie ein gültiges Token. Das React-Frontend sendet es immer mit, der Ablauf bleibt also unverändert.
> - **iperf3 (Phase 3):** Download nutzt jetzt `-R`. Python hatte Download und Upload vertauscht, weil `-R` „Server sendet“ bedeutet. Die Testsperre wird direkt nach dem Test freigegeben; Python hielt sie noch 10 s länger fest. Die Live-Werte funktionieren jetzt auch unter Linux, Python lieferte sie nur unter Windows. Zusätzlich werden Bytes, CPU-Last, Retransmits (wo iperf3 sie liefert) sowie bei UDP Jitter und Verlust gespeichert; in Python waren diese Felder immer `null`. `raw_output` enthält die JSON-Zeilen statt der Textausgabe. Beim Start werden Tests, die durch einen Absturz in `pending`/`running` hängen geblieben sind, als fehlgeschlagen markiert.
> - **Statistiken (Phase 4):** `failed_tests` zählt nur echte Fehlschläge. Python rechnete `total - successful` und zählte so auch wartende und laufende Tests mit. „Heute“ (`tests_today`) bezieht sich auf die lokale Zeitzone des Servers statt auf UTC. `avg_jitter_ms` und `avg_packet_loss_percent` mitteln über beide Richtungen.
> - **Neu: UDP-Zielbandbreite.** Server-Profile haben `default_udp_bandwidth_mbps`, Tests speichern die verwendete Rate in `udp_bandwidth_mbps`. `POST /tests/run` akzeptiert optional `udp_bandwidth_mbps`; ohne Angabe gilt die Server-Vorgabe, ohne Vorgabe der iperf3-Standard von 1 Mbit/s. Erlaubt sind Werte über 0 bis 100 000 Mbit/s. `0` ist gesperrt, weil iperf3 das als „unbegrenzt“ auslegt. Der Scheduler (Phase 7) nutzt die Server-Vorgabe, die neue Oberfläche (Phase 9) bekommt Eingabefelder dafür.
> - **Traceroute (Phase 5):** Ein Hop gilt als beantwortet, sobald eine IP in der Zeile steht. Python wertete teilweise Timeouts wie `12 ms * 14 ms 1.2.3.4` als Totalausfall und war von der Sprache der Meldung abhängig. Python hatte zwei verschiedene Interpolationen (die Service-Variante übersprang Hop 1); es gibt jetzt eine, nach der gründlicheren Live-Variante. `timeout` und `count` aus `POST /traces` werden beachtet (Python ignorierte sie). Bei Zeitüberschreitung (90 s) bleiben die bisherigen Hops erhalten. `total_rtt_ms` ist die RTT bis zum Ziel statt der Summe aller Hop-RTTs. `packet_loss` je Hop bleibt `null`, weil die Werkzeuge ihn nicht liefern; Python schrieb `0`. Die Verfolgung läuft auf die aufgelöste IPv4-Adresse. GeoIP wird über `geoip.path` gesucht; übergangsweise auch unter `backend/geoip/` (bis Phase 10).
> - **Live-Trace (Phase 6):** Alle 15 s geht ein Keep-Alive-Kommentar raus, damit Proxys lange Timeout-Hops nicht abbrechen. Schließt der Client die Verbindung, wird `tracert`/`traceroute` beendet und nichts gespeichert; Python lief weiter. Bei Zeitüberschreitung mit Teilergebnis wird gespeichert und `complete` gesendet, nur ganz ohne Hops kommt `error`. `complete` enthält zusätzlich `trace_id`. Der gespeicherte Trace hat jetzt `started_at`, `completed_at` und `source_ip`, die bei Python fehlten.
> - **Scheduler (Phase 7):** Auto-Trace funktioniert jetzt. Im Python-Scheduler warf `TracerouteService(db)` einen `TypeError`, und das Ergebnis wäre ohnehin nicht gespeichert worden. Der Trace wird mit dem Test verknüpft gespeichert, auch wenn der Test fehlschlug, denn gerade dann hilft er bei der Diagnose. Wird ein Server gespeichert, ohne dass sich das Intervall ändert, läuft der Countdown weiter; Python setzte ihn bei jedem Speichern zurück. Deaktivierte Server werden ausgetragen statt bei jedem Lauf übersprungen. `Runner.Submit` liefert einen Kanal, der nach Testende geschlossen wird. Geplante Tests nutzen wie bisher `default_parallel`; `default_num_streams` bleibt ungenutzt.
> - **Admin (Phase 8):** Werden Tests bereinigt, gehen ihre Traces per Cascade mit. In Python blieben sie verwaist, weil SQLite dort ohne Foreign Keys lief. Die Trace-Bereinigung nach Alter filtert auf `created_at`; Python nutzte `started_at`, das bei Live-Traces fehlte, sodass diese nie gelöscht wurden. Negative `days` werden abgelehnt. In der Liste öffentlicher Server sind drei falsch beschriftete Einträge korrigiert (`speedtest.uztelecom.uz` war als „USA East“ bezeichnet, `speedtest.wtnet.de` als „Worldstream Netherlands“, `proof.ovh.net` als „Bouygues“), die Texte sind deutsch. Die Erreichbarkeit der Server ist nicht geprüft.
> - Noch offen: Klicktest mit dem React-Frontend im Browser (Python-Backend stoppen, `_release\iperf3-tracker.exe` auf Port 8000 starten, `npm start`). Die API ist per Go-Tests, curl und den automatisierten Routenabgleich geprüft.
>
> **Änderungen gegenüber der Fassung vom 3. Aug.:**
> - Das Frontend wird **nicht** mehr 1:1 übernommen, sondern im Spherifyer-Stil neu gebaut (Abschnitt 8).
> - `chi` entfällt zugunsten von reinem `net/http` (Go-1.22+-Routing), wie im Spherifyer.
> - Go 1.24, Projektstruktur am Repo-Root wie im Spherifyer.
> - Frischer Start mit leerer Datenbank: Keine User, passlib-Hashes oder Altdaten werden übernommen (Abschnitt 6.1).
> - Trace-Routen gegen das Frontend verifiziert, dabei einen Bug gefunden (Abschnitt 5).
> - nginx- und Frontend-Container entfallen.

---

## 1. Leitprinzipien

- **Eine Binary, eine UI.** Eine statisch gelinkte Go-Binary liefert API und Oberfläche aus (`internal/web/static/` via `embed`). Kein Node, kein npm, kein nginx im Betrieb.
- **Spherifyer als Referenz.** Für Projektlayout, `net/http`-Server, `writeJSON`-Helper, `modernc.org/sqlite`, `build.cmd`, `internal/version` (Build-Datum im Footer) und das komplette UI-Design gilt: Was Spherifyer schon löst, wird übernommen statt neu erfunden.
- **API bleibt stabil, solange React noch läuft.** Bis die neue UI fertig ist (Phase 9), bedient das Go-Backend die bestehenden API-Verträge (Pfade, `snake_case`, Statuswerte lowercase). So dient das alte React-Frontend während Phase 1–8 als Testoberfläche, und Contract-Tests gegen das Python-Backend bleiben möglich. Bereinigungen am API passieren erst danach (siehe 5.1).
- **Schlank statt Framework-schwer.** Standardbibliothek plus wenige gezielte Libraries.
- **Frischer Start, gleiches Schema.** Die Go-Anwendung startet mit einer neuen, leeren DB. Es findet keine Datenübernahme statt. Das Schema bleibt trotzdem 1:1 gleich, damit Code und Contract-Tests vergleichbar sind.

---

## 2. Technologie-Auswahl

| Bereich | Python/React (alt) | Go (neu) | Begründung |
|---|---|---|---|
| HTTP-Router | FastAPI | `net/http` `ServeMux` (Go 1.22+ Muster wie `GET /api/servers/{id}`) | Wie im Spherifyer, keine Abhängigkeit nötig. |
| DB-Treiber | aiosqlite + SQLAlchemy | `modernc.org/sqlite` | Pure Go, kein CGO, gleicher Treiber wie im Spherifyer. |
| DB-Zugriff | SQLAlchemy ORM | `database/sql` | Die Queries sind überschaubar. |
| JWT | python-jose (HS256) | `golang-jwt/jwt/v5` | HS256, Claims `sub` und `exp`. |
| Passwort-Hash | passlib pbkdf2_sha256 | `golang.org/x/crypto/bcrypt` | Standardverfahren, keine passlib-Kompatibilität (siehe 6.1). |
| Konfiguration | pydantic-settings / Env | `config.yaml` (`gopkg.in/yaml.v3`) **plus** Env-Override | YAML wie im Spherifyer, Env für Docker. |
| Scheduler | APScheduler | `time.Ticker` je Server in Goroutines | Interval-basiert, leichtgewichtig. |
| GeoIP | geoip2 | `oschwald/geoip2-golang` | Liest dasselbe `GeoLite2-City.mmdb`. |
| iperf3 | subprocess + Text-Regex | `os/exec` + `iperf3 --json-stream` (ab iperf 3.17) | Kein ausgereifter Go-Client mit iperf3-Protokoll verfügbar (go-iperf ist nur ein Wrapper, pre-v1). Strukturiertes JSON statt Regex auf Textausgabe. |
| traceroute | subprocess | `os/exec` | Die Binary bleibt Systemvoraussetzung. |
| SSE | StreamingResponse | `http.Flusher` | Nativ. |
| Logging | logging + Filter | `log/slog` | Standardbibliothek. |
| UI-Framework | React 18 + CRA + react-router | Vanilla `index.html` + `app.js`, Tabs über `showTab()` | Wie im Spherifyer. |
| Charts | recharts | Chart.js 4 (jsDelivr) | Wie im Spherifyer, inkl. `applyChartTheme()`. |
| Karte | react-leaflet | Leaflet 1.9 (jsDelivr/unpkg) | Direkt ohne Wrapper. |
| Icons | react-icons (Feather) | Inline-SVG (Lucide-Pfade) | Wie die Navigation im Spherifyer. |

---

## 3. Ziel-Projektstruktur

Die Struktur liegt am Repo-Root wie im Spherifyer. Das Python-Backend bleibt bis Phase 10 in `backend/`, React bis dahin in `frontend/`.

```
go.mod                         # module iperf3-tracker, go 1.24
build.cmd                      # nach Vorlage Spherifyer (clean/tidy/release, Build-Datum per -ldflags)
config.example.yaml
cmd/
  iperf3-tracker/
    main.go                    # Config laden, DB öffnen, Scheduler + HTTP starten, Graceful Shutdown
internal/
  version/version.go           # BuildDate (per ldflags), wie im Spherifyer
  config/config.go             # YAML + Env-Override
  db/
    db.go                      # Öffnen, PRAGMAs, Schema-Init
    schema.sql                 # aus den SQLAlchemy-Modellen (embed)
  model/model.go               # Structs + Enums mit json-Tags (snake_case)
  auth/
    auth.go                    # bcrypt, JWT erstellen/prüfen
    (Middleware: siehe web/middleware.go)
  store/                       # users.go, servers.go, tests.go, traces.go
  iperf/
    runner.go                  # Warteschlange, iperf3 ausführen, Live-Status
    stream.go                  # Auswertung --json-stream
    binary.go                  # iperf3 finden, Version prüfen
    testdata/*.jsonl           # echte Mitschnitte (iperf 3.21)
  trace/
    traceroute.go              # tracert/traceroute je runtime.GOOS
    geoip.go                   # mmdb-Reader
    interpolate.go             # GeoIP-Interpolation
    parser_test.go
  scheduler/scheduler.go       # Ticker je Server, Auto-Trace
  web/
    server.go                  # Routen, writeJSON, Static-Embed, Index
    middleware.go              # Bearer-Auth, Admin-Guard, Token-Prüfung für SSE
    handlers_*.go              # auth, servers, tests, stats, traces, livetrace, admin, public
    static/
      index.html               # Layout, CSS-Tokens, Seiten-Container
      app.js                   # Logik: API-Client, Tabs, Charts, Karte
geoip/GeoLite2-City.mmdb
Dockerfile
```

---

## 4. Datenbank-Schema (1:1)

Das Schema entspricht den SQLAlchemy-Modellen (inkl. `migrations/001`–`004`) und wird beim Start per `CREATE TABLE IF NOT EXISTS` in einer **neuen** DB-Datei angelegt (Standard `data/iperf3-tracker.db`). Die alte `backend/iperf_tracker.db` wird nicht angefasst.

Tabellen: `users`, `servers`, `tests`, `traces`, `trace_hops`. Spalten, Typen, Defaults, Indizes und Foreign Keys bleiben exakt erhalten, insbesondere:
- `servers.default_num_streams` (001), `servers.auto_trace_enabled` (003), `trace_hops.geoip_interpolated` (004)
- PRAGMAs pro Connection: `foreign_keys=ON`, `journal_mode=WAL`, `busy_timeout=5000`
- Enums werden als `TEXT` gespeichert: `tcp|udp`, `download|upload|bidirectional`, `pending|running|completed|failed`
- Zeitstempel in UTC, JSON-Ausgabe im RFC3339-Format mit `Z`

---

## 5. API-Verträge

Alle Endpunkte liegen unter `/api`. Auth läuft über `Authorization: Bearer <JWT>`, sofern ein Endpunkt nicht als Public markiert ist.

### Auth (`/api/auth`)
- `POST /auth/login` (Public): liefert `{access_token, token_type:"bearer", user}` und aktualisiert `last_login`
- `POST /auth/register` (Admin): liefert User
- `GET /auth/me`: liefert User
- `GET /auth/users` (Admin): liefert User[]
- `DELETE /auth/users/{user_id}` (Admin, Löschen des eigenen Kontos nicht erlaubt): liefert `{message}`
- `POST /auth/init-admin` (Public, nur wenn keine gültigen User existieren)

### Servers (`/api/servers`)
- `GET /servers` (`enabled?`, `skip=0`, `limit=100`), `GET /servers/{id}`
- `POST /servers` (201) und Scheduling starten, `PUT /servers/{id}` (partial) und Scheduling aktualisieren, `DELETE /servers/{id}` (204) und Scheduling beenden

### Tests (`/api/tests`)
- `GET /tests` (`server_id?`, `status?`, `from_date?`, `to_date?`, `skip`, `limit`), absteigend nach `created_at`
- `GET /tests/{id}`: TestDetail inkl. `server` und `raw_output`
- `POST /tests/run`: 201, sofort `pending`, die Ausführung läuft im Hintergrund
- `DELETE /tests/{id}` (204), `GET /tests/server/{server_id}/latest` (Test oder null)
- `GET /tests/{id}/live`: Live-Status (Polling)

### Stats (`/api/stats`)
- `GET /stats/dashboard`, `GET /stats/servers`, `GET /stats/servers/{id}`

### Traces — gegen `frontend/src/services/api.ts` verifiziert
Diese Routen nutzt das Frontend tatsächlich:
- `POST /traces` (201), `GET /traces/{id}`, `GET /traces?limit=`, `GET /traces/test/{test_id}`, `DELETE /traces/{id}`

Diese Routen bietet nur das Backend an, das Frontend nutzt sie nicht: `POST|GET|DELETE /tests/{id}/trace`, `GET /traces/recent`. Sie werden trotzdem portiert, weil der Scheduler und Auto-Trace sie intern nutzen bzw. weil sie für Contract-Tests gebraucht werden. Gestrichen werden sie erst in 5.1.

> **Bug im Bestand:** `PeeringMap.tsx:80` ruft `api.getTrace(testId)` auf, also `GET /traces/{testId}`, und übergibt dabei eine **Test-ID als Trace-ID**. Richtig wäre `GET /tests/{id}/trace` oder `GET /traces/test/{id}`. Die neue UI nutzt die richtige Route. Im React-Bestand wird das nicht mehr korrigiert.

### Live-Trace SSE
- `GET /live-trace/stream/{destination}?token=<JWT>` liefert `text/event-stream` mit den Events `start`, `hop`, `interpolation_complete`, `complete` und `error`

### Admin (`/api/admin`)
- `DELETE /admin/cleanup/tests` (`days?`, `server_id?`, `all?`), `DELETE /admin/cleanup/traces` (`days?`, `all?`), `GET /admin/stats/database`

### Public Servers (Public)
- `GET /public-servers` (10 kuratierte Server als Konstante), `GET /public-servers/search?query=`

### Root/Health
- `GET /` liefert jetzt die **UI** (`index.html`), nicht mehr JSON. Das bisherige `{name, version, docs}` wandert nach `GET /api/info`, ergänzt um `build_date`.
- `GET /health` liefert `{status:"healthy", scheduler_running}`
- `/static/*` liefert Assets aus dem Embed

### 5.1 API-Bereinigung nach dem UI-Wechsel (Phase 10)
Sobald React entfernt ist, gilt:
- Nicht genutzte Doppelrouten werden entfernt (`/traces/recent` geht in `/traces?limit=` auf).
- CORS wird auf Same-Origin reduziert, weil UI und API vom selben Origin kommen.
- Die hartkodierte Port-8000-Ermittlung im Frontend (`api.ts:14`, `PeeringMap.tsx:147`) entfällt. Die neue UI nutzt relative URLs (`/api/...`).

---

## 6. Kritische Kompatibilitäts-Details

1. **Passwort-Hash:** bcrypt, frischer Start (siehe 6.1).
2. **JWT:** HS256, Payload `{"sub": <username>, "exp": <unix>}`, 24 h gültig. `secret_key` kommt aus Config oder Env. Fehlt er, wird beim ersten Start ein zufälliger Key erzeugt und in `config.yaml` geschrieben (nicht mehr hartkodiert).
3. **Nullbare Felder:** werden über Pointer (`*float64`, `*time.Time`) abgebildet und als `null` serialisiert.
4. **Statuswerte** bleiben lowercase.
5. **Sequenzielle Testausführung:** Ein globaler Semaphore (`chan struct{}` mit Kapazität 1) stellt sicher, dass nur ein iperf3-Test gleichzeitig läuft.
6. **Live-Status:** `map[int]LiveStatus` hinter `sync.RWMutex`.
7. **iperf3-Aufruf:** `iperf3 -c <host> -p <port> -t <dur> -P <streams> -i 1 --json-stream --forceflush`, ergänzt um `-u`, `-R` (**Download**) oder `--bidir`. Ausgewertet werden die JSON-Ereignisse `interval` (Live-Werte), `end` (Ergebnis) und `error`. Abweichungen zu Python siehe Stand oben.
8. **traceroute/tracert-Parser:** plattformabhängig über `runtime.GOOS`, gleiche Zeilenformate wie bisher.
9. **GeoIP-Interpolation** 1:1: gleiche /24-Nachbarschaft (±5 Hops), Fallback auf den nächsten Hop, ±0,05° Zufalls-Offset, `geoip_interpolated=true`.

### 6.1 Auth: frischer Start
Es werden keine Benutzer und keine passlib-Hashes übernommen. Die DB ist neu und leer.
- Sind beim Start keine User vorhanden, wird `admin`/`admin123` mit bcrypt angelegt und die Warnung „Passwort ändern“ geloggt. Das entspricht dem bisherigen Verhalten aus Commit `fa00d3a`. `POST /auth/init-admin` bleibt für dieselbe Situation erhalten.
- Es gibt keine Erkennung von Alt-Hashes und keinen Migrationscode.

---

## 7. Umsetzungsphasen

Phase 1–8 bauen das Backend und werden jeweils gegen das **unveränderte React-Frontend** geprüft. Phase 9 ersetzt die UI. Phase 10 räumt auf.

### Phase 0 – Setup
- `go.mod` (Go 1.24), `build.cmd` nach Spherifyer-Vorlage, `internal/version`
- Config (YAML + Env), SQLite öffnen, PRAGMAs setzen, Schema-Init
- `/health`, `/api/info`, CORS (vorerst `*`, weil React noch auf `:3000` läuft)

### Phase 1 – Auth
- bcrypt, JWT, Bearer-Middleware, Admin-Guard, Standard-Admin beim ersten Start (6.1)
- Endpunkte `/auth/*`
- **Meilenstein:** Login mit React gegen das Go-Backend klappt (frische DB, `admin`/`admin123`).

### Phase 2 – Server-CRUD
- **Meilenstein:** `ServerManager` funktioniert vollständig.

### Phase 3 – Tests
- CRUD, Filter, Pagination, iperf3-Runner, JSON-Stream-Auswertung mit Fixture-Tests, Semaphore, Live-Status
- **Meilenstein:** `TestRunner`, `LiveTestDisplay` und der Dashboard-Live-Test funktionieren.

### Phase 4 – Statistiken
- **Meilenstein:** Dashboard-Kacheln und Charts zeigen dieselben Werte wie mit dem Python-Backend.

### Phase 5 – Traceroute + GeoIP
- Runner, Parser mit Fixtures, GeoIP, Interpolation, Trace-CRUD

### Phase 6 – Live-Trace SSE
- **Meilenstein:** Die Live-Traceroute in der `PeeringMap` funktioniert.

### Phase 7 – Scheduler
- Ticker je Server, Synchronisierung beim Start, Auto-Trace, `scheduler_running`

### Phase 8 – Admin + Public Servers
- **Meilenstein:** Funktionsparität des Backends ist erreicht. Das Python-Backend kann abgeschaltet werden.

### Phase 9 – Neue Oberfläche im Spherifyer-Stil (Details in Abschnitt 8)
- 9a: Grundgerüst: `index.html` mit Tokens, Header, Tabs und Footer, außerdem `app.js` mit API-Client, Theme, Tabs und Login
- 9b: Dashboard
- 9c: Test starten + Live-Anzeige
- 9d: Server
- 9e: Peering-Map (Leaflet, SSE, Topologie)
- 9f: Admin
- **Meilenstein:** Alle Funktionen des React-Frontends sind in der neuen UI verfügbar.

### Phase 10 – Deployment & Abschluss
- `Dockerfile` als Multi-Stage-Build (`golang:1.24` → `debian:trixie-slim` (iperf3 3.18; bookworm hat nur 3.12 ohne `--json-stream`) mit `iperf3`, `traceroute` und `ca-certificates`)
- `docker-compose.yml` mit **einem** Service: Port 8000, Volumes für DB und GeoIP. Der Frontend-Service und nginx entfallen.
- `start.bat` wird ersetzt durch `build.cmd` und den direkten Aufruf der `.exe`
- Optional: Betrieb als Windows-Dienst, nach dem Muster von `internal/svc` im Spherifyer
- `backend/` (Python) und `frontend/` (React) entfernen, API-Bereinigung (5.1), README und Screenshots aktualisieren

---

## 8. Oberfläche: Angleichung an den Spherifyer

### 8.1 Übernahme 1:1 aus `Spherifyer/internal/web/static/index.html`
- **CSS-Tokens:** der komplette `:root`- und `:root[data-theme="light"]`-Block (`--accent`, `--ok/--warn/--crit`, `--bg-grad`, `--surface`, `--border`, `--heading/--text/--muted/--faint`, `--card-shadow`, `--grid` …). Der Hintergrund-Verlauf liegt in `body::before`.
- **Theme:** Das Inline-Script im `<head>` setzt `data-theme` vor dem ersten Paint (kein Aufflackern). `toggleTheme()` und `applyChartTheme()` werden aus `app.js` übernommen. Standard ist dunkel. Der localStorage-Key lautet `iperf-theme`.
- **Komponenten-Klassen:** `.header`/`.header-inner`/`.brand`, `.nav` mit Icon-Buttons, `.actions`, `.btn`, `.icon-btn`, `.status-menu`/`.status-panel`, `.view-switch`/`.view-btn`, `.summary`/`.sum-card`, `.section`/`.section-title`, `.ds-grid`/`.ds-card`, `.chart-box`, `.grid-2`, `.tbl-wrap` + Tabellen, `.bar-bg`/`.bar-fill`, `.sev-badge`, `.badge-type`, `.ds-toggle-label`, Formularfelder, Footer mit Build-Datum
- **Seitenmechanik:** `.page`/`.page.active` mit `showTab(id, btn)`. Der aktive Tab wird zusätzlich im URL-Hash gespeichert (`#dashboard`), damit Reload und Lesezeichen funktionieren.

### 8.2 Abweichungen und Ergänzungen für den iperf3-Tracker
- **Brand:** „iperf3-Tracker“. Das Brand-Icon ist ein Tacho-SVG, eingebettet im selben 34-px-Rahmen wie das Spherifyer-Icon.
- **Login-Seite:** Sie fehlt im Spherifyer (dort gibt es keine Auth). Neu ist eine zentrierte `.ds-card` auf dem Verlaufshintergrund. Der Token liegt in `localStorage`. Bei einer 401-Antwort zeigt der API-Client wieder den Login.
- **Header-Aktionen:** Status-Punkt (Scheduler läuft / Test läuft / Fehler) mit Detail-Panel wie im Spherifyer, daneben Theme-Umschalter und ein Benutzermenü (Name, Admin-Badge, Abmelden). Statt des Refresh-Buttons gibt es einen Button „Test starten“, der einen Schnellstart-Dialog öffnet.
- **Modal-Dialog** (Server anlegen/bearbeiten, Schnellstart): Diesen gibt es im Spherifyer nicht. Er wird neu im selben Token-Stil gebaut (`--menu-bg`, `--border`, `--card-shadow`, 16 px Radius).
- **Sprache:** Die UI wird **deutsch** wie im Spherifyer (bisher war sie englisch).

### 8.3 Seitenzuordnung (React → neue Tabs)

| Tab | Ersetzt | Inhalt im Spherifyer-Stil |
|---|---|---|
| **Dashboard** | `Dashboard.tsx` (832 Z.) | `.summary` mit 5 `.sum-card` (Server, Tests, Ø Download, Ø Upload, unter Schwellwert). `.view-switch` 1 h / 24 h / Woche / Monat. Schwellwert-Eingaben als `.ds-toggle-label`. `.grid-2` mit zwei `.chart-box` (Download/Upload über Zeit, Chart.js-Linien je Server, Schwellwert als gestrichelter Datensatz). Darüber erscheint die Live-Test-Anzeige, solange ein Test läuft. Einstellungen werden im localStorage gespeichert. |
| **Test starten** | `TestRunner.tsx` + `LiveTestDisplay.tsx` | Formular in einer `.ds-card`, bei UDP mit Feld „Zielbandbreite (Mbit/s)“, vorbelegt mit der Server-Vorgabe. Die Live-Anzeige zeigt einen Fortschrittsbalken (`.bar-bg`/`.bar-fill`) und aktuelle Mbit/s und pollt alle 500 ms. Darunter stehen die letzten Tests als sortierbare Tabelle mit `.sev-badge` für den Status. |
| **Peering-Map** | `PeeringMap.tsx`, `LiveMap.tsx`, `NetworkTopology.tsx` | Die Leaflet-Karte füllt eine `.chart-box`. Kachelquelle je Theme (hell/dunkel) wird beim `toggleTheme()` umgeschaltet. Links steht eine Trace-Liste je Server, die Live-Trace per `EventSource` läuft als Overlay. Bei rein privaten Hops erscheint eine SVG-Topologie, die sich an der Topologie-Ansicht im Spherifyer orientiert (`#topoContainer`, Tooltip, Legende). |
| **Server** | `ServerManager.tsx` | `.ds-grid` mit einer `.ds-card` je Server. Badges zeigen TCP/UDP, Richtung, Intervall und Auto-Trace. Das Modal enthält das Feld „UDP-Zielbandbreite (Mbit/s)“. Dazu kommen Aktionen (Test, Traceroute, Bearbeiten, Löschen) und Statistiken aus `/stats/servers`. Das Modal enthält eine Auswahl öffentlicher Server. |
| **Admin** (nur für Admins) | `AdminPanel.tsx` | `.sum-card`s mit DB-Statistik, eine Benutzertabelle mit Anlegen/Löschen und Bereinigung (Tests/Traces nach Tagen/Server) mit Bestätigung. |

### 8.4 Externe Bibliotheken
Wie im Spherifyer per CDN: Chart.js 4.4 (jsDelivr) und Leaflet 1.9 (jsDelivr, JS + CSS).
> Offene Entscheidung: Für den Betrieb ohne Internet könnten die Bibliotheken unter `static/vendor/` eingebettet werden (ca. 350 KB). Die Kartenkacheln brauchen ohnehin Internet. **Empfehlung:** CDN wie im Spherifyer, Vendoring nur bei Bedarf.

### 8.5 Umfang
`app.js` wird voraussichtlich 1.500–2.000 Zeilen haben (Spherifyer: 1.391), `index.html` etwa 600 Zeilen. Am meisten Aufwand macht die Peering-Map (Leaflet, SSE, Topologie). Alle anderen Seiten sind im Wesentlichen Karten, Tabellen und Charts, für die der Spherifyer schon Muster liefert.

---

## 9. Test- & Abnahmestrategie

1. **Parser-Fixtures** (Go-Unit-Tests): echte `--json-stream`-Mitschnitte (TCP/UDP × download/upload/bidir, Fehlerfall) und tracert/traceroute-Ausgaben (Windows/Linux).
2. **Contract-Tests** (Phase 1–8): Gleiche Requests gehen an Python (`:8001`) und Go (`:8000`), die JSON-Bodies werden verglichen. Bekannte Abweichung: Zeitstempel haben jetzt ein `Z`-Suffix.
3. **Smoke-Test mit React** nach jeder Backend-Phase.
4. **UI-Abnahme** in Phase 9: jede Seite in Hell und Dunkel, schmale Fensterbreite und Funktionsvergleich mit dem React-Stand.
5. **Erststart:** Ohne DB-Datei starten. Schema und Admin müssen angelegt werden, ein Login muss möglich sein. Ein zweiter Start darf nichts verändern.

---

## 10. Risiken & offene Punkte

| Risiko | Auswirkung | Gegenmaßnahme |
|---|---|---|
| iperf3 älter als 3.17 auf dem Zielsystem | Tests schlagen fehl | Versionsprüfung beim Start mit Warnung im Log; Docker-Image auf trixie |
| UI-Neubau dauert länger als das Backend | Lange Phase mit zwei Frontends | React bleibt bis Phase 9 voll funktionsfähig; die neue UI geht Tab für Tab live (`/`), React läuft bis dahin parallel auf `:3000` |
| Funktionen gehen beim UI-Neubau verloren | Regressionen | Checkliste je React-Komponente (8.3) als Abnahmeliste |
| CDN nicht erreichbar | Keine Charts/Karte | Vendoring-Option (8.4) |
| Zeitzonen und Zeitformat | Falsche Zeiten | Durchgängig UTC + RFC3339, die UI formatiert mit `toLocaleString('de-DE')` |
| SQLite-Locking | „database is locked“ | WAL + `busy_timeout`, sequenzielle Testausführung |

---

## 11. Zusammenfassung

Die Portierung ist gut machbar. Am Backend kostet vor allem die exakte Nachbildung des iperf3- und traceroute-Parsings sowie der GeoIP-Interpolation Aufwand. Durch den Wechsel auf eine eingebettete Vanilla-UI im Spherifyer-Stil kommt der Neubau des Frontends dazu. Dafür entfallen Node, npm, nginx und ein Container, und beide Tools teilen dieselbe Designsprache und Codebasis-Struktur.

**Nächster Schritt:** Phase 0 + 1 (Setup, Schema, Standard-Admin, JWT-Auth) und Login-Test mit dem bestehenden React-Frontend gegen eine frische DB.
