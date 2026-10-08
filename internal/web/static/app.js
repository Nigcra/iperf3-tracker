'use strict';

// ---------- Theme ----------

// Grid line colour used in the charts (theme-dependent, updated by applyChartTheme).
let GRID = 'rgba(148,163,184,0.14)';

// applyChartTheme aligns Chart.js text and grid colours with the active theme so
// charts stay legible on both the dark and the light background.
function applyChartTheme() {
  const light = document.documentElement.getAttribute('data-theme') === 'light';
  GRID = light ? 'rgba(15,23,42,0.10)' : 'rgba(148,163,184,0.14)';
  if (window.Chart) {
    Chart.defaults.color = light ? '#475569' : '#94a3b8';
    Chart.defaults.borderColor = GRID;
    Chart.defaults.font.family = "'Segoe UI', system-ui, Arial, sans-serif";
    if (Chart.defaults.plugins && Chart.defaults.plugins.legend) {
      Chart.defaults.plugins.legend.labels.color = light ? '#334155' : '#cbd5e1';
    }
  }
}
applyChartTheme();

// toggleTheme flips the light/dark theme, persists it and lets the active page
// re-render theme-dependent parts (charts, map tiles).
function toggleTheme() {
  const el = document.documentElement;
  const next = el.getAttribute('data-theme') === 'light' ? 'dark' : 'light';
  el.setAttribute('data-theme', next);
  try { localStorage.setItem('iperf-theme', next); } catch (e) {}
  applyChartTheme();
  const page = pages[currentTab];
  if (page && page.onTheme) page.onTheme();
}

// ---------- Hilfsfunktionen ----------
function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, c =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}
function fmt(n, digits = 2) {
  return n == null ? '–' : Number(n).toLocaleString('de-DE', { maximumFractionDigits: digits });
}
function fmtDate(iso) { return iso ? new Date(iso).toLocaleString('de-DE') : '–'; }

function storageGet(key) { try { return localStorage.getItem(key); } catch (e) { return null; } }
function storageSet(key, value) {
  try { value == null ? localStorage.removeItem(key) : localStorage.setItem(key, value); } catch (e) {}
}

// ---------- API ----------

// ApiError trägt den HTTP-Status und die Meldung aus {"detail": "..."}.
class ApiError extends Error {
  constructor(status, message) { super(message); this.status = status; }
}

// api ruft die JSON-API auf. Bei 401 (Sitzung abgelaufen) wird abgemeldet.
async function api(method, path, body) {
  const headers = {};
  const token = storageGet('iperf-token');
  if (token) headers.Authorization = 'Bearer ' + token;
  const opts = { method, headers };
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch('/api' + path, opts);
  if (res.status === 204) return null;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    if (res.status === 401 && path !== '/auth/login') logout();
    throw new ApiError(res.status, (data && data.detail) || ('HTTP ' + res.status));
  }
  return data;
}
const apiGet = path => api('GET', path);

// ---------- Sitzung ----------
let currentUser = null;

function showLogin(message) {
  currentUser = null;
  stopStatusPolling();
  document.getElementById('appHeader').hidden = true;
  document.getElementById('appMain').hidden = true;
  document.getElementById('loginView').hidden = false;
  const err = document.getElementById('loginError');
  err.textContent = message || '';
  err.hidden = !message;
  document.getElementById('loginUser').focus();
}

function showApp(user) {
  currentUser = user;
  document.getElementById('loginView').hidden = true;
  document.getElementById('appHeader').hidden = false;
  document.getElementById('appMain').hidden = false;
  document.getElementById('userName').textContent = user.username;
  document.getElementById('userAdminBadge').hidden = !user.is_admin;
  document.getElementById('navAdmin').hidden = !user.is_admin;
  startStatusPolling();
  showTab(tabFromHash());
}

async function submitLogin(ev) {
  ev.preventDefault();
  const btn = document.getElementById('loginBtn');
  btn.disabled = true;
  try {
    const res = await api('POST', '/auth/login', {
      username: document.getElementById('loginUser').value.trim(),
      password: document.getElementById('loginPass').value,
    });
    storageSet('iperf-token', res.access_token);
    document.getElementById('loginPass').value = '';
    showApp(res.user);
  } catch (e) {
    showLogin(e.status === 401 ? 'Benutzername oder Passwort falsch.' : e.message);
  } finally {
    btn.disabled = false;
  }
}

function logout() {
  storageSet('iperf-token', null);
  closeMenus();
  showLogin();
}

// ---------- Navigation ----------

// pages bündelt je Tab die Lade- und Aufräumfunktionen; die einzelnen Seiten
// tragen sich in den folgenden Ausbaustufen hier ein.
const pages = {
  dashboard: {},
  tests: {},
  peering: {},
  servers: {},
  admin: { adminOnly: true },
};
let currentTab = null;

function tabFromHash() {
  const tab = window.location.hash.slice(1);
  return pages[tab] ? tab : 'dashboard';
}

function showTab(tab) {
  if (!pages[tab] || (pages[tab].adminOnly && !(currentUser && currentUser.is_admin))) tab = 'dashboard';
  if (currentTab && currentTab !== tab && pages[currentTab].leave) pages[currentTab].leave();
  currentTab = tab;
  document.querySelectorAll('.page').forEach(p => p.classList.toggle('active', p.id === 'page-' + tab));
  document.querySelectorAll('.nav button').forEach(b => b.classList.toggle('active', b.dataset.tab === tab));
  if (window.location.hash.slice(1) !== tab) {
    try { history.pushState(null, '', '#' + tab); } catch (e) {}
  }
  if (pages[tab].enter) pages[tab].enter();
}

window.addEventListener('hashchange', () => { if (currentUser) showTab(tabFromHash()); });

// ---------- Menüs ----------
function toggleMenu(ev, id) {
  ev.stopPropagation();
  const panel = document.getElementById(id);
  const open = panel.hidden;
  closeMenus();
  panel.hidden = !open;
}
function closeMenus() {
  document.querySelectorAll('.menu-panel').forEach(p => { p.hidden = true; });
}
document.addEventListener('click', ev => {
  if (!ev.target.closest('.menu')) closeMenus();
});

// ---------- Status ----------
let statusTimer = null;

// loadStatus fasst Dienstzustand, Scheduler und laufende Tests im Statuspunkt zusammen.
async function loadStatus() {
  const icon = document.getElementById('statusIcon');
  const btn = document.getElementById('statusBtn');
  const panel = document.getElementById('statusPanel');
  let cls, sym, title, rows;
  try {
    const [health, running, pending] = await Promise.all([
      fetch('/health').then(r => r.json()),
      apiGet('/tests?status=running&limit=10'),
      apiGet('/tests?status=pending&limit=10'),
    ]);
    const active = running.length + pending.length;
    rows = '<div class="sp-row"><span class="sp-label">Dienst:</span> erreichbar</div>'
         + '<div class="sp-row"><span class="sp-label">Scheduler:</span> ' + (health.scheduler_running ? 'aktiv' : 'aus') + '</div>'
         + '<div class="sp-row"><span class="sp-label">Laufende Tests:</span> ' + running.length
         + (pending.length ? ' (+' + pending.length + ' wartend)' : '') + '</div>';
    if (active) { cls = 'status-dot-warn'; sym = '&#9679;'; title = 'Test läuft'; }
    else { cls = 'status-dot-ok'; sym = '&#9679;'; title = 'Status: OK'; }
    btn.classList.remove('has-error');
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return;
    cls = 'status-dot-err'; sym = '&#9888;'; title = 'Dienst nicht erreichbar';
    rows = '<div class="sp-row sp-err">&#9888; Dienst nicht erreichbar</div>';
    btn.classList.add('has-error');
  }
  icon.className = cls;
  icon.innerHTML = sym;
  btn.title = title;
  panel.innerHTML = rows;
}

function startStatusPolling() {
  stopStatusPolling();
  loadStatus();
  statusTimer = setInterval(loadStatus, 5000);
}
function stopStatusPolling() {
  if (statusTimer) clearInterval(statusTimer);
  statusTimer = null;
}

async function loadInfo() {
  try {
    const info = await fetch('/api/info').then(r => r.json());
    document.getElementById('appVersion').textContent = info.version || '';
    document.getElementById('buildDate').textContent = info.build_date || '–';
  } catch (e) {}
}

// ---------- Meldungen ----------

// notify zeigt eine kurze Meldung unten rechts (kind: ok | err).
function notify(message, kind = 'ok') {
  let box = document.getElementById('toasts');
  if (!box) {
    box = document.createElement('div');
    box.id = 'toasts';
    document.body.appendChild(box);
  }
  const t = document.createElement('div');
  t.className = 'toast toast-' + kind;
  t.textContent = message;
  box.appendChild(t);
  setTimeout(() => t.remove(), kind === 'err' ? 7000 : 3500);
}

// ---------- Icons ----------
const ICON_PLAY = '<svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><polygon points="6 3 20 12 6 21 6 3"/></svg>';
const ICON_MAP = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14.106 5.553a2 2 0 0 0 1.788 0l3.659-1.83A1 1 0 0 1 21 4.619v12.764a1 1 0 0 1-.553.894l-4.553 2.277a2 2 0 0 1-1.788 0l-4.212-2.106a2 2 0 0 0-1.788 0l-3.659 1.83A1 1 0 0 1 3 19.381V6.618a1 1 0 0 1 .553-.894l4.553-2.277a2 2 0 0 1 1.788 0z"/><path d="M15 5.764v15"/><path d="M9 3.236v15"/></svg>';

// ---------- Gemeinsame Daten ----------

// Farbpalette je Server; die Zuordnung folgt der Server-ID, damit ein Server in
// allen Diagrammen und Karten dieselbe Farbe hat.
const SERIES_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ef4444', '#06b6d4', '#ec4899', '#f97316'];
function serverColor(servers, id) {
  const ids = servers.map(s => s.id).sort((a, b) => a - b);
  const i = ids.indexOf(id);
  return SERIES_COLORS[(i < 0 ? 0 : i) % SERIES_COLORS.length];
}

function protocolLabel(p) { return (p || '').toUpperCase(); }
function directionLabel(d) {
  return { download: 'Download', upload: 'Upload', bidirectional: 'Bidirektional' }[d] || d;
}
function streamsLabel(n) { return n + (n === 1 ? ' Stream' : ' Streams'); }

// syncCards gleicht ein Raster stabiler Karten ab (je Schlüssel ein Element):
// Karten bleiben erhalten, ihr Inhalt wird nur bei Änderungen ersetzt. So gehen
// Hover, Fokus und Klicks bei periodischen Aktualisierungen nicht verloren.
function syncCards(container, cards, cls = 'ds-card') {
  const existing = new Map();
  [...container.children].forEach(el => (el.dataset.key ? existing.set(el.dataset.key, el) : el.remove()));
  cards.forEach(([key, html], i) => {
    key = String(key);
    let el = existing.get(key);
    if (!el) {
      el = document.createElement('div');
      el.className = cls;
      el.dataset.key = key;
    }
    existing.delete(key);
    if (el._html !== html) { el.innerHTML = html; el._html = html; }
    if (container.children[i] !== el) container.insertBefore(el, container.children[i] || null);
  });
  existing.forEach(el => el.remove());
}

// openPeering wechselt zur Peering-Map und wählt dort den Server vor.
let pendingPeeringServer = null;
function openPeering(serverId) {
  pendingPeeringServer = serverId;
  showTab('peering');
}

// ---------- Dashboard ----------

const RANGE_HOURS = { '1h': 1, '24h': 24, '7d': 168, '30d': 720 };

function storedNumber(key, def) {
  const v = parseFloat(storageGet(key));
  return isNaN(v) ? def : v;
}

const dashboard = {
  range: storageGet('iperf-dash-range') || '24h',
  thrDown: storedNumber('iperf-dash-thr-down', 100),
  thrUp: storedNumber('iperf-dash-thr-up', 50),
  dash: null,
  servers: [],
  stats: [],
  tests: [],
  liveIds: new Map(),      // Test-ID → Server-ID der verfolgten Tests
  liveByServer: new Map(), // Server-ID → Live-Status
  charts: {},
  timers: [],

  enter() {
    if (!RANGE_HOURS[this.range]) this.range = '24h';
    document.getElementById('thrDown').value = this.thrDown;
    document.getElementById('thrUp').value = this.thrUp;
    this.markRange();
    this.load();
    this.pollLive();
    this.timers = [setInterval(() => this.load(), 30000), setInterval(() => this.pollLive(), 1500)];
  },
  leave() {
    this.timers.forEach(clearInterval);
    this.timers = [];
  },
  onTheme() { this.renderCharts(); },

  setRange(r) {
    this.range = r;
    storageSet('iperf-dash-range', r);
    this.markRange();
    this.load();
  },
  markRange() {
    document.querySelectorAll('#dashRange .view-btn').forEach(b => b.classList.toggle('active', b.dataset.range === this.range));
  },
  setThreshold(kind, value) {
    const v = Math.max(0, parseFloat(value) || 0);
    if (kind === 'down') { this.thrDown = v; storageSet('iperf-dash-thr-down', v); }
    else { this.thrUp = v; storageSet('iperf-dash-thr-up', v); }
    this.renderSummary();
    this.renderCharts();
  },

  async load() {
    const since = new Date(Date.now() - RANGE_HOURS[this.range] * 3600e3).toISOString();
    try {
      const [dash, stats, servers, tests] = await Promise.all([
        apiGet('/stats/dashboard'),
        apiGet('/stats/servers'),
        apiGet('/servers'),
        apiGet('/tests?limit=1000&status=completed&from_date=' + encodeURIComponent(since)),
      ]);
      Object.assign(this, { dash, stats, servers, tests });
      this.renderSummary();
      this.renderCharts();
      this.renderServers();
    } catch (e) {
      if (e.status !== 401) notify('Dashboard konnte nicht geladen werden: ' + e.message, 'err');
    }
  },

  // belowThreshold zählt Tests im Zeitraum, die in einer gemessenen Richtung
  // unter dem jeweiligen Schwellwert liegen.
  belowThreshold() {
    return this.tests.filter(t =>
      (t.download_bandwidth_mbps != null && t.download_bandwidth_mbps < this.thrDown) ||
      (t.upload_bandwidth_mbps != null && t.upload_bandwidth_mbps < this.thrUp)).length;
  },

  renderSummary() {
    const d = this.dash;
    if (!d) return;
    const below = this.belowThreshold();
    const pct = this.tests.length ? Math.round(below / this.tests.length * 100) : 0;
    const card = (val, lbl, sub, cls = '') =>
      `<div class="sum-card ${cls}"><div class="val">${val}</div><div class="lbl">${esc(lbl)}</div><div class="sub">${esc(sub)}</div></div>`;
    document.getElementById('dashSummary').innerHTML =
      card(d.total_servers, 'Server', d.active_servers + ' aktiv') +
      card(fmt(d.total_tests, 0), 'Tests', d.tests_today + ' heute') +
      card(fmt(d.avg_download_mbps, 1), 'Ø Download', 'Mbit/s · alle Tests') +
      card(fmt(d.avg_upload_mbps, 1), 'Ø Upload', 'Mbit/s · alle Tests') +
      card(below, 'Unter Schwellwert', pct + ' % im Zeitraum', below ? 'warn' : 'ok');
  },

  renderCharts() {
    this.renderChart('chartDown', 'download_bandwidth_mbps', this.thrDown, '#ef4444');
    this.renderChart('chartUp', 'upload_bandwidth_mbps', this.thrUp, '#f59e0b');
  },

  renderChart(canvasId, key, threshold, thresholdColor) {
    const canvas = document.getElementById(canvasId);
    const wrap = canvas.parentElement;
    if (this.charts[canvasId]) { this.charts[canvasId].destroy(); delete this.charts[canvasId]; }
    wrap.querySelector('.chart-empty')?.remove();
    if (!window.Chart) {
      wrap.insertAdjacentHTML('beforeend', '<div class="chart-empty">Diagramme nicht verfügbar (Chart.js nicht geladen)</div>');
      return;
    }

    const now = Date.now();
    const min = now - RANGE_HOURS[this.range] * 3600e3;
    const points = new Map(); // Server-ID → Punkte
    for (const t of this.tests) {
      const v = t[key];
      if (v == null || v <= 0) continue;
      if (!points.has(t.server_id)) points.set(t.server_id, []);
      points.get(t.server_id).push({ x: Date.parse(t.created_at), y: v });
    }
    const datasets = [...points.entries()].sort((a, b) => a[0] - b[0]).map(([id, data]) => {
      const sv = this.servers.find(s => s.id === id);
      const color = serverColor(this.servers, id);
      return {
        label: sv ? sv.name : 'Server ' + id,
        data: data.sort((a, b) => a.x - b.x),
        borderColor: color,
        backgroundColor: color,
        pointBackgroundColor: ctx => (ctx.raw && ctx.raw.y < threshold ? '#ef4444' : color),
        pointBorderColor: ctx => (ctx.raw && ctx.raw.y < threshold ? '#ef4444' : color),
        pointRadius: 3.5, pointHoverRadius: 6, borderWidth: 2, tension: 0.3,
      };
    });
    const max = Math.max(threshold, ...datasets.flatMap(ds => ds.data.map(p => p.y)));
    // Skala in üblichen Leitungsstufen: 1 / 2,5 / 10 Gbit/s, darüber in 10-Gbit-Schritten.
    const yMax = max <= 1000 ? 1000 : max <= 2500 ? 2500 : max <= 10000 ? 10000 : Math.ceil(max / 10000) * 10000;
    datasets.push({
      label: 'Schwellwert', data: [{ x: min, y: threshold }, { x: now, y: threshold }],
      borderColor: thresholdColor, backgroundColor: thresholdColor, borderDash: [6, 4], borderWidth: 1.5, pointRadius: 0, pointHoverRadius: 0,
    });

    const hours = RANGE_HOURS[this.range];
    const tick = v => {
      const d = new Date(v);
      return hours <= 24 ? d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })
                         : d.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit' });
    };
    this.charts[canvasId] = new Chart(canvas, {
      type: 'line',
      data: { datasets },
      options: {
        responsive: true, maintainAspectRatio: false, animation: { duration: 250 },
        interaction: { mode: 'nearest', intersect: true },
        scales: {
          x: { type: 'linear', min, max: now, ticks: { callback: tick, maxTicksLimit: 8 }, grid: { color: GRID } },
          y: { min: 0, max: yMax, ticks: { callback: v => fmt(v, 0) }, grid: { color: GRID } },
        },
        plugins: {
          legend: {
            position: 'bottom',
            labels: {
              usePointStyle: true, boxWidth: 8, boxHeight: 8,
              // Legende in der Serienfarbe, nicht in der (ggf. roten) Farbe des ersten Punkts.
              generateLabels: chart => Chart.defaults.plugins.legend.labels.generateLabels(chart).map(l => {
                const c = chart.data.datasets[l.datasetIndex].borderColor;
                return Object.assign(l, { fillStyle: c, strokeStyle: c });
              }),
            },
          },
          tooltip: {
            filter: item => item.dataset.label !== 'Schwellwert',
            callbacks: {
              title: items => items.length ? new Date(items[0].raw.x).toLocaleString('de-DE') : '',
              label: item => `${item.dataset.label}: ${fmt(item.raw.y, 1)} Mbit/s`,
            },
          },
        },
      },
    });
    if (!points.size) wrap.insertAdjacentHTML('beforeend', '<div class="chart-empty">Keine Messwerte im Zeitraum</div>');
  },

  renderServers() {
    const grid = document.getElementById('dashServers');
    if (!this.servers.length) {
      grid.innerHTML = '<div class="ds-card"><p class="muted" style="margin-bottom:12px">Noch keine Server angelegt.</p>'
        + '<button class="btn" onclick="showTab(\'servers\')">Server anlegen</button></div>';
      return;
    }
    const byId = new Map(this.stats.map(s => [s.server_id, s]));
    syncCards(grid, [...this.servers].sort((a, b) => a.name.localeCompare(b.name, 'de')).map(sv => {
      const st = byId.get(sv.id) || {};
      const live = this.liveByServer.get(sv.id);
      const color = serverColor(this.servers, sv.id);
      const okPct = st.total_tests ? Math.round(st.successful_tests / st.total_tests * 100) : null;

      const badges = [
        `<span class="badge-type">${protocolLabel(sv.default_protocol)}</span>`,
        `<span class="sev-badge sev-info">${directionLabel(sv.default_direction)}</span>`,
        `<span class="sev-badge sev-info">${streamsLabel(sv.default_parallel)}</span>`,
        sv.schedule_enabled ? `<span class="sev-badge sev-info">alle ${sv.schedule_interval_minutes} min</span>` : '',
        okPct != null ? `<span class="sev-badge ${okPct >= 90 ? 'sev-ok' : okPct >= 50 ? 'sev-warn' : 'sev-crit'}">${okPct} % OK</span>` : '',
        sv.enabled ? '' : '<span class="sev-badge sev-crit">deaktiviert</span>',
      ].join('');

      let liveBlock = '';
      if (live) {
        const state = { pending: 'Wartet', running: 'Läuft', completed: 'Abgeschlossen', failed: 'Fehlgeschlagen' }[live.status] || live.status;
        const time = live.status === 'running' ? ` · ${live.elapsed_seconds} / ${live.total_seconds} s` : '';
        liveBlock = `<div class="live-line"><span class="${live.status === 'running' ? 'pulse' : ''}">${state}${time}</span><span>${live.progress} %</span></div>`
          + `<div class="bar-bg"><div class="bar-fill ${live.status === 'failed' ? 'failed' : ''}" style="width:${live.status === 'failed' ? 100 : live.progress}%"></div></div>`;
      }
      const showLive = live && live.status !== 'failed' && live.status !== 'pending';
      const down = showLive ? live.current_download_mbps : st.avg_download_mbps;
      const up = showLive ? live.current_upload_mbps : st.avg_upload_mbps;
      const prefix = showLive ? '' : 'Ø ';
      const stat = (label, val, cls = '') => `<div class="stat"><span class="stat-label">${label}</span><span class="stat-val ${cls}">${val}</span></div>`;

      return [sv.id, `
        <div class="ds-header">
          <span style="width:10px;height:10px;border-radius:50%;background:${color};flex:none"></span>
          <span class="ds-name">${esc(sv.name)}</span>
          <div class="card-actions">
            <button class="icon-btn" title="Traceroute auf der Peering-Map" onclick="openPeering(${sv.id})">${ICON_MAP}</button>
            <button class="icon-btn" title="Schnelltest mit den Vorgaben des Servers" onclick="dashboard.quickTest(${sv.id})" ${live || !sv.enabled ? 'disabled' : ''}>${ICON_PLAY}</button>
          </div>
        </div>
        <div class="ds-badges">${badges}</div>
        ${liveBlock}
        <div class="ds-stats">
          ${stat(prefix + 'Download', down == null ? '–' : fmt(down, 1) + ' Mbit/s', 'down')}
          ${stat(prefix + 'Upload', up == null ? '–' : fmt(up, 1) + ' Mbit/s', 'up')}
          ${stat('Ø Jitter', st.avg_jitter_ms == null ? '–' : fmt(st.avg_jitter_ms, 2) + ' ms')}
          ${stat('Ø Paketverlust', st.avg_packet_loss_percent == null ? '–' : fmt(st.avg_packet_loss_percent, 2) + ' %')}
          ${stat('Tests', st.total_tests ? fmt(st.total_tests, 0) + (st.failed_tests ? ` (${st.failed_tests} fehlgeschl.)` : '') : '–')}
          ${stat('Letzter Test', st.last_test_at ? fmtDate(st.last_test_at) : '–')}
        </div>`];
    }));
  },

  // pollLive verfolgt wartende und laufende Tests. Der Server meldet einen Test
  // noch 10 s nach Ende als laufend, so bleiben die Endwerte kurz sichtbar.
  async pollLive() {
    try {
      const [running, pending] = await Promise.all([
        apiGet('/tests?status=running&limit=50'),
        apiGet('/tests?status=pending&limit=50'),
      ]);
      for (const t of [...running, ...pending]) this.liveIds.set(t.id, t.server_id);
      if (!this.liveIds.size && !this.liveByServer.size) return;

      const results = await Promise.all([...this.liveIds].map(([id, sid]) =>
        apiGet(`/tests/${id}/live`).then(l => [id, sid, l]).catch(() => [id, sid, null])));
      const byServer = new Map();
      let finished = false;
      for (const [id, sid, l] of results) {
        if (!l || !l.is_running) { this.liveIds.delete(id); finished = true; continue; }
        // Bei mehreren Tests eines Servers hat der laufende Vorrang vor wartenden.
        const prev = byServer.get(sid);
        if (!prev || prev.status === 'pending') byServer.set(sid, l);
      }
      this.liveByServer = byServer;
      this.renderServers();
      if (finished) this.load();
    } catch (e) {}
  },

  async quickTest(serverId) {
    const sv = this.servers.find(s => s.id === serverId);
    if (!sv) return;
    try {
      const t = await api('POST', '/tests/run', {
        server_id: sv.id,
        protocol: sv.default_protocol,
        direction: sv.default_direction,
        duration: sv.default_duration,
        parallel_streams: sv.default_parallel,
      });
      this.liveIds.set(t.id, sv.id);
      notify(`Test für ${sv.name} gestartet`);
      this.pollLive();
    } catch (e) {
      notify('Test konnte nicht gestartet werden: ' + e.message, 'err');
    }
  },
};
pages.dashboard = dashboard;

// ---------- Start ----------
(async function init() {
  loadInfo();
  if (!storageGet('iperf-token')) { showLogin(); return; }
  try {
    showApp(await apiGet('/auth/me'));
  } catch (e) {
    showLogin(e.status === 401 ? '' : 'Dienst nicht erreichbar: ' + e.message);
  }
})();
