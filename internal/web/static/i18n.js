'use strict';

// ---------- Sprachen ----------

// STRINGS hält je Schlüssel die deutsche und die englische Fassung. Platzhalter
// in geschweiften Klammern ({name}) ersetzt t() aus dem übergebenen Objekt.
const LANGS = ['de', 'en'];
const STRINGS = {
  // Allgemein
  'cancel':               ['Abbrechen', 'Cancel'],
  'close':                ['Schließen', 'Close'],
  'save':                 ['Speichern', 'Save'],
  'delete':               ['Löschen', 'Delete'],
  'later':                ['Später', 'Later'],
  'unknown':              ['unbekannt', 'unknown'],
  'unknownError':         ['Unbekannter Fehler', 'Unknown error'],
  'lang.choose':          ['Sprache wählen', 'Choose language'],

  // Anmeldung und Benutzermenü
  'login.user':           ['Benutzername', 'Username'],
  'login.pass':           ['Passwort', 'Password'],
  'login.submit':         ['Anmelden', 'Sign in'],
  'login.failed':         ['Benutzername oder Passwort falsch.', 'Wrong username or password.'],
  'login.defaultPw':      ['Das Standardpasswort ist noch aktiv – bitte über das Benutzermenü ändern.',
                           'The default password is still active – please change it via the user menu.'],
  'login.unreachable':    ['Dienst nicht erreichbar: {msg}', 'Service unreachable: {msg}'],
  'user.title':           ['Benutzer', 'User'],
  'user.menu':            ['Benutzermenü', 'User menu'],
  'user.changePassword':  ['Passwort ändern', 'Change password'],
  'user.logout':          ['Abmelden', 'Sign out'],

  // Kopfzeile
  'nav.dashboard':        ['Dashboard', 'Dashboard'],
  'nav.tests':            ['Tests', 'Tests'],
  'nav.peering':          ['Peering-Map', 'Peering map'],
  'nav.servers':          ['Server', 'Servers'],
  'nav.admin':            ['Admin', 'Admin'],
  'theme.title':          ['Hell / Dunkel umschalten', 'Toggle light / dark'],
  'theme.aria':           ['Theme umschalten', 'Toggle theme'],
  'test.start':           ['Test starten', 'Start test'],
  'test.startNew':        ['Neuen Test starten', 'Start a new test'],

  // Status
  'status.show':          ['Status anzeigen', 'Show status'],
  'status.service':       ['Dienst:', 'Service:'],
  'status.reachable':     ['erreichbar', 'reachable'],
  'status.scheduler':     ['Scheduler:', 'Scheduler:'],
  'status.on':            ['aktiv', 'active'],
  'status.off':           ['aus', 'off'],
  'status.running':       ['Laufende Tests:', 'Running tests:'],
  'status.waiting':       ['(+{n} wartend)', '(+{n} waiting)'],
  'status.installing':    ['wird installiert …', 'installing …'],
  'status.unavailable':   ['nicht verfügbar', 'not available'],
  'status.install':       ['iperf3 installieren', 'Install iperf3'],
  'status.titleBroken':   ['iperf3 nicht verfügbar – Details anzeigen', 'iperf3 not available – show details'],
  'status.titleInstall':  ['iperf3 wird installiert', 'Installing iperf3'],
  'status.titleTest':     ['Test läuft', 'Test running'],
  'status.titleOk':       ['Status: OK', 'Status: OK'],
  'status.down':          ['Dienst nicht erreichbar', 'Service unreachable'],

  // iperf3-Installation
  'setup.installed':      ['iperf3 {version} installiert – Tests können jetzt laufen.', 'iperf3 {version} installed – tests can run now.'],
  'setup.failed':         ['iperf3-Installation fehlgeschlagen: {msg}', 'iperf3 installation failed: {msg}'],
  'setup.missingUser':    ['iperf3 ist auf dem Server nicht verfügbar – Tests schlagen fehl. Bitte einen Administrator informieren.',
                           'iperf3 is not available on the server – tests will fail. Please inform an administrator.'],
  'setup.question':       ['iperf3 installieren?', 'Install iperf3?'],
  'setup.text':           ['iperf3 wurde auf diesem System nicht gefunden – ohne iperf3 laufen keine Tests.\n\nSoll es jetzt über den Paketmanager installiert werden? Ausgeführt wird:\n{cmd}',
                           'iperf3 was not found on this system – no tests can run without it.\n\nInstall it now via the package manager? This runs:\n{cmd}'],
  'setup.install':        ['Installieren', 'Install'],
  'setup.started':        ['iperf3 wird installiert – das kann einige Minuten dauern …', 'Installing iperf3 – this may take a few minutes …'],
  'setup.impossible':     ['Installation nicht möglich: {msg}', 'Installation not possible: {msg}'],

  // Gemeinsame Begriffe
  'dir.download':         ['Download', 'Download'],
  'dir.upload':           ['Upload', 'Upload'],
  'dir.bidirectional':    ['Bidirektional', 'Bidirectional'],
  'streams.one':          ['{n} Stream', '{n} stream'],
  'streams.other':        ['{n} Streams', '{n} streams'],
  'st.pending':           ['Wartet', 'Pending'],
  'st.running':           ['Läuft', 'Running'],
  'st.completed':         ['Abgeschlossen', 'Completed'],
  'st.failed':            ['Fehlgeschlagen', 'Failed'],
  'f.server':             ['Server', 'Server'],
  'f.protocol':           ['Protokoll', 'Protocol'],
  'f.direction':          ['Richtung', 'Direction'],
  'f.duration':           ['Testdauer (s)', 'Duration (s)'],
  'f.parallel':           ['Parallele Streams', 'Parallel streams'],
  'f.udp':                ['UDP-Zielbandbreite (Mbit/s)', 'UDP target bandwidth (Mbit/s)'],
  'f.udpPlaceholder':     ['leer = iperf3-Standard (1 Mbit/s)', 'empty = iperf3 default (1 Mbit/s)'],
  'badge.every':          ['alle {n} min', 'every {n} min'],
  'badge.disabled':       ['deaktiviert', 'disabled'],
  'badge.active':         ['aktiv', 'active'],
  'badge.okPct':          ['{n} % OK', '{n} % OK'],
  'udp.rate':             ['UDP-Zielrate', 'UDP target rate'],
  'udp.default':          ['Standard (1 Mbit/s)', 'Default (1 Mbit/s)'],

  // Dashboard
  'dash.history':         ['Bandbreite im Verlauf', 'Bandwidth history'],
  'dash.minDown':         ['Min. Download', 'Min. download'],
  'dash.minUp':           ['Min. Upload', 'Min. upload'],
  'dash.range7d':         ['7 Tage', '7 days'],
  'dash.range30d':        ['30 Tage', '30 days'],
  'dash.servers':         ['Server', 'Servers'],
  'dash.loadFailed':      ['Dashboard konnte nicht geladen werden: {msg}', 'Could not load the dashboard: {msg}'],
  'dash.activeCount':     ['{n} aktiv', '{n} active'],
  'dash.today':           ['{n} heute', '{n} today'],
  'dash.avgDown':         ['Ø Download', 'Avg. download'],
  'dash.avgUp':           ['Ø Upload', 'Avg. upload'],
  'dash.allTests':        ['Mbit/s · alle Tests', 'Mbit/s · all tests'],
  'dash.below':           ['Unter Schwellwert', 'Below threshold'],
  'dash.belowPct':        ['{n} % im Zeitraum', '{n} % in range'],
  'dash.noChart':         ['Diagramme nicht verfügbar (Chart.js nicht geladen)', 'Charts not available (Chart.js not loaded)'],
  'dash.threshold':       ['Schwellwert', 'Threshold'],
  'dash.noData':          ['Keine Messwerte im Zeitraum', 'No measurements in this range'],
  'dash.noServers':       ['Noch keine Server angelegt.', 'No servers yet.'],
  'dash.createServer':    ['Server anlegen', 'Add server'],
  'dash.toMap':           ['Traceroute auf der Peering-Map', 'Traceroute on the peering map'],
  'dash.quickTest':       ['Schnelltest mit den Vorgaben des Servers', 'Quick test with the server defaults'],
  'dash.avgPrefix':       ['Ø ', 'Avg. '],
  'dash.jitter':          ['Ø Jitter', 'Avg. jitter'],
  'dash.loss':            ['Ø Paketverlust', 'Avg. packet loss'],
  'dash.tests':           ['Tests', 'Tests'],
  'dash.failedShort':     ['({n} fehlgeschl.)', '({n} failed)'],
  'dash.lastTest':        ['Letzter Test', 'Last test'],
  'dash.testStarted':     ['Test für {name} gestartet', 'Test for {name} started'],
  'test.startFailed':     ['Test konnte nicht gestartet werden: {msg}', 'Could not start the test: {msg}'],

  // Server
  'servers.title':        ['Server-Profile', 'Server profiles'],
  'servers.add':          ['+ Server hinzufügen', '+ Add server'],
  'servers.loadFailed':   ['Server konnten nicht geladen werden: {msg}', 'Could not load servers: {msg}'],
  'servers.empty':        ['Noch keine Server angelegt. Über „Server hinzufügen“ einen eigenen oder einen öffentlichen iperf3-Server eintragen.',
                           'No servers yet. Use “Add server” to enter your own or a public iperf3 server.'],
  'servers.edit':         ['Bearbeiten', 'Edit'],
  'servers.disable':      ['Deaktivieren', 'Disable'],
  'servers.enable':       ['Aktivieren', 'Enable'],
  'servers.noSchedule':   ['kein Zeitplan', 'no schedule'],
  'servers.autoTrace':    ['Auto-Trace', 'Auto trace'],
  'servers.duration':     ['Testdauer', 'Duration'],
  'servers.streams':      ['Streams', 'Streams'],
  'servers.formAdd':      ['Server hinzufügen', 'Add server'],
  'servers.formEdit':     ['Server bearbeiten', 'Edit server'],
  'servers.public':       ['Öffentlichen Server übernehmen', 'Use a public server'],
  'servers.own':          ['– eigenen Server eintragen –', '– enter your own server –'],
  'servers.name':         ['Name *', 'Name *'],
  'servers.host':         ['Host *', 'Host *'],
  'servers.port':         ['Port', 'Port'],
  'servers.desc':         ['Beschreibung', 'Description'],
  'servers.defaults':     ['Testvorgaben', 'Test defaults'],
  'servers.schedule':     ['Zeitplan', 'Schedule'],
  'servers.scheduleCheck':['Regelmäßig testen', 'Test regularly'],
  'servers.interval':     ['Intervall (min)', 'Interval (min)'],
  'servers.autoTraceCheck':['Nach jedem geplanten Test eine Traceroute aufzeichnen', 'Record a traceroute after every scheduled test'],
  'servers.enabledCheck': ['Server aktiv (inaktive Server werden weder geplant noch getestet)', 'Server active (inactive servers are neither scheduled nor tested)'],
  'servers.required':     ['Name und Host sind Pflichtfelder.', 'Name and host are required.'],
  'servers.saved':        ['Server „{name}“ gespeichert', 'Server “{name}” saved'],
  'servers.disabled':     ['„{name}“ deaktiviert', '“{name}” disabled'],
  'servers.enabled':      ['„{name}“ aktiviert', '“{name}” enabled'],
  'servers.deleteQ':      ['Server löschen?', 'Delete server?'],
  'servers.deleteText':   ['„{name}“ wird mit allen zugehörigen Tests und Traces endgültig gelöscht.',
                           '“{name}” will be permanently deleted along with all its tests and traces.'],
  'servers.deleted':      ['„{name}“ gelöscht', '“{name}” deleted'],

  // Tests
  'tests.parallelHint':   ['⚠ Viele öffentliche Server erlauben höchstens 4 parallele Streams.', '⚠ Many public servers allow at most 4 parallel streams.'],
  'tests.history':        ['Testverlauf', 'Test history'],
  'tests.allServers':     ['Alle Server', 'All servers'],
  'tests.allStatus':      ['Alle Status', 'All statuses'],
  'tests.time':           ['Zeitpunkt', 'Time'],
  'tests.status':         ['Status', 'Status'],
  'tests.loadMore':       ['Mehr laden', 'Load more'],
  'tests.noActive':       ['Keine aktiven Server – bitte unter „Server“ anlegen', 'No active servers – add one under “Servers”'],
  'tests.started':        ['Test gestartet', 'Test started'],
  'tests.result':         ['Ergebnis: {name}', 'Result: {name}'],
  'tests.badgeFailed':    ['FEHLGESCHLAGEN', 'FAILED'],
  'tests.badgeDone':      ['ABGESCHLOSSEN', 'COMPLETED'],
  'tests.badgeWaiting':   ['WARTET', 'WAITING'],
  'tests.badgeReady':     ['BEREIT', 'READY'],
  'tests.details':        ['Details', 'Details'],
  'tests.waitingSlot':    ['Wartet auf freien Testplatz', 'Waiting for a free test slot'],
  'tests.runningAgainst': ['Läuft gegen {name}', 'Running against {name}'],
  'tests.elapsed':        ['{e} s vergangen · {r} s verbleibend', '{e} s elapsed · {r} s remaining'],
  'tests.liveTitle':      ['Live-Anzeige', 'Live view'],
  'tests.noTest':         ['Kein Test aktiv.', 'No test running.'],
  'tests.noTestHint':     ['Starte links einen Test – laufende geplante Tests erscheinen hier automatisch.',
                           'Start a test on the left – running scheduled tests appear here automatically.'],
  'tests.loadFailed':     ['Tests konnten nicht geladen werden: {msg}', 'Could not load tests: {msg}'],
  'tests.none':           ['Keine Tests gefunden', 'No tests found'],
  'tests.created':        ['Angelegt', 'Created'],
  'tests.startedAt':      ['Gestartet', 'Started'],
  'tests.runtime':        ['Laufzeit', 'Runtime'],
  'tests.planned':        ['{runtime} (geplant {n} s)', '{runtime} (planned {n} s)'],
  'tests.bytesDown':      ['Datenmenge Download', 'Data volume download'],
  'tests.bytesUp':        ['Datenmenge Upload', 'Data volume upload'],
  'tests.jitter':         ['Jitter', 'Jitter'],
  'tests.loss':           ['Paketverlust', 'Packet loss'],
  'tests.retransmits':    ['Retransmits', 'Retransmits'],
  'tests.cpu':            ['CPU-Last (lokal)', 'CPU load (local)'],
  'tests.raw':            ['Rohausgabe von iperf3', 'Raw iperf3 output'],
  'tests.delete':         ['Test löschen', 'Delete test'],
  'tests.deleteQ':        ['Test löschen?', 'Delete test?'],
  'tests.deleteText':     ['Test #{id} gegen „{name}“ wird samt Traces endgültig gelöscht.',
                           'Test #{id} against “{name}” will be permanently deleted along with its traces.'],
  'tests.deleted':        ['Test #{id} gelöscht', 'Test #{id} deleted'],

  // Peering-Map
  'peering.start':        ['Traceroute starten', 'Start traceroute'],
  'peering.abort':        ['Abbrechen', 'Cancel'],
  'peering.saved':        ['Gespeicherte Traces', 'Saved traces'],
  'peering.ip':           ['IP-Adresse', 'IP address'],
  'peering.hostname':     ['Hostname', 'Hostname'],
  'peering.location':     ['Standort', 'Location'],
  'peering.noLeaflet':    ['Karte nicht verfügbar (Leaflet konnte nicht geladen werden). Der Pfad wird unten angezeigt.',
                           'Map not available (Leaflet could not be loaded). The path is shown below.'],
  'peering.loadFailed':   ['Traces konnten nicht geladen werden: {msg}', 'Could not load traces: {msg}'],
  'peering.traceCount':   ['{name} ({n} Traces)', '{name} ({n} traces)'],
  'peering.noServers':    ['Keine Server angelegt', 'No servers yet'],
  'peering.addFirst':     ['Zuerst unter „Server“ einen Server anlegen.', 'Add a server under “Servers” first.'],
  'peering.noTraces':     ['Noch keine Traces. „Traceroute starten“ zeichnet den ersten auf.', 'No traces yet. “Start traceroute” records the first one.'],
  'peering.hops':         ['{n} Hops', '{n} hops'],
  'peering.incomplete':   ['unvollständig', 'incomplete'],
  'peering.complete':     ['vollständig', 'complete'],
  'peering.noGeo':        ['Keine Standortdaten – vermutlich ein privates Netz. Der Pfad ist unten aufgeführt.',
                           'No location data – probably a private network. The path is listed below.'],
  'peering.waiting':      ['Warte auf die ersten Hops …', 'Waiting for the first hops …'],
  'peering.noTrace':      ['Kein Trace ausgewählt.', 'No trace selected.'],
  'peering.hop':          ['Hop {n}', 'Hop {n}'],
  'peering.estimated':    ['(Standort geschätzt)', '(location estimated)'],
  'peering.locationRow':  ['Standort: {place}', 'Location: {place}'],
  'peering.located':      ['{n} verortet', '{n} located'],
  'peering.liveTitle':    ['Live-Traceroute zu {host}', 'Live traceroute to {host}'],
  'peering.traceFrom':    ['Trace vom {date}', 'Trace from {date}'],
  'peering.deleteTrace':  ['Trace löschen', 'Delete trace'],
  'peering.dest':         ['Ziel', 'Destination'],
  'peering.source':       ['Quelle', 'Source'],
  'peering.hopsLabel':    ['Hops', 'Hops'],
  'peering.rttDest':      ['RTT zum Ziel', 'RTT to destination'],
  'peering.status':       ['Status', 'Status'],
  'peering.running':      ['läuft …', 'running …'],
  'peering.test':         ['Zu Test', 'Test'],
  'peering.kindFirst':    ['Start', 'Start'],
  'peering.kindDest':     ['Ziel', 'Destination'],
  'peering.kindEst':      ['geschätzt', 'estimated'],
  'peering.kindTimeout':  ['keine Antwort', 'no response'],
  'peering.noHopsYet':    ['Noch keine Hops.', 'No hops yet.'],
  'peering.noReply':      ['* keine Antwort', '* no response'],
  'peering.noHops':       ['Noch keine Hops', 'No hops yet'],
  'peering.failed':       ['Traceroute fehlgeschlagen: {msg}', 'Traceroute failed: {msg}'],
  'peering.lost':         ['Verbindung zur Live-Traceroute verloren', 'Lost connection to the live traceroute'],
  'peering.done':         ['Traceroute abgeschlossen', 'Traceroute completed'],
  'peering.aborted':      ['Traceroute abgebrochen', 'Traceroute cancelled'],
  'peering.deleteQ':      ['Trace löschen?', 'Delete trace?'],
  'peering.deleteText':   ['Der Trace wird mit allen Hops endgültig gelöscht.', 'The trace will be permanently deleted along with all its hops.'],
  'peering.deleted':      ['Trace gelöscht', 'Trace deleted'],

  // Passwort
  'pw.current':           ['Aktuelles Passwort', 'Current password'],
  'pw.new':               ['Neues Passwort (mind. 6 Zeichen)', 'New password (at least 6 characters)'],
  'pw.repeat':            ['Neues Passwort wiederholen', 'Repeat new password'],
  'pw.change':            ['Ändern', 'Change'],
  'pw.mismatch':          ['Die neuen Passwörter stimmen nicht überein.', 'The new passwords do not match.'],
  'pw.changed':           ['Passwort geändert', 'Password changed'],

  // Administration
  'admin.users':          ['Benutzer', 'Users'],
  'admin.addUser':        ['+ Benutzer anlegen', '+ Create user'],
  'admin.username':       ['Benutzername', 'Username'],
  'admin.email':          ['E-Mail', 'Email'],
  'admin.role':           ['Rolle', 'Role'],
  'admin.created':        ['Angelegt', 'Created'],
  'admin.lastLogin':      ['Letzte Anmeldung', 'Last login'],
  'admin.cleanup':        ['Bereinigung', 'Cleanup'],
  'admin.deleteTests':    ['Tests löschen', 'Delete tests'],
  'admin.deleteTestsDesc':['Mit den Tests werden auch ihre Traces gelöscht.', 'Deleting tests also deletes their traces.'],
  'admin.scope':          ['Umfang', 'Scope'],
  'admin.olderThan':      ['älter als … Tage', 'older than … days'],
  'admin.all':            ['alle', 'all'],
  'admin.days':           ['Tage', 'Days'],
  'admin.allServers':     ['alle Server', 'all servers'],
  'admin.deleteTraces':   ['Traces löschen', 'Delete traces'],
  'admin.deleteTracesDesc':['Betrifft gespeicherte und Live-Traces samt ihrer Hops.', 'Affects saved and live traces including their hops.'],
  'admin.loadFailed':     ['Administration konnte nicht geladen werden: {msg}', 'Could not load the administration: {msg}'],
  'admin.none':           ['noch keine', 'none yet'],
  'admin.hops':           ['{n} Hops', '{n} hops'],
  'admin.servers':        ['Server', 'Servers'],
  'admin.signedIn':       ['angemeldet', 'signed in'],
  'admin.administrator':  ['Administrator', 'Administrator'],
  'admin.user':           ['Benutzer', 'User'],
  'admin.locked':         ['gesperrt', 'locked'],
  'admin.deleteUser':     ['Benutzer löschen', 'Delete user'],
  'admin.createUser':     ['Benutzer anlegen', 'Create user'],
  'admin.ufName':         ['Benutzername *', 'Username *'],
  'admin.ufMail':         ['E-Mail *', 'Email *'],
  'admin.ufPass':         ['Passwort * (mind. 6 Zeichen)', 'Password * (at least 6 characters)'],
  'admin.ufAdmin':        ['Administrator (darf Benutzer verwalten und Daten bereinigen)', 'Administrator (may manage users and clean up data)'],
  'admin.create':         ['Anlegen', 'Create'],
  'admin.userCreated':    ['Benutzer „{name}“ angelegt', 'User “{name}” created'],
  'admin.deleteUserQ':    ['Benutzer löschen?', 'Delete user?'],
  'admin.deleteUserText': ['„{name}“ kann sich danach nicht mehr anmelden.', '“{name}” will no longer be able to sign in.'],
  'admin.userDeleted':    ['Benutzer „{name}“ gelöscht', 'User “{name}” deleted'],
  'admin.invalidDays':    ['Bitte eine gültige Anzahl Tage angeben.', 'Please enter a valid number of days.'],
  'admin.allTests':       ['Alle Tests', 'All tests'],
  'admin.allTestsOf':     ['Alle Tests von „{name}“', 'All tests of “{name}”'],
  'admin.testsOlder':     ['{what}, die älter als {n} Tage sind,', '{what} older than {n} days'],
  'admin.deleteTestsQ':   ['Tests löschen?', 'Delete tests?'],
  'admin.deleteTestsText':['{what} werden samt ihrer Traces endgültig gelöscht.', '{what} will be permanently deleted along with their traces.'],
  'admin.testsDeleted':   ['{n} Test(s) gelöscht', '{n} test(s) deleted'],
  'admin.allTraces':      ['Alle Traces', 'All traces'],
  'admin.tracesOlder':    ['Alle Traces, die älter als {n} Tage sind,', 'All traces older than {n} days'],
  'admin.deleteTracesQ':  ['Traces löschen?', 'Delete traces?'],
  'admin.deleteTracesText':['{what} werden mit ihren Hops endgültig gelöscht.', '{what} will be permanently deleted along with their hops.'],
  'admin.tracesDeleted':  ['{traces} Trace(s) und {hops} Hop(s) gelöscht', '{traces} trace(s) and {hops} hop(s) deleted'],
};

// detectLang wählt die gespeicherte Sprache, sonst die des Browsers (Deutsch
// nur bei deutscher Browsersprache, sonst Englisch).
function detectLang() {
  let saved = null;
  try { saved = localStorage.getItem('iperf-lang'); } catch (e) {}
  if (LANGS.includes(saved)) return saved;
  const nav = (navigator.languages && navigator.languages[0]) || navigator.language || '';
  return nav.toLowerCase().startsWith('de') ? 'de' : 'en';
}
let LANG = detectLang();

// t liefert den Text zum Schlüssel in der aktiven Sprache.
function t(key, vars) {
  const entry = STRINGS[key];
  let s = entry ? entry[LANGS.indexOf(LANG)] ?? entry[0] : key;
  if (vars) s = s.replace(/\{(\w+)\}/g, (m, k) => (vars[k] != null ? vars[k] : m));
  return s;
}

// locale liefert das Gebietsschema für Zahlen und Datumsangaben.
function locale() { return LANG === 'de' ? 'de-DE' : 'en-GB'; }

// applyI18n setzt die statischen Texte: data-i18n (Text), data-i18n-placeholder,
// data-i18n-title und data-i18n-aria (aria-label).
function applyI18n(root = document) {
  root.querySelectorAll('[data-i18n]').forEach(el => { el.textContent = t(el.dataset.i18n); });
  root.querySelectorAll('[data-i18n-placeholder]').forEach(el => { el.placeholder = t(el.dataset.i18nPlaceholder); });
  root.querySelectorAll('[data-i18n-title]').forEach(el => { el.title = t(el.dataset.i18nTitle); });
  root.querySelectorAll('[data-i18n-aria]').forEach(el => { el.setAttribute('aria-label', t(el.dataset.i18nAria)); });
  document.documentElement.lang = LANG;
  document.querySelectorAll('[data-lang-current]').forEach(el => { el.textContent = LANG.toUpperCase(); });
  document.querySelectorAll('[data-lang]').forEach(el => {
    const active = el.dataset.lang === LANG;
    el.classList.toggle('active', active);
    el.setAttribute('aria-pressed', active);
  });
}

// setLang wechselt die Sprache, speichert sie und meldet den Wechsel per
// Ereignis "langchange", damit die Seiten ihre dynamischen Inhalte neu aufbauen.
function setLang(lang) {
  if (!LANGS.includes(lang)) return;
  const changed = lang !== LANG;
  LANG = lang;
  try { localStorage.setItem('iperf-lang', lang); } catch (e) {}
  applyI18n();
  if (changed) window.dispatchEvent(new Event('langchange'));
}

applyI18n();
