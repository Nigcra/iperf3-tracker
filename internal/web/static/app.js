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
  return n == null ? '–' : Number(n).toLocaleString(locale(), { maximumFractionDigits: digits });
}
function fmtDate(iso) { return iso ? new Date(iso).toLocaleString(locale()) : '–'; }

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
// Accept-Language sorgt für Meldungen in der gewählten Sprache.
async function api(method, path, body) {
  const headers = { 'Accept-Language': LANG };
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
  iperfSetup.reset();
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
    const usesDefault = res.user.username === 'admin' && document.getElementById('loginPass').value === 'admin123';
    storageSet('iperf-token', res.access_token);
    document.getElementById('loginPass').value = '';
    showApp(res.user);
    if (usesDefault) notify(t('login.defaultPw'), 'warn');
  } catch (e) {
    showLogin(e.status === 401 ? t('login.failed') : e.message);
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

// ---------- Sprache ----------

// Die statischen Texte setzt setLang() selbst; hier bauen Statusmenü und
// aktive Seite ihre dynamischen Inhalte in der neuen Sprache auf.
window.addEventListener('langchange', () => {
  if (!currentUser) return;
  loadStatus();
  const page = pages[currentTab];
  if (page && page.onLang) page.onLang();
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
    const [status, running, pending] = await Promise.all([
      apiGet('/status'),
      apiGet('/tests?status=running&limit=10'),
      apiGet('/tests?status=pending&limit=10'),
    ]);
    const ip = status.iperf3;
    const active = running.length + pending.length;
    let iperfRow;
    if (ip.installing) iperfRow = t('status.installing');
    else if (ip.available) iperfRow = esc(ip.version);
    else iperfRow = `<span class="sp-err">${esc(ip.error || t('status.unavailable'))}</span>`;
    rows = `<div class="sp-row"><span class="sp-label">${t('status.service')}</span> ${t('status.reachable')}</div>`
         + '<div class="sp-row"><span class="sp-label">iperf3:</span> ' + iperfRow + '</div>'
         + `<div class="sp-row"><span class="sp-label">${t('status.scheduler')}</span> ${status.scheduler_running ? t('status.on') : t('status.off')}</div>`
         + `<div class="sp-row"><span class="sp-label">${t('status.running')}</span> ${running.length}`
         + (pending.length ? ' ' + t('status.waiting', { n: pending.length }) : '') + '</div>';
    if (ip.installable && !ip.installing && currentUser && currentUser.is_admin) {
      rows += '<button class="btn" style="width:100%;margin-top:10px" onclick="iperfSetup.prompt()">' + t('status.install') + '</button>';
    }
    iperfSetup.update(ip);
    const broken = !ip.available && !ip.installing;
    if (broken) { cls = 'status-dot-err'; sym = '&#9888;'; title = t('status.titleBroken'); }
    else if (ip.installing) { cls = 'status-dot-warn'; sym = '&#9679;'; title = t('status.titleInstall'); }
    else if (active) { cls = 'status-dot-warn'; sym = '&#9679;'; title = t('status.titleTest'); }
    else { cls = 'status-dot-ok'; sym = '&#9679;'; title = t('status.titleOk'); }
    btn.classList.toggle('has-error', broken);
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return;
    cls = 'status-dot-err'; sym = '&#9888;'; title = t('status.down');
    rows = '<div class="sp-row sp-err">&#9888; ' + t('status.down') + '</div>';
    btn.classList.add('has-error');
  }
  icon.className = cls;
  icon.innerHTML = sym;
  btn.title = title;
  panel.innerHTML = rows;
}

// iperfSetup fragt nach, wenn iperf3 fehlt, und meldet das Ergebnis einer
// Installation. Installiert wird nur nach Bestätigung durch einen Admin.
const iperfSetup = {
  status: null,
  asked: false,       // einmal je Anmeldung nachfragen bzw. hinweisen
  prompting: false,
  wasInstalling: false,

  reset() { this.asked = false; this.wasInstalling = false; },

  update(ip) {
    this.status = ip;
    if (this.wasInstalling && !ip.installing) {
      if (ip.available) notify(t('setup.installed', { version: ip.version }));
      else notify(t('setup.failed', { msg: ip.error || t('unknownError') }), 'err');
    }
    this.wasInstalling = ip.installing;
    if (ip.available || ip.installing || this.prompting || this.asked) return;
    this.asked = true;
    if (!currentUser || !currentUser.is_admin) {
      notify(t('setup.missingUser'), 'warn');
      return;
    }
    let later = false;
    try { later = sessionStorage.getItem('iperf-install-later') === '1'; } catch (e) {}
    if (ip.installable && !later) this.prompt();
  },

  async prompt() {
    const ip = this.status;
    if (!ip || !ip.installable || this.prompting) return;
    closeMenus();
    this.prompting = true;
    const ok = await confirmDialog(t('setup.question'), t('setup.text', { cmd: ip.install_command }),
      t('setup.install'), false, t('later'));
    this.prompting = false;
    if (!ok) {
      try { sessionStorage.setItem('iperf-install-later', '1'); } catch (e) {}
      return;
    }
    try {
      await api('POST', '/iperf3/install');
      this.wasInstalling = true;
      notify(t('setup.started'));
      loadStatus();
    } catch (e) {
      notify(t('setup.impossible', { msg: e.message }), 'err');
    }
  },
};

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

// notify zeigt eine kurze Meldung unten rechts (kind: ok | warn | err).
function notify(message, kind = 'ok') {
  let box = document.getElementById('toasts');
  if (!box) {
    box = document.createElement('div');
    box.id = 'toasts';
    document.body.appendChild(box);
  }
  const el = document.createElement('div');
  el.className = 'toast toast-' + kind;
  el.textContent = message;
  box.appendChild(el);
  // Höchstens drei Meldungen gleichzeitig, damit sie keine Formulare verdecken.
  while (box.children.length > 3) box.firstChild.remove();
  setTimeout(() => el.remove(), { ok: 3500, err: 7000, warn: 12000 }[kind] || 3500);
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
function directionLabel(d) { return STRINGS['dir.' + d] ? t('dir.' + d) : d; }
function streamsLabel(n) { return t(n === 1 ? 'streams.one' : 'streams.other', { n }); }

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
  onLang() {
    this.renderSummary();
    this.renderCharts();
    this.renderServers();
  },

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
      if (e.status !== 401) notify(t('dash.loadFailed', { msg: e.message }), 'err');
    }
  },

  // belowThreshold zählt Tests im Zeitraum, die in einer gemessenen Richtung
  // unter dem jeweiligen Schwellwert liegen.
  belowThreshold() {
    return this.tests.filter(x =>
      (x.download_bandwidth_mbps != null && x.download_bandwidth_mbps < this.thrDown) ||
      (x.upload_bandwidth_mbps != null && x.upload_bandwidth_mbps < this.thrUp)).length;
  },

  renderSummary() {
    const d = this.dash;
    if (!d) return;
    const below = this.belowThreshold();
    const pct = this.tests.length ? Math.round(below / this.tests.length * 100) : 0;
    const card = (val, lbl, sub, cls = '') =>
      `<div class="sum-card ${cls}"><div class="val">${val}</div><div class="lbl">${esc(lbl)}</div><div class="sub">${esc(sub)}</div></div>`;
    document.getElementById('dashSummary').innerHTML =
      card(d.total_servers, t('dash.servers'), t('dash.activeCount', { n: d.active_servers })) +
      card(fmt(d.total_tests, 0), t('dash.tests'), t('dash.today', { n: d.tests_today })) +
      card(fmt(d.avg_download_mbps, 1), t('dash.avgDown'), t('dash.allTests')) +
      card(fmt(d.avg_upload_mbps, 1), t('dash.avgUp'), t('dash.allTests')) +
      card(below, t('dash.below'), t('dash.belowPct', { n: pct }), below ? 'warn' : 'ok');
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
      wrap.insertAdjacentHTML('beforeend', '<div class="chart-empty">' + t('dash.noChart') + '</div>');
      return;
    }

    const now = Date.now();
    const min = now - RANGE_HOURS[this.range] * 3600e3;
    const points = new Map(); // Server-ID → Punkte
    for (const x of this.tests) {
      const v = x[key];
      if (v == null || v <= 0) continue;
      if (!points.has(x.server_id)) points.set(x.server_id, []);
      points.get(x.server_id).push({ x: Date.parse(x.created_at), y: v });
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
      label: t('dash.threshold'), isThreshold: true, data: [{ x: min, y: threshold }, { x: now, y: threshold }],
      borderColor: thresholdColor, backgroundColor: thresholdColor, borderDash: [6, 4], borderWidth: 1.5, pointRadius: 0, pointHoverRadius: 0,
    });

    const hours = RANGE_HOURS[this.range];
    const tick = v => {
      const d = new Date(v);
      return hours <= 24 ? d.toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' })
                         : d.toLocaleDateString(locale(), { day: '2-digit', month: '2-digit' });
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
            filter: item => !item.dataset.isThreshold,
            callbacks: {
              title: items => items.length ? new Date(items[0].raw.x).toLocaleString(locale()) : '',
              label: item => `${item.dataset.label}: ${fmt(item.raw.y, 1)} Mbit/s`,
            },
          },
        },
      },
    });
    if (!points.size) wrap.insertAdjacentHTML('beforeend', '<div class="chart-empty">' + t('dash.noData') + '</div>');
  },

  renderServers() {
    const grid = document.getElementById('dashServers');
    if (!this.servers.length) {
      grid.innerHTML = `<div class="ds-card"><p class="muted" style="margin-bottom:12px">${t('dash.noServers')}</p>`
        + `<button class="btn" onclick="showTab('servers')">${t('dash.createServer')}</button></div>`;
      return;
    }
    const byId = new Map(this.stats.map(s => [s.server_id, s]));
    syncCards(grid, [...this.servers].sort((a, b) => a.name.localeCompare(b.name, LANG)).map(sv => {
      const st = byId.get(sv.id) || {};
      const live = this.liveByServer.get(sv.id);
      const color = serverColor(this.servers, sv.id);
      const okPct = st.total_tests ? Math.round(st.successful_tests / st.total_tests * 100) : null;

      const badges = [
        `<span class="badge-type">${protocolLabel(sv.default_protocol)}</span>`,
        `<span class="sev-badge sev-info">${directionLabel(sv.default_direction)}</span>`,
        `<span class="sev-badge sev-info">${streamsLabel(sv.default_parallel)}</span>`,
        sv.schedule_enabled ? `<span class="sev-badge sev-info">${t('badge.every', { n: sv.schedule_interval_minutes })}</span>` : '',
        okPct != null ? `<span class="sev-badge ${okPct >= 90 ? 'sev-ok' : okPct >= 50 ? 'sev-warn' : 'sev-crit'}">${t('badge.okPct', { n: okPct })}</span>` : '',
        sv.enabled ? '' : `<span class="sev-badge sev-crit">${t('badge.disabled')}</span>`,
      ].join('');

      let liveBlock = '';
      if (live) {
        const state = statusLabel(live.status);
        const time = live.status === 'running' ? ` · ${live.elapsed_seconds} / ${live.total_seconds} s` : '';
        liveBlock = `<div class="live-line"><span class="${live.status === 'running' ? 'pulse' : ''}">${state}${time}</span><span>${live.progress} %</span></div>`
          + `<div class="bar-bg"><div class="bar-fill ${live.status === 'failed' ? 'failed' : ''}" style="width:${live.status === 'failed' ? 100 : live.progress}%"></div></div>`;
      }
      const showLive = live && live.status !== 'failed' && live.status !== 'pending';
      const down = showLive ? live.current_download_mbps : st.avg_download_mbps;
      const up = showLive ? live.current_upload_mbps : st.avg_upload_mbps;
      const prefix = showLive ? '' : t('dash.avgPrefix');
      const stat = (label, val, cls = '') => `<div class="stat"><span class="stat-label">${label}</span><span class="stat-val ${cls}">${val}</span></div>`;

      return [sv.id, `
        <div class="ds-header">
          <span style="width:10px;height:10px;border-radius:50%;background:${color};flex:none"></span>
          <span class="ds-name">${esc(sv.name)}</span>
          <div class="card-actions">
            <button class="icon-btn" title="${t('dash.toMap')}" onclick="openPeering(${sv.id})">${ICON_MAP}</button>
            <button class="icon-btn" title="${t('dash.quickTest')}" onclick="dashboard.quickTest(${sv.id})" ${live || !sv.enabled ? 'disabled' : ''}>${ICON_PLAY}</button>
          </div>
        </div>
        <div class="ds-badges">${badges}</div>
        ${liveBlock}
        <div class="ds-stats">
          ${stat(prefix + t('dir.download'), down == null ? '–' : fmt(down, 1) + ' Mbit/s', 'down')}
          ${stat(prefix + t('dir.upload'), up == null ? '–' : fmt(up, 1) + ' Mbit/s', 'up')}
          ${stat(t('dash.jitter'), st.avg_jitter_ms == null ? '–' : fmt(st.avg_jitter_ms, 2) + ' ms')}
          ${stat(t('dash.loss'), st.avg_packet_loss_percent == null ? '–' : fmt(st.avg_packet_loss_percent, 2) + ' %')}
          ${stat(t('dash.tests'), st.total_tests ? fmt(st.total_tests, 0) + (st.failed_tests ? ' ' + t('dash.failedShort', { n: st.failed_tests }) : '') : '–')}
          ${stat(t('dash.lastTest'), st.last_test_at ? fmtDate(st.last_test_at) : '–')}
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
      for (const x of [...running, ...pending]) this.liveIds.set(x.id, x.server_id);
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
      const test = await api('POST', '/tests/run', {
        server_id: sv.id,
        protocol: sv.default_protocol,
        direction: sv.default_direction,
        duration: sv.default_duration,
        parallel_streams: sv.default_parallel,
      });
      this.liveIds.set(test.id, sv.id);
      notify(t('dash.testStarted', { name: sv.name }));
      this.pollLive();
    } catch (e) {
      notify(t('test.startFailed', { msg: e.message }), 'err');
    }
  },
};
pages.dashboard = dashboard;

// ---------- Bestätigungsdialog ----------

// confirmDialog fragt eine Bestätigung ab und liefert true bei "OK". danger
// färbt die Bestätigung rot (Löschen); sonst erscheint sie als normale Aktion.
function confirmDialog(title, text, okLabel, danger = true, cancelLabel = t('cancel')) {
  const modal = document.getElementById('confirmModal');
  document.getElementById('confirmTitle').textContent = title;
  document.getElementById('confirmText').textContent = text;
  const ok = document.getElementById('confirmOk');
  const cancel = document.getElementById('confirmCancel');
  ok.textContent = okLabel;
  ok.classList.toggle('btn-danger', danger);
  cancel.textContent = cancelLabel;
  modal.hidden = false;
  cancel.focus();
  return new Promise(resolve => {
    const close = result => {
      modal.hidden = true;
      ok.onclick = cancel.onclick = modal.onclick = null;
      document.removeEventListener('keydown', onKey);
      resolve(result);
    };
    const onKey = ev => { if (ev.key === 'Escape') close(false); };
    ok.onclick = () => close(true);
    cancel.onclick = () => close(false);
    modal.onclick = ev => { if (ev.target === modal) close(false); };
    document.addEventListener('keydown', onKey);
  });
}

// ---------- Server-Profile ----------

const ICON_EDIT = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/></svg>';
const ICON_POWER = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.77.04"/></svg>';
const ICON_TRASH = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>';

const serversPage = {
  servers: [],
  publicServers: null,
  editing: null, // Server-ID beim Bearbeiten, sonst null

  enter() { this.load(); },
  onLang() {
    this.render();
    // Orte und Beschreibungen der öffentlichen Server neu laden.
    this.publicServers = null;
    const sel = this.val('sfPublic');
    while (sel.options.length > 1) sel.remove(1);
  },

  async load() {
    try {
      this.servers = await apiGet('/servers');
      this.render();
    } catch (e) {
      if (e.status !== 401) notify(t('servers.loadFailed', { msg: e.message }), 'err');
    }
  },

  render() {
    const grid = document.getElementById('serverGrid');
    if (!this.servers.length) {
      grid.innerHTML = `<div class="ds-card"><p class="muted">${t('servers.empty')}</p></div>`;
      return;
    }
    const row = (label, val) => `<div class="stat"><span class="stat-label">${label}</span><span class="stat-val">${val}</span></div>`;
    syncCards(grid, [...this.servers].sort((a, b) => a.name.localeCompare(b.name, LANG)).map(sv => {
      const color = serverColor(this.servers, sv.id);
      const udp = sv.default_protocol === 'udp'
        ? row(t('udp.rate'), sv.default_udp_bandwidth_mbps ? fmt(sv.default_udp_bandwidth_mbps, 1) + ' Mbit/s' : t('udp.default')) : '';
      return [sv.id, `
        <div class="ds-header">
          <span style="width:10px;height:10px;border-radius:50%;background:${color};flex:none"></span>
          <span class="ds-name">${esc(sv.name)}</span>
          <div class="card-actions">
            <button class="icon-btn" title="${t('servers.edit')}" onclick="serversPage.openForm(${sv.id})">${ICON_EDIT}</button>
            <button class="icon-btn" title="${sv.enabled ? t('servers.disable') : t('servers.enable')}" onclick="serversPage.toggle(${sv.id})">${ICON_POWER}</button>
            <button class="icon-btn" title="${t('delete')}" onclick="serversPage.remove(${sv.id})">${ICON_TRASH}</button>
          </div>
        </div>
        <div class="host">${esc(sv.host)}:${sv.port}</div>
        ${sv.description ? `<div class="desc">${esc(sv.description)}</div>` : ''}
        <div class="ds-badges">
          <span class="sev-badge ${sv.enabled ? 'sev-ok' : 'sev-crit'}">${sv.enabled ? t('badge.active') : t('badge.disabled')}</span>
          <span class="sev-badge sev-info">${sv.schedule_enabled ? t('badge.every', { n: sv.schedule_interval_minutes }) : t('servers.noSchedule')}</span>
          ${sv.schedule_enabled && sv.auto_trace_enabled ? `<span class="sev-badge sev-info">${t('servers.autoTrace')}</span>` : ''}
        </div>
        <div class="ds-stats">
          ${row(t('f.protocol'), protocolLabel(sv.default_protocol))}
          ${row(t('f.direction'), directionLabel(sv.default_direction))}
          ${row(t('servers.duration'), sv.default_duration + ' s')}
          ${row(t('servers.streams'), sv.default_parallel)}
          ${udp}
        </div>`];
    }));
    grid.querySelectorAll('.ds-card').forEach(card => {
      const sv = this.servers.find(s => String(s.id) === card.dataset.key);
      card.classList.toggle('inactive', !!sv && !sv.enabled);
    });
  },

  // ----- Dialog -----
  val(id) { return document.getElementById(id); },

  async openForm(id) {
    const sv = id ? this.servers.find(s => s.id === id) : null;
    this.editing = sv ? sv.id : null;
    const d = sv || {
      name: '', host: '', port: 5201, description: '', enabled: true,
      default_protocol: 'tcp', default_direction: 'download', default_duration: 10, default_parallel: 1,
      default_udp_bandwidth_mbps: null, schedule_enabled: false, schedule_interval_minutes: 30, auto_trace_enabled: false,
    };
    this.val('serverFormTitle').textContent = sv ? t('servers.formEdit') : t('servers.formAdd');
    this.val('sfName').value = d.name;
    this.val('sfHost').value = d.host;
    this.val('sfPort').value = d.port;
    this.val('sfDesc').value = d.description || '';
    this.val('sfProto').value = d.default_protocol;
    this.val('sfDir').value = d.default_direction;
    this.val('sfDuration').value = d.default_duration;
    this.val('sfParallel').value = d.default_parallel;
    this.val('sfUdp').value = d.default_udp_bandwidth_mbps ?? '';
    this.val('sfSchedule').checked = d.schedule_enabled;
    this.val('sfInterval').value = d.schedule_interval_minutes;
    this.val('sfAutoTrace').checked = d.auto_trace_enabled;
    this.val('sfEnabled').checked = d.enabled;
    this.val('sfPublic').value = '';
    this.val('publicPick').hidden = !!sv;
    this.showError('');
    this.syncForm();
    this.val('serverModal').hidden = false;
    this.val(sv ? 'sfName' : 'sfPublic').focus();
    document.addEventListener('keydown', this.onKey);
    if (!sv) this.loadPublicServers();
  },

  onKey(ev) { if (ev.key === 'Escape') serversPage.closeForm(); },

  closeForm() {
    this.val('serverModal').hidden = true;
    document.removeEventListener('keydown', this.onKey);
  },

  // syncForm blendet abhängige Felder ein oder aus (UDP-Rate, Intervall, Auto-Trace).
  syncForm() {
    const udp = this.val('sfProto').value === 'udp';
    const sched = this.val('sfSchedule').checked;
    this.val('sfUdpField').hidden = !udp;
    this.val('sfInterval').disabled = !sched;
    this.val('sfAutoTrace').disabled = !sched;
    this.val('sfAutoTraceRow').classList.toggle('disabled', !sched);
  },

  async loadPublicServers() {
    if (this.publicServers) return;
    try {
      this.publicServers = await apiGet('/public-servers');
      this.val('sfPublic').insertAdjacentHTML('beforeend', this.publicServers.map((p, i) =>
        `<option value="${i}">${esc(p.name)} – ${esc(p.location)} (${esc(p.host)})</option>`).join(''));
    } catch (e) {}
  },

  applyPublic(index) {
    const p = this.publicServers && this.publicServers[index];
    if (!p) return;
    this.val('sfName').value = p.name;
    this.val('sfHost').value = p.host;
    this.val('sfPort').value = p.port;
    this.val('sfDesc').value = p.description;
  },

  showError(msg) {
    const el = this.val('serverFormError');
    el.textContent = msg;
    el.hidden = !msg;
  },

  async save(ev) {
    ev.preventDefault();
    const num = id => { const v = this.val(id).value.trim(); return v === '' ? null : Number(v); };
    const body = {
      name: this.val('sfName').value.trim(),
      host: this.val('sfHost').value.trim(),
      port: num('sfPort') ?? 5201,
      description: this.val('sfDesc').value.trim() || null,
      default_protocol: this.val('sfProto').value,
      default_direction: this.val('sfDir').value,
      default_duration: num('sfDuration') ?? 10,
      default_parallel: num('sfParallel') ?? 1,
      default_udp_bandwidth_mbps: this.val('sfProto').value === 'udp' ? num('sfUdp') : null,
      schedule_enabled: this.val('sfSchedule').checked,
      schedule_interval_minutes: num('sfInterval') ?? 30,
      auto_trace_enabled: this.val('sfSchedule').checked && this.val('sfAutoTrace').checked,
      enabled: this.val('sfEnabled').checked,
    };
    if (!body.name || !body.host) { this.showError(t('servers.required')); return; }
    const btn = this.val('serverFormSave');
    btn.disabled = true;
    try {
      if (this.editing) await api('PUT', '/servers/' + this.editing, body);
      else await api('POST', '/servers', body);
      this.closeForm();
      notify(t('servers.saved', { name: body.name }));
      await this.load();
    } catch (e) {
      this.showError(e.message);
    } finally {
      btn.disabled = false;
    }
  },

  async toggle(id) {
    const sv = this.servers.find(s => s.id === id);
    if (!sv) return;
    try {
      await api('PUT', '/servers/' + id, { enabled: !sv.enabled });
      notify(t(sv.enabled ? 'servers.disabled' : 'servers.enabled', { name: sv.name }));
      await this.load();
    } catch (e) {
      notify(e.message, 'err');
    }
  },

  async remove(id) {
    const sv = this.servers.find(s => s.id === id);
    if (!sv) return;
    const ok = await confirmDialog(t('servers.deleteQ'), t('servers.deleteText', { name: sv.name }), t('delete'));
    if (!ok) return;
    try {
      await api('DELETE', '/servers/' + id);
      notify(t('servers.deleted', { name: sv.name }));
      await this.load();
    } catch (e) {
      notify(e.message, 'err');
    }
  },
};
pages.servers = serversPage;

// ---------- Tests ----------

const STATUS_CLASS = { pending: 'sev-info', running: 'sev-warn', completed: 'sev-ok', failed: 'sev-crit' };

function statusLabel(status) { return STRINGS['st.' + status] ? t('st.' + status) : esc(status); }
function statusBadge(status) {
  return `<span class="sev-badge ${STATUS_CLASS[status] || 'sev-info'}">${statusLabel(status)}</span>`;
}
function fmtMbps(v) { return v == null ? '–' : fmt(v, 1) + ' Mbit/s'; }
function fmtBytes(b) {
  if (b == null) return '–';
  const units = [['GB', 1e9], ['MB', 1e6], ['KB', 1e3]];
  for (const [u, f] of units) if (b >= f) return fmt(b / f, 2) + ' ' + u;
  return fmt(b, 0) + ' B';
}

const testsPage = {
  servers: [],
  tests: [],
  pageSize: 50,
  skip: 0,
  hasMore: false,
  // Live-Panel: verfolgter Test, sein Verlauf und das Ergebnis des letzten Tests.
  liveId: null,
  liveData: null,
  liveStart: 0,
  series: [],
  final: null,
  spark: null,
  timer: null,
  busy: false,

  async enter() {
    const panel = document.getElementById('livePanel');
    if (!panel.firstChild) {
      panel.innerHTML = '<div id="liveInfo" style="display:flex;flex-direction:column;flex:1"></div>'
        + '<div class="live-spark" id="liveSparkWrap" hidden><canvas id="liveSpark"></canvas></div>';
    }
    this.renderLive();
    await this.loadServers();
    this.reloadList();
    this.tick();
    this.timer = setInterval(() => this.tick(), 700);
  },
  leave() {
    clearInterval(this.timer);
    this.timer = null;
  },
  onTheme() {
    if (this.spark) { this.spark.destroy(); this.spark = null; }
    this.renderSpark();
  },
  onLang() {
    if (this.spark) { this.spark.destroy(); this.spark = null; }
    this.loadServers();
    this.renderLive();
    this.reloadList();
    // Fehlermeldungen kommen übersetzt vom Server, daher Ergebnis und Details neu laden.
    if (this.final) {
      const id = this.final.id;
      apiGet('/tests/' + id).then(f => { if (this.final && this.final.id === id) { this.final = f; this.renderLive(); } }).catch(() => {});
    }
    if (this.detail) {
      const id = this.detail.id;
      apiGet('/tests/' + id).then(d => { if (this.detail && this.detail.id === id) this.renderDetail(d); }).catch(() => {});
    }
  },

  // ----- Formular -----
  async loadServers() {
    try {
      this.servers = (await apiGet('/servers')).sort((a, b) => a.name.localeCompare(b.name, LANG));
    } catch (e) {
      return;
    }
    const sel = document.getElementById('tfServer');
    const prev = sel.value;
    const active = this.servers.filter(s => s.enabled);
    sel.innerHTML = active.length
      ? active.map(s => `<option value="${s.id}">${esc(s.name)} (${esc(s.host)})</option>`).join('')
      : `<option value="">${t('tests.noActive')}</option>`;
    sel.disabled = !active.length;
    document.getElementById('tfStart').disabled = !active.length;
    if (active.some(s => String(s.id) === prev)) sel.value = prev;
    else this.applyServerDefaults();

    const filter = document.getElementById('tlServer');
    const prevFilter = filter.value;
    filter.innerHTML = `<option value="">${t('tests.allServers')}</option>`
      + this.servers.map(s => `<option value="${s.id}">${esc(s.name)}</option>`).join('');
    filter.value = prevFilter;
  },

  // applyServerDefaults belegt das Formular mit den Vorgaben des gewählten Servers.
  applyServerDefaults() {
    const sv = this.servers.find(s => String(s.id) === document.getElementById('tfServer').value);
    if (!sv) return;
    document.getElementById('tfProto').value = sv.default_protocol;
    document.getElementById('tfDir').value = sv.default_direction;
    document.getElementById('tfDuration').value = sv.default_duration;
    document.getElementById('tfParallel').value = sv.default_parallel;
    document.getElementById('tfUdp').value = sv.default_udp_bandwidth_mbps ?? '';
    this.syncForm();
  },

  syncForm() {
    document.getElementById('tfUdpField').hidden = document.getElementById('tfProto').value !== 'udp';
    document.getElementById('tfParallelHint').hidden = !(Number(document.getElementById('tfParallel').value) > 4);
  },

  async start(ev) {
    ev.preventDefault();
    const num = id => { const v = document.getElementById(id).value.trim(); return v === '' ? null : Number(v); };
    const proto = document.getElementById('tfProto').value;
    const body = {
      server_id: Number(document.getElementById('tfServer').value),
      protocol: proto,
      direction: document.getElementById('tfDir').value,
      duration: num('tfDuration') ?? 10,
      parallel_streams: num('tfParallel') ?? 1,
    };
    if (proto === 'udp' && num('tfUdp') != null) body.udp_bandwidth_mbps = num('tfUdp');
    const btn = document.getElementById('tfStart');
    btn.disabled = true;
    try {
      const test = await api('POST', '/tests/run', body);
      notify(t('tests.started'));
      this.adopt(test.id);
      this.reloadList();
    } catch (e) {
      notify(t('test.startFailed', { msg: e.message }), 'err');
    } finally {
      btn.disabled = false;
    }
  },

  // ----- Live-Panel -----
  adopt(id) {
    this.liveId = id;
    this.liveData = null;
    this.final = null;
    this.series = [];
    this.liveStart = Date.now();
    if (this.spark) { this.spark.destroy(); this.spark = null; }
    this.tick();
  },

  // tick verfolgt den aktuellen Test. Ohne eigenen Test wird ein laufender
  // oder wartender Test übernommen (z. B. vom Scheduler gestartet).
  async tick() {
    if (this.busy) return;
    this.busy = true;
    try {
      if (!this.liveId) {
        const [running, pending] = await Promise.all([
          apiGet('/tests?status=running&limit=1'),
          apiGet('/tests?status=pending&limit=1'),
        ]);
        const next = running[0] || pending[0];
        if (!next) return;
        this.liveId = next.id;
        this.liveData = null;
        this.final = null;
        this.series = [];
        this.liveStart = Date.now();
        if (this.spark) { this.spark.destroy(); this.spark = null; }
      }
      const id = this.liveId;
      const l = await apiGet(`/tests/${id}/live`);
      this.liveData = l;
      if (l.status === 'running') {
        this.series.push({ x: (Date.now() - this.liveStart) / 1000, down: l.current_download_mbps, up: l.current_upload_mbps });
      }
      if (l.status === 'completed' || l.status === 'failed') {
        this.final = await apiGet('/tests/' + id);
        this.liveId = null;
        this.reloadList();
      }
      this.renderLive();
    } catch (e) {
      if (e.status === 404) this.liveId = null;
    } finally {
      this.busy = false;
    }
  },

  renderLive() {
    const info = document.getElementById('liveInfo');
    if (!info) return;
    const l = this.liveData;
    const f = this.final;
    const metrics = (down, up) => `<div class="live-metrics">
        <div class="live-metric down"><div class="lbl">${t('dir.download')}</div><div class="num">${down == null ? '–' : fmt(down, 1)}<small>Mbit/s</small></div></div>
        <div class="live-metric up"><div class="lbl">${t('dir.upload')}</div><div class="num">${up == null ? '–' : fmt(up, 1)}<small>Mbit/s</small></div></div>
      </div>`;

    if (f) {
      const sv = this.servers.find(s => s.id === f.server_id);
      const failed = f.status === 'failed';
      info.innerHTML = `
        <div class="live-head"><span class="live-title">${t('tests.result', { name: esc(sv ? sv.name : 'Server ' + f.server_id) })}</span>
          <span class="live-badge ${failed ? 'failed' : 'done'}">${failed ? t('tests.badgeFailed') : t('tests.badgeDone')}</span></div>
        <div class="bar-bg"><div class="bar-fill ${failed ? 'failed' : ''}" style="width:100%"></div></div>
        ${failed ? `<div class="error-box">${esc(f.error_message || t('unknownError'))}</div>` : metrics(f.download_bandwidth_mbps, f.upload_bandwidth_mbps)}
        <div class="live-line"><span>${fmtDate(f.completed_at || f.created_at)}</span>
          <a href="#" onclick="testsPage.openDetail(${f.id}); return false">${t('tests.details')}</a></div>`;
    } else if (l) {
      const pending = l.status === 'pending';
      const remaining = Math.max(0, l.total_seconds - l.elapsed_seconds);
      info.innerHTML = `
        <div class="live-head"><span class="live-title">${pending ? t('tests.waitingSlot') : t('tests.runningAgainst', { name: esc(l.server_name) })}</span>
          <span class="live-badge ${pending ? 'idle' : ''}">${pending ? t('tests.badgeWaiting') : 'LIVE'}</span></div>
        <div class="live-line"><span>${t('tests.elapsed', { e: l.elapsed_seconds, r: remaining })}</span><span>${l.progress} %</span></div>
        <div class="bar-bg"><div class="bar-fill" style="width:${l.progress}%"></div></div>
        ${metrics(pending ? null : l.current_download_mbps, pending ? null : l.current_upload_mbps)}`;
    } else {
      info.innerHTML = `
        <div class="live-head"><span class="live-title">${t('tests.liveTitle')}</span><span class="live-badge idle">${t('tests.badgeReady')}</span></div>
        <div class="live-empty"><div>${t('tests.noTest')}</div>
          <div style="font-size:.85em">${t('tests.noTestHint')}</div></div>`;
    }
    this.renderSpark();
  },

  renderSpark() {
    const wrap = document.getElementById('liveSparkWrap');
    if (!wrap) return;
    wrap.hidden = this.series.length < 2 || !window.Chart;
    if (wrap.hidden) return;
    const down = this.series.map(p => ({ x: p.x, y: p.down }));
    const up = this.series.map(p => ({ x: p.x, y: p.up }));
    if (this.spark) {
      this.spark.data.datasets[0].data = down;
      this.spark.data.datasets[1].data = up;
      this.spark.update('none');
      return;
    }
    const line = (label, data, color) => ({ label, data, borderColor: color, backgroundColor: color, borderWidth: 2, pointRadius: 0, tension: 0.3 });
    this.spark = new Chart(document.getElementById('liveSpark'), {
      type: 'line',
      data: { datasets: [line(t('dir.download'), down, '#3b82f6'), line(t('dir.upload'), up, '#10b981')] },
      options: {
        responsive: true, maintainAspectRatio: false, animation: false,
        scales: {
          x: { type: 'linear', ticks: { callback: v => fmt(v, 0) + ' s', maxTicksLimit: 6 }, grid: { color: GRID } },
          y: { beginAtZero: true, ticks: { callback: v => fmt(v, 0), maxTicksLimit: 4 }, grid: { color: GRID } },
        },
        plugins: {
          legend: { display: false },
          tooltip: { callbacks: { title: () => '', label: i => `${i.dataset.label}: ${fmt(i.raw.y, 1)} Mbit/s` } },
        },
      },
    });
  },

  // ----- Verlauf -----
  reloadList() {
    this.skip = 0;
    this.tests = [];
    this.loadMore();
  },

  async loadMore() {
    const params = new URLSearchParams({ limit: this.pageSize, skip: this.skip });
    const server = document.getElementById('tlServer').value;
    const status = document.getElementById('tlStatus').value;
    if (server) params.set('server_id', server);
    if (status) params.set('status', status);
    try {
      const page = await apiGet('/tests?' + params);
      this.tests = this.skip === 0 ? page : this.tests.concat(page);
      this.skip += page.length;
      this.hasMore = page.length === this.pageSize;
      this.renderList();
    } catch (e) {
      if (e.status !== 401) notify(t('tests.loadFailed', { msg: e.message }), 'err');
    }
  },

  renderList() {
    const name = id => { const s = this.servers.find(x => x.id === id); return s ? s.name : 'Server ' + id; };
    document.getElementById('testRows').innerHTML = this.tests.length
      ? this.tests.map(x => `<tr onclick="testsPage.openDetail(${x.id})">
          <td>${fmtDate(x.created_at)}</td><td>${esc(name(x.server_id))}</td><td>${protocolLabel(x.protocol)}</td>
          <td>${directionLabel(x.direction)}</td><td>${x.parallel_streams}</td>
          <td class="num">${fmtMbps(x.download_bandwidth_mbps)}</td><td class="num">${fmtMbps(x.upload_bandwidth_mbps)}</td>
          <td>${statusBadge(x.status)}</td></tr>`).join('')
      : `<tr><td colspan="8" class="muted" style="text-align:center;padding:24px">${t('tests.none')}</td></tr>`;
    document.getElementById('tlMore').hidden = !this.hasMore;
  },

  // ----- Details -----
  async openDetail(id) {
    let test;
    try {
      test = await apiGet('/tests/' + id);
    } catch (e) {
      notify(e.message, 'err');
      return;
    }
    this.renderDetail(test);
    document.getElementById('testModal').hidden = false;
    document.addEventListener('keydown', this.onKey);
  },

  renderDetail(x) {
    this.detail = x;
    const row = (label, val) => `<div class="stat"><span class="stat-label">${label}</span><span class="stat-val">${val}</span></div>`;
    const runtime = x.started_at && x.completed_at
      ? fmt((Date.parse(x.completed_at) - Date.parse(x.started_at)) / 1000, 1) + ' s' : '–';
    const udp = x.protocol === 'udp';
    document.getElementById('testModalTitle').textContent = `Test #${x.id} – ${x.server.name}`;
    document.getElementById('testModalBody').innerHTML = `
      <div class="ds-badges">${statusBadge(x.status)}<span class="badge-type">${protocolLabel(x.protocol)}</span>
        <span class="sev-badge sev-info">${directionLabel(x.direction)}</span><span class="sev-badge sev-info">${streamsLabel(x.parallel_streams)}</span></div>
      ${x.error_message ? `<div class="error-box">${esc(x.error_message)}</div>` : ''}
      <div class="detail-grid">
        ${row(t('f.server'), `${esc(x.server.host)}:${x.server.port}`)}
        ${row(t('tests.created'), fmtDate(x.created_at))}
        ${row(t('tests.startedAt'), fmtDate(x.started_at))}
        ${row(t('tests.runtime'), t('tests.planned', { runtime, n: x.duration }))}
        ${row(t('dir.download'), fmtMbps(x.download_bandwidth_mbps))}
        ${row(t('dir.upload'), fmtMbps(x.upload_bandwidth_mbps))}
        ${row(t('tests.bytesDown'), fmtBytes(x.download_bytes))}
        ${row(t('tests.bytesUp'), fmtBytes(x.upload_bytes))}
        ${udp ? row(t('udp.rate'), x.udp_bandwidth_mbps ? fmtMbps(x.udp_bandwidth_mbps) : t('udp.default')) : ''}
        ${udp ? row(t('tests.jitter'), [x.download_jitter_ms, x.upload_jitter_ms].filter(v => v != null).map(v => fmt(v, 2) + ' ms').join(' / ') || '–') : ''}
        ${udp ? row(t('tests.loss'), [x.download_packet_loss_percent, x.upload_packet_loss_percent].filter(v => v != null).map(v => fmt(v, 2) + ' %').join(' / ') || '–') : ''}
        ${row(t('tests.retransmits'), x.retransmits == null ? '–' : fmt(x.retransmits, 0))}
        ${row(t('tests.cpu'), x.cpu_percent == null ? '–' : fmt(x.cpu_percent, 1) + ' %')}
      </div>
      ${x.raw_output ? `<details class="raw"><summary>${t('tests.raw')}</summary><pre>${esc(x.raw_output)}</pre></details>` : ''}`;
    document.getElementById('testModalDelete').onclick = () => this.deleteTest(x);
  },

  onKey(ev) {
    // Escape im Bestätigungsdialog schließt nur diesen.
    if (ev.key === 'Escape' && document.getElementById('confirmModal').hidden) testsPage.closeDetail();
  },

  closeDetail() {
    this.detail = null;
    document.getElementById('testModal').hidden = true;
    document.removeEventListener('keydown', this.onKey);
  },

  async deleteTest(x) {
    const ok = await confirmDialog(t('tests.deleteQ'), t('tests.deleteText', { id: x.id, name: x.server.name }), t('delete'));
    if (!ok) return;
    try {
      await api('DELETE', '/tests/' + x.id);
      this.closeDetail();
      if (this.final && this.final.id === x.id) { this.final = null; this.series = []; this.renderLive(); }
      notify(t('tests.deleted', { id: x.id }));
      this.reloadList();
    } catch (e) {
      notify(e.message, 'err');
    }
  },
};
pages.tests = testsPage;

// ---------- Peering-Map ----------

// OpenStreetMap-Standardkacheln (ohne API-Schlüssel; CARTO verlangt inzwischen
// einen). Die dunkle Darstellung entsteht per CSS-Filter auf der Kachelebene.
const TILE_URL = 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
const tileAttribution = () => '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
  + (LANG === 'de' ? '-Mitwirkende' : ' contributors');
const HOP_COLORS = { first: '#10b981', dest: '#ef4444', est: '#f59e0b', hop: '#3b82f6' };

const peering = {
  map: null,
  tiles: null,
  layer: null,
  servers: [],
  traces: [],
  serverId: null,
  trace: null,         // angezeigter gespeicherter Trace
  live: null,          // { source, host, hops } während einer Live-Traceroute

  async enter() {
    this.initMap();
    await this.load();
    if (pendingPeeringServer != null) {
      const id = pendingPeeringServer;
      pendingPeeringServer = null;
      this.selectServer(id);
      if (!this.live) this.startLive();
    }
  },

  initMap() {
    if (!window.L) {
      this.mapMessage(t('peering.noLeaflet'));
      return;
    }
    if (!this.map) {
      this.map = L.map('pmMap', { worldCopyJump: true }).setView([50.5, 10], 4);
      this.layer = L.layerGroup().addTo(this.map);
      this.setTiles();
    }
    setTimeout(() => this.map.invalidateSize(), 0);
  },

  onLang() {
    this.setRunning(!!this.live);
    // Die Kartenquelle steht in der Attribution; Kachelebene neu anlegen.
    if (this.tiles) { this.tiles.remove(); this.tiles = null; }
    this.setTiles();
    if (!window.L) this.mapMessage(t('peering.noLeaflet'));
    if (this.live) this.show();
    this.load();
  },

  setTiles() {
    if (!this.map || this.tiles) return;
    this.tiles = L.tileLayer(TILE_URL, { attribution: tileAttribution(), maxZoom: 19 }).addTo(this.map);
  },

  mapMessage(text) {
    const el = document.getElementById('pmMapEmpty');
    el.textContent = text || '';
    el.hidden = !text;
  },

  async load() {
    try {
      const [servers, traces] = await Promise.all([apiGet('/servers'), apiGet('/traces?limit=500')]);
      this.servers = servers.sort((a, b) => a.name.localeCompare(b.name, LANG));
      this.traces = traces;
    } catch (e) {
      if (e.status !== 401) notify(t('peering.loadFailed', { msg: e.message }), 'err');
      return;
    }
    const sel = document.getElementById('pmServer');
    sel.innerHTML = this.servers.length
      ? this.servers.map(s => `<option value="${s.id}">${t('peering.traceCount', { name: esc(s.name), n: this.tracesFor(s).length })}</option>`).join('')
      : `<option value="">${t('peering.noServers')}</option>`;
    document.getElementById('pmStart').disabled = !this.servers.length && !this.live;
    if (!this.servers.some(s => s.id === this.serverId)) this.serverId = this.servers[0] ? this.servers[0].id : null;
    if (this.serverId != null) sel.value = this.serverId;
    if (!this.live) {
      const list = this.tracesFor(this.server());
      if (!this.trace || !list.some(x => x.id === this.trace.id)) this.trace = list[0] || null;
      else this.trace = list.find(x => x.id === this.trace.id);
      this.show();
    }
    this.renderList();
  },

  server() { return this.servers.find(s => s.id === this.serverId); },
  tracesFor(sv) { return sv ? this.traces.filter(x => x.destination_host === sv.host) : []; },

  selectServer(id) {
    if (this.live) return;
    this.serverId = Number(id);
    document.getElementById('pmServer').value = this.serverId;
    this.trace = this.tracesFor(this.server())[0] || null;
    this.renderList();
    this.show();
  },

  selectTrace(id) {
    if (this.live) return;
    this.trace = this.traces.find(x => x.id === id) || null;
    this.renderList();
    this.show();
  },

  renderList() {
    const list = document.getElementById('pmTraceList');
    const traces = this.tracesFor(this.server());
    if (!this.server()) { list.innerHTML = `<p class="muted">${t('peering.addFirst')}</p>`; return; }
    if (!traces.length) { list.innerHTML = `<p class="muted">${t('peering.noTraces')}</p>`; return; }
    list.innerHTML = traces.map(tr => `
      <button class="trace-item ${this.trace && this.trace.id === tr.id && !this.live ? 'active' : ''}" onclick="peering.selectTrace(${tr.id})" ${this.live ? 'disabled' : ''}>
        <div class="t">${fmtDate(tr.created_at)}</div>
        <div class="m">${t('peering.hops', { n: tr.total_hops })}${tr.total_rtt_ms != null ? ' · ' + fmt(tr.total_rtt_ms, 1) + ' ms' : ''}${tr.test_id ? ' · Test #' + tr.test_id : ''}${tr.completed ? '' : ` · <span style="color:var(--crit)">${t('peering.incomplete')}</span>`}</div>
      </button>`).join('');
  },

  // show zeigt den Live-Trace bzw. den gewählten gespeicherten Trace auf Karte und in den Details.
  show() {
    const hops = this.live ? this.live.hops : (this.trace ? this.trace.hops : []);
    this.drawMap(hops);
    this.renderDetails(hops);
    this.renderOverlay();
  },

  // hopKind bestimmt die Markierung: erster Hop, Ziel, geschätzter Standort oder normal.
  hopKind(h, i, hops) {
    if (!h.responded) return 'timeout';
    const destIP = this.live ? null : this.trace && this.trace.destination_ip;
    const isLast = i === hops.length - 1 && (!this.live || this.live.done);
    if ((destIP && h.ip_address === destIP) || (isLast && h.responded)) return 'dest';
    if (i === hops.findIndex(x => x.responded)) return 'first';
    if (h.geoip_interpolated) return 'est';
    return 'hop';
  },

  drawMap(hops) {
    if (!this.map) return;
    this.layer.clearLayers();
    const geo = hops.map((h, i) => [h, i]).filter(([h]) => h.latitude != null && h.longitude != null);
    if (!geo.length) {
      this.mapMessage(hops.length ? t('peering.noGeo') : t(this.live ? 'peering.waiting' : 'peering.noTrace'));
      return;
    }
    this.mapMessage('');
    const coords = geo.map(([h]) => [h.latitude, h.longitude]);
    L.polyline(coords, { color: '#3b82f6', weight: 3, opacity: 0.75, dashArray: '10, 6' }).addTo(this.layer);
    for (const [h, i] of geo) {
      const kind = this.hopKind(h, i, hops);
      const icon = L.divIcon({
        className: 'hop-icon', iconSize: [26, 26], iconAnchor: [13, 13],
        html: `<div class="hop-marker" style="background:${HOP_COLORS[kind] || HOP_COLORS.hop}">${h.hop_number}</div>`,
      });
      const place = [h.city, h.country].filter(Boolean).join(', ') || t('unknown');
      L.marker([h.latitude, h.longitude], { icon }).bindPopup(
        `<strong>${t('peering.hop', { n: h.hop_number })}</strong>${h.geoip_interpolated ? ` <span style="color:#f59e0b">${t('peering.estimated')}</span>` : ''}<br>`
        + `${esc(h.ip_address)}${h.hostname ? '<br>' + esc(h.hostname) : ''}<br>`
        + `RTT: ${h.rtt_ms != null ? fmt(h.rtt_ms, 1) + ' ms' : '–'}<br>${t('peering.locationRow', { place: esc(place) })}`).addTo(this.layer);
    }
    this.map.fitBounds(L.latLngBounds(coords), { padding: [50, 50], maxZoom: 9 });
  },

  renderOverlay() {
    const el = document.getElementById('pmOverlay');
    if (!this.live) { el.hidden = true; return; }
    const located = this.live.hops.filter(h => h.latitude != null).length;
    el.hidden = false;
    el.innerHTML = `<span class="live-badge">LIVE</span><span>${esc(this.live.host)}</span><span class="sep">|</span>`
      + `<span>${t('peering.hops', { n: this.live.hops.length })}</span><span class="sep">|</span><span>${t('peering.located', { n: located })}</span>`;
  },

  renderDetails(hops) {
    const box = document.getElementById('pmDetails');
    const tr = this.live ? null : this.trace;
    box.hidden = !this.live && !tr;
    if (box.hidden) return;

    document.getElementById('pmDetailTitle').textContent = this.live
      ? t('peering.liveTitle', { host: this.live.host })
      : t('peering.traceFrom', { date: fmtDate(tr.created_at) });
    document.getElementById('pmDetailActions').innerHTML = tr
      ? `<button class="btn btn-secondary" onclick="peering.deleteTrace(${tr.id})">${ICON_TRASH} ${t('peering.deleteTrace')}</button>` : '';

    const stat = (label, val) => `<div class="stat"><span class="stat-label">${label}</span><span class="stat-val">${val}</span></div>`;
    const lastRtt = [...hops].reverse().find(h => h.rtt_ms != null);
    document.getElementById('pmSummary').innerHTML = [
      stat(t('peering.dest'), esc(this.live ? this.live.host : tr.destination_host) + (tr && tr.destination_ip && tr.destination_ip !== tr.destination_host ? ` (${esc(tr.destination_ip)})` : '')),
      tr && tr.source_ip ? stat(t('peering.source'), esc(tr.source_ip)) : '',
      stat(t('peering.hopsLabel'), hops.length),
      stat(t('peering.rttDest'), tr && tr.total_rtt_ms != null ? fmt(tr.total_rtt_ms, 1) + ' ms' : (lastRtt ? fmt(lastRtt.rtt_ms, 1) + ' ms' : '–')),
      stat(t('peering.status'), this.live ? t('peering.running') : tr.completed ? t('peering.complete') : `<span style="color:var(--crit)">${esc(tr.error_message || t('peering.incomplete'))}</span>`),
      tr && tr.test_id ? stat(t('peering.test'), '#' + tr.test_id) : '',
    ].join('');

    const label = { first: t('peering.kindFirst'), dest: t('peering.kindDest'), est: t('peering.kindEst'), timeout: t('peering.kindTimeout') };
    document.getElementById('pmChain').innerHTML = hops.map((h, i) => {
      const kind = this.hopKind(h, i, hops);
      return `<span class="hop-chip ${kind}" title="${label[kind] || ''}"><span class="n">${h.hop_number}</span>${h.responded ? esc(h.ip_address) : '*'}</span>`;
    }).join('<span class="path-arrow">→</span>') || `<span class="muted">${t('peering.noHopsYet')}</span>`;

    document.getElementById('pmHops').innerHTML = hops.map(h => {
      const place = [h.city, h.country_code || h.country].filter(Boolean).join(', ');
      return `<tr>
        <td>${h.hop_number}</td>
        <td class="mono">${h.responded ? esc(h.ip_address) : `<span class="muted">${t('peering.noReply')}</span>`}</td>
        <td>${h.hostname ? esc(h.hostname) : '–'}</td>
        <td class="num">${h.rtt_ms != null ? fmt(h.rtt_ms, 1) + ' ms' : '–'}</td>
        <td>${place ? esc(place) : '–'}${h.geoip_interpolated ? ` <span class="sev-badge sev-warn">${t('peering.kindEst')}</span>` : ''}</td>
      </tr>`;
    }).join('') || `<tr><td colspan="5" class="muted" style="text-align:center;padding:20px">${t('peering.noHops')}</td></tr>`;
  },

  // ----- Live-Traceroute (Server-Sent Events) -----
  toggleLive() {
    if (this.live) this.stopLive(true);
    else this.startLive();
  },

  startLive() {
    const sv = this.server();
    if (!sv) return;
    const url = `/api/live-trace/stream/${encodeURIComponent(sv.host)}?token=${encodeURIComponent(storageGet('iperf-token') || '')}&lang=${LANG}`;
    const source = new EventSource(url);
    this.live = { source, host: sv.host, hops: [], done: false };
    this.setRunning(true);
    this.show();

    source.onmessage = ev => {
      let msg;
      try { msg = JSON.parse(ev.data); } catch (e) { return; }
      if (!this.live || this.live.source !== source) return;
      switch (msg.type) {
        case 'hop':
          this.live.hops.push(msg.data);
          this.show();
          break;
        case 'interpolation_complete':
          this.live.hops = msg.hops;
          this.live.done = true;
          this.show();
          break;
        case 'complete':
          this.finishLive(msg.trace_id);
          break;
        case 'error':
          notify(t('peering.failed', { msg: msg.message }), 'err');
          this.stopLive(false);
          break;
      }
    };
    // Ohne close() würde EventSource nach Verbindungsende neu verbinden und
    // damit eine weitere Traceroute starten.
    source.onerror = () => {
      if (this.live && this.live.source === source) {
        notify(t('peering.lost'), 'err');
        this.stopLive(false);
      } else {
        source.close();
      }
    };
  },

  async finishLive(traceId) {
    this.live.source.close();
    this.live = null;
    this.setRunning(false);
    notify(t('peering.done'));
    await this.load();
    if (traceId) this.selectTrace(traceId);
  },

  stopLive(byUser) {
    if (!this.live) return;
    this.live.source.close();
    this.live = null;
    this.setRunning(false);
    if (byUser) notify(t('peering.aborted'));
    this.renderList();
    this.show();
  },

  setRunning(running) {
    const btn = document.getElementById('pmStart');
    btn.textContent = running ? t('peering.abort') : t('peering.start');
    btn.classList.toggle('btn-danger', running);
    document.getElementById('pmServer').disabled = running;
  },

  async deleteTrace(id) {
    const ok = await confirmDialog(t('peering.deleteQ'), t('peering.deleteText'), t('delete'));
    if (!ok) return;
    try {
      await api('DELETE', '/traces/' + id);
      notify(t('peering.deleted'));
      this.trace = null;
      await this.load();
    } catch (e) {
      notify(e.message, 'err');
    }
  },
};
pages.peering = peering;

// ---------- Passwort ändern ----------

const passwordDialog = {
  el: id => document.getElementById(id),

  open() {
    ['pwCurrent', 'pwNew', 'pwRepeat'].forEach(id => { this.el(id).value = ''; });
    this.error('');
    this.el('passwordModal').hidden = false;
    this.el('pwCurrent').focus();
    document.addEventListener('keydown', this.onKey);
  },
  onKey(ev) { if (ev.key === 'Escape') passwordDialog.close(); },
  close() {
    this.el('passwordModal').hidden = true;
    document.removeEventListener('keydown', this.onKey);
  },
  error(msg) {
    this.el('passwordError').textContent = msg;
    this.el('passwordError').hidden = !msg;
  },

  async save(ev) {
    ev.preventDefault();
    const current = this.el('pwCurrent').value;
    const next = this.el('pwNew').value;
    if (next !== this.el('pwRepeat').value) { this.error(t('pw.mismatch')); return; }
    const btn = this.el('passwordSave');
    btn.disabled = true;
    try {
      await api('POST', '/auth/change-password', { current_password: current, new_password: next });
      this.close();
      document.querySelectorAll('.toast-warn').forEach(el => el.remove());
      notify(t('pw.changed'));
    } catch (e) {
      this.error(e.message);
    } finally {
      btn.disabled = false;
    }
  },
};

// ---------- Administration ----------

const adminPage = {
  adminOnly: true,
  users: [],
  servers: [],

  enter() {
    this.syncCleanup();
    this.load();
  },
  onLang() { this.load(); },

  async load() {
    try {
      const [stats, users, servers] = await Promise.all([
        apiGet('/admin/stats/database'), apiGet('/auth/users'), apiGet('/servers'),
      ]);
      this.users = users;
      this.servers = servers.sort((a, b) => a.name.localeCompare(b.name, LANG));
      this.renderSummary(stats);
      this.renderUsers();
      const sel = document.getElementById('ctServer');
      const prev = sel.value;
      sel.innerHTML = `<option value="">${t('admin.allServers')}</option>`
        + this.servers.map(s => `<option value="${s.id}">${esc(s.name)}</option>`).join('');
      sel.value = prev;
    } catch (e) {
      if (e.status !== 401) notify(t('admin.loadFailed', { msg: e.message }), 'err');
    }
  },

  renderSummary(st) {
    const day = iso => iso ? new Date(iso).toLocaleDateString(locale()) : '–';
    const card = (val, lbl, sub = '') =>
      `<div class="sum-card"><div class="val">${val}</div><div class="lbl">${esc(lbl)}</div>${sub ? `<div class="sub">${esc(sub)}</div>` : ''}</div>`;
    document.getElementById('adminSummary').innerHTML =
      card(fmt(st.total_tests, 0), t('dash.tests'), st.oldest_test ? `${day(st.oldest_test)} – ${day(st.newest_test)}` : t('admin.none')) +
      card(fmt(st.total_traces, 0), 'Traces', t('admin.hops', { n: fmt(st.total_hops, 0) })) +
      card(fmt(st.total_servers, 0), t('admin.servers')) +
      card(fmt(st.total_users, 0), t('admin.users'));
  },

  renderUsers() {
    document.getElementById('userRows').innerHTML = this.users.map(u => {
      const self = currentUser && u.id === currentUser.id;
      return `<tr>
        <td><strong style="color:var(--heading)">${esc(u.username)}</strong>${self ? ` <span class="sev-badge sev-info">${t('admin.signedIn')}</span>` : ''}</td>
        <td>${esc(u.email)}</td>
        <td>${u.is_admin ? `<span class="badge-type">${t('admin.administrator')}</span>` : t('admin.user')}${u.is_active ? '' : ` <span class="sev-badge sev-crit">${t('admin.locked')}</span>`}</td>
        <td>${fmtDate(u.created_at)}</td>
        <td>${u.last_login ? fmtDate(u.last_login) : '–'}</td>
        <td style="text-align:right">${self ? '' : `<button class="icon-btn" title="${t('admin.deleteUser')}" onclick="adminPage.deleteUser(${u.id})">${ICON_TRASH}</button>`}</td>
      </tr>`;
    }).join('');
  },

  // ----- Benutzer anlegen -----
  openUserForm() {
    ['ufName', 'ufMail', 'ufPass'].forEach(id => { document.getElementById(id).value = ''; });
    document.getElementById('ufAdmin').checked = false;
    this.userError('');
    document.getElementById('userModal').hidden = false;
    document.getElementById('ufName').focus();
    document.addEventListener('keydown', this.onKey);
  },
  onKey(ev) { if (ev.key === 'Escape') adminPage.closeUserForm(); },
  closeUserForm() {
    document.getElementById('userModal').hidden = true;
    document.removeEventListener('keydown', this.onKey);
  },
  userError(msg) {
    const el = document.getElementById('userFormError');
    el.textContent = msg;
    el.hidden = !msg;
  },

  async saveUser(ev) {
    ev.preventDefault();
    const body = {
      username: document.getElementById('ufName').value.trim(),
      email: document.getElementById('ufMail').value.trim(),
      password: document.getElementById('ufPass').value,
      is_admin: document.getElementById('ufAdmin').checked,
    };
    const btn = document.getElementById('userFormSave');
    btn.disabled = true;
    try {
      await api('POST', '/auth/register', body);
      this.closeUserForm();
      notify(t('admin.userCreated', { name: body.username }));
      this.load();
    } catch (e) {
      this.userError(e.message);
    } finally {
      btn.disabled = false;
    }
  },

  async deleteUser(id) {
    const u = this.users.find(x => x.id === id);
    if (!u) return;
    if (!await confirmDialog(t('admin.deleteUserQ'), t('admin.deleteUserText', { name: u.username }), t('delete'))) return;
    try {
      await api('DELETE', '/auth/users/' + id);
      notify(t('admin.userDeleted', { name: u.username }));
      this.load();
    } catch (e) {
      notify(e.message, 'err');
    }
  },

  // ----- Bereinigung -----
  syncCleanup() {
    document.getElementById('ctDaysField').hidden = document.getElementById('ctScope').value !== 'days';
    document.getElementById('crDaysField').hidden = document.getElementById('crScope').value !== 'days';
  },

  days(id) {
    const v = parseInt(document.getElementById(id).value, 10);
    return isNaN(v) || v < 0 ? null : v;
  },

  async cleanupTests() {
    const scope = document.getElementById('ctScope').value;
    const serverId = document.getElementById('ctServer').value;
    const sv = this.servers.find(s => String(s.id) === serverId);
    const params = new URLSearchParams();
    let what = sv ? t('admin.allTestsOf', { name: sv.name }) : t('admin.allTests');
    if (scope === 'days') {
      const d = this.days('ctDays');
      if (d == null) { notify(t('admin.invalidDays'), 'err'); return; }
      params.set('days', d);
      what = t('admin.testsOlder', { what, n: d });
    } else if (!sv) {
      params.set('all', 'true');
    }
    if (sv) params.set('server_id', sv.id);
    if (!await confirmDialog(t('admin.deleteTestsQ'), t('admin.deleteTestsText', { what }), t('delete'))) return;
    try {
      const res = await api('DELETE', '/admin/cleanup/tests?' + params);
      notify(t('admin.testsDeleted', { n: fmt(res.deleted_count, 0) }));
      this.load();
    } catch (e) {
      notify(e.message, 'err');
    }
  },

  async cleanupTraces() {
    const params = new URLSearchParams();
    let what;
    if (document.getElementById('crScope').value === 'days') {
      const d = this.days('crDays');
      if (d == null) { notify(t('admin.invalidDays'), 'err'); return; }
      params.set('days', d);
      what = t('admin.tracesOlder', { n: d });
    } else {
      params.set('all', 'true');
      what = t('admin.allTraces');
    }
    if (!await confirmDialog(t('admin.deleteTracesQ'), t('admin.deleteTracesText', { what }), t('delete'))) return;
    try {
      const res = await api('DELETE', '/admin/cleanup/traces?' + params);
      notify(t('admin.tracesDeleted', { traces: fmt(res.deleted_traces, 0), hops: fmt(res.deleted_hops, 0) }));
      this.load();
    } catch (e) {
      notify(e.message, 'err');
    }
  },
};
pages.admin = adminPage;

// ---------- Start ----------
(async function init() {
  loadInfo();
  if (!storageGet('iperf-token')) { showLogin(); return; }
  try {
    showApp(await apiGet('/auth/me'));
  } catch (e) {
    showLogin(e.status === 401 ? '' : t('login.unreachable', { msg: e.message }));
  }
})();
