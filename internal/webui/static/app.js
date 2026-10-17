(() => {
  'use strict';
  const MAX_ROWS = 500;
  const $ = (id) => document.getElementById(id);
  const hdr = { 'X-Requested-With': 'pve-mcp-ui', 'Content-Type': 'application/json' };
  const state = { filter: 'all', search: '', paused: false, held: [], entries: [], seen: new Set(), es: null };

  const kindGroup = (k) => ({
    TOOL_MUTATION: 'mutations', TOOL_DRY_RUN: 'dryruns', TOOL_READ: 'reads', RESOURCE_READ: 'reads',
    PROMPT_GET: 'reads', TOOL_ERROR: 'errors', SERVER_LOG: 'logs',
  }[k] || 'logs');
  const kindClass = (k) => ({
    TOOL_MUTATION: 'k-mut', TOOL_DRY_RUN: 'k-dry', TOOL_ERROR: 'k-err',
    TOOL_READ: 'k-read', RESOURCE_READ: 'k-read', PROMPT_GET: 'k-read',
  }[k] || 'k-log');

  async function api(path, opts = {}) {
    const res = await fetch(path, { credentials: 'same-origin', headers: hdr, ...opts });
    if (res.status === 401) { showLogin(); throw new Error('unauthorized'); }
    return res;
  }

  function showLogin() {
    if (state.es) { state.es.close(); state.es = null; }
    $('app').hidden = true; $('login').hidden = false; $('token').focus();
  }
  function showApp() {
    $('login').hidden = true; $('app').hidden = false;
    connect(); refreshStatus();
  }

  function matches(e) {
    if (state.filter !== 'all' && kindGroup(e.kind) !== state.filter) return false;
    if (!state.search) return true;
    const q = state.search.toLowerCase();
    return (e.message + ' ' + (e.detail || '') + ' ' + e.kind).toLowerCase().includes(q);
  }

  function rowFor(e) {
    const tr = document.createElement('tr');
    const t = document.createElement('td'); t.className = 't';
    t.textContent = new Date(e.time).toLocaleTimeString();
    const k = document.createElement('td'); k.className = 'k ' + kindClass(e.kind);
    k.textContent = e.level && e.kind === 'SERVER_LOG' ? e.level : e.kind.replace('TOOL_', '').replace('_', '-');
    const m = document.createElement('td');
    if (e.detail) {
      const d = document.createElement('details');
      const s = document.createElement('summary'); s.textContent = e.message;
      const p = document.createElement('pre'); p.textContent = e.detail;
      d.append(s, p); m.append(d);
    } else { m.textContent = e.message; }
    const ms = document.createElement('td'); ms.className = 'd';
    ms.textContent = e.duration_ms ? String(e.duration_ms) : '';
    tr.append(t, k, m, ms);
    return tr;
  }

  function render() {
    const rows = $('rows');
    rows.replaceChildren(...state.entries.filter(matches).map(rowFor));
    $('empty').hidden = rows.children.length > 0;
  }

  function ingest(e) {
    if (state.seen.has(e.id)) return;
    state.seen.add(e.id);
    state.entries.push(e);
    if (state.entries.length > MAX_ROWS) {
      const old = state.entries.shift(); state.seen.delete(old.id);
    }
    if (matches(e)) {
      $('rows').append(rowFor(e));
      while ($('rows').children.length > MAX_ROWS) $('rows').firstChild.remove();
      $('empty').hidden = true;
    }
  }

  function connect() {
    if (state.es) state.es.close();
    const es = new EventSource('/api/logs/stream');
    state.es = es;
    es.onopen = () => { $('conn').textContent = 'live'; $('conn').className = 'badge ok'; };
    es.onmessage = (ev) => {
      const e = JSON.parse(ev.data);
      if (state.paused) { state.held.push(e); $('pause').textContent = 'Resume (' + state.held.length + ')'; } else ingest(e);
    };
    es.onerror = () => {
      $('conn').textContent = 'reconnecting'; $('conn').className = 'badge warn';
      // A closed stream (401 or server gone) will not retry on its own.
      if (es.readyState === EventSource.CLOSED) {
        api('/api/status').then(() => connect()).catch(() => {});
      }
    };
  }

  const fmtBytes = (n) => n >= 1048576 ? (n / 1048576).toFixed(1) + ' MB' : Math.round(n / 1024) + ' KB';
  const fmtUp = (s) => s >= 3600 ? Math.floor(s / 3600) + 'h ' + Math.floor(s % 3600 / 60) + 'm' : Math.floor(s / 60) + 'm ' + (s % 60) + 's';

  async function refreshStatus() {
    try {
      const s = await (await api('/api/status')).json();
      const pve = $('pve');
      pve.textContent = s.pve.ok ? 'PVE ' + s.pve.latency_ms + ' ms' : 'PVE unreachable';
      pve.title = s.pve.error || '';
      pve.className = 'badge ' + (s.pve.ok ? 'ok' : 'bad');
      $('transport').textContent = s.transport.toUpperCase();
      $('tools').textContent = s.tools + ' tools';
      $('heap').textContent = 'heap ' + fmtBytes(s.heap_bytes);
      $('uptime').textContent = 'up ' + fmtUp(s.uptime_seconds);
      if (document.activeElement !== $('level')) $('level').value = s.log_level;
    } catch (_) { /* login screen already shown */ }
  }

  $('login-form').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const res = await fetch('/api/auth/login', { method: 'POST', headers: hdr, credentials: 'same-origin', body: JSON.stringify({ token: $('token').value }) });
    $('token').value = '';
    if (res.ok) { $('login-error').textContent = ''; showApp(); } else { $('login-error').textContent = 'Invalid token'; }
  });
  $('logout').addEventListener('click', async () => {
    await fetch('/api/auth/logout', { method: 'POST', headers: hdr, credentials: 'same-origin' });
    showLogin();
  });
  $('filters').addEventListener('click', (ev) => {
    const b = ev.target.closest('button'); if (!b) return;
    state.filter = b.dataset.filter;
    for (const x of $('filters').children) x.classList.toggle('on', x === b);
    render();
  });
  $('search').addEventListener('input', (ev) => { state.search = ev.target.value.trim(); render(); });
  $('pause').addEventListener('click', () => {
    state.paused = !state.paused;
    if (!state.paused) { state.held.splice(0).forEach(ingest); $('pause').textContent = 'Pause'; } else $('pause').textContent = 'Resume';
  });
  $('clear').addEventListener('click', async () => {
    await api('/api/logs/clear', { method: 'POST' });
    state.entries = []; state.held = []; state.seen.clear(); render();
  });
  $('level').addEventListener('change', async (ev) => {
    await api('/api/loglevel', { method: 'PATCH', body: JSON.stringify({ level: ev.target.value }) });
    refreshStatus();
  });

  setInterval(() => { if (!$('app').hidden) refreshStatus(); }, 10000);
  api('/api/status').then(() => showApp()).catch(() => showLogin());
})();
