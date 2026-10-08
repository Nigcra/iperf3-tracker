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
