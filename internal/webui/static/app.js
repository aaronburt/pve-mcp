(() => {
  'use strict';
  const MAX_ROWS = 500;
  const $ = (id) => document.getElementById(id);
  const hdr = { 'X-Requested-With': 'pve-mcp-ui', 'Content-Type': 'application/json' };
  const state = { filter: 'all', search: '', paused: false, held: [], entries: [], seen: new Set(), open: new Set(), es: null, capacity: 250 };

  const kindGroup = (k) => ({
    TOOL_MUTATION: 'mutations', TOOL_DRY_RUN: 'dryruns', TOOL_READ: 'reads', RESOURCE_READ: 'reads',
    PROMPT_GET: 'reads', TOOL_ERROR: 'errors', SERVER_LOG: 'logs',
  }[k] || 'logs');
  const kindClass = (e) => {
    if (e.kind === 'SERVER_LOG' && e.level === 'warn') return 'k-warn';
    return ({
      TOOL_MUTATION: 'k-mut', TOOL_DRY_RUN: 'k-dry', TOOL_ERROR: 'k-err',
      TOOL_READ: 'k-read', RESOURCE_READ: 'k-read', PROMPT_GET: 'k-prompt',
    }[e.kind] || 'k-log');
  };
  const kindLabel = (e) => {
    if (e.kind === 'SERVER_LOG') return e.level ? 'LOG ' + e.level.toUpperCase() : 'LOG';
    return ({
      TOOL_MUTATION: 'MUTATION', TOOL_DRY_RUN: 'DRY-RUN', TOOL_READ: 'READ',
      RESOURCE_READ: 'RESOURCE', PROMPT_GET: 'PROMPT', TOOL_ERROR: 'ERROR',
    }[e.kind]) || e.kind;
  };

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

  function chevron() {
    const span = document.createElement('span');
    span.className = 'chev';
    span.setAttribute('aria-hidden', 'true');
    span.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>';
    return span;
  }

  function rowFor(e) {
    const row = document.createElement('div');
    row.className = 'evrow';
    const line = document.createElement('div');
    line.className = 'evline';

    const t = document.createElement('span');
    t.className = 'time mono';
    t.textContent = new Date(e.time).toLocaleTimeString();

    const k = document.createElement('span');
    k.className = 'kind ' + kindClass(e);
    const dot = document.createElement('i');
    k.append(dot, document.createTextNode(kindLabel(e)));

    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'evtoggle';

    const text = document.createElement('span');
    text.className = 'evtext';
    const msg = document.createElement('span');
    msg.className = 'msg';
    msg.textContent = e.message;
    text.append(msg);
    if (e.detail) {
      const det = document.createElement('span');
      det.className = 'det';
      det.textContent = e.detail;
      text.append(det);
    }
    toggle.append(text, chevron());

    const panel = document.createElement('dl');
    panel.className = 'evpanel';
    const addRow = (label, value, block) => {
      const dt = document.createElement('dt');
      dt.textContent = label;
      const dd = document.createElement('dd');
      if (block) {
        const pre = document.createElement('pre');
        pre.textContent = value;
        dd.append(pre);
      } else {
        dd.className = 'mono';
        dd.textContent = value;
      }
      panel.append(dt, dd);
    };
    addRow('Kind', kindLabel(e), false);
    addRow('Duration', e.duration_ms ? e.duration_ms + ' ms' : 'Not timed', false);
    if (e.level) addRow('Level', e.level, false);
    if (e.detail) addRow('Detail', e.detail, true);

    const open = state.open.has(e.id);
    row.classList.toggle('open', open);
    toggle.setAttribute('aria-expanded', String(open));
    panel.hidden = !open;
    toggle.addEventListener('click', () => {
      const next = !row.classList.contains('open');
      row.classList.toggle('open', next);
      toggle.setAttribute('aria-expanded', String(next));
      panel.hidden = !next;
      if (next) state.open.add(e.id); else state.open.delete(e.id);
    });

    line.append(t, k, toggle);
    row.append(line, panel);
    return row;
  }

  function updateCounts() {
    const count = (kind) => state.entries.filter((e) => e.kind === kind).length;
    $('n-mut').textContent = count('TOOL_MUTATION');
    $('n-dry').textContent = count('TOOL_DRY_RUN');
    $('n-err').textContent = count('TOOL_ERROR');
    $('buf').textContent = 'Buffer ' + state.entries.length + ' of ' + state.capacity;
  }

  function render() {
    const rows = $('rows');
    rows.replaceChildren(...state.entries.filter(matches).map(rowFor));
    $('empty').hidden = rows.children.length > 0;
    updateCounts();
  }

  function ingest(e) {
    if (state.seen.has(e.id)) return;
    state.seen.add(e.id);
    state.entries.push(e);
    if (state.entries.length > MAX_ROWS) {
      const old = state.entries.shift(); state.seen.delete(old.id); state.open.delete(old.id);
    }
    if (matches(e)) {
      $('rows').append(rowFor(e));
      while ($('rows').children.length > MAX_ROWS) $('rows').firstChild.remove();
      $('empty').hidden = true;
    }
    updateCounts();
  }

  function connect() {
    if (state.es) state.es.close();
    const es = new EventSource('/api/logs/stream');
    state.es = es;
    es.onopen = () => { $('conn').textContent = 'Live'; $('conn').className = 'ok'; };
    es.onmessage = (ev) => {
      const e = JSON.parse(ev.data);
      if (state.paused) { state.held.push(e); $('pause').textContent = 'Resume (' + state.held.length + ')'; } else ingest(e);
    };
    es.onerror = () => {
      $('conn').textContent = 'Reconnecting'; $('conn').className = 'warn';
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
      pve.textContent = s.pve.ok ? s.pve.latency_ms + ' ms' : 'Down';
      pve.className = 'num ' + (s.pve.ok ? 'ok' : 'bad');
      $('pve-sub').textContent = s.pve.ok ? 'Reachable' : (s.pve.error || 'Unreachable');
      $('transport').textContent = s.transport.toUpperCase();
      $('tools').textContent = s.tools + ' tools';
      $('heap').textContent = 'Heap ' + fmtBytes(s.heap_bytes);
      $('uptime').textContent = 'Up ' + fmtUp(s.uptime_seconds);
      if (s.buffer && s.buffer.capacity) state.capacity = s.buffer.capacity;
      updateCounts();
      if (document.activeElement !== $('level')) $('level').value = s.log_level;
    } catch (_) {}
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
  $('search-toggle').addEventListener('click', () => {
    const opening = $('search-wrap').hidden;
    $('search-wrap').hidden = !opening;
    $('search-toggle').setAttribute('aria-expanded', String(opening));
    if (opening) {
      $('search').focus();
    } else {
      $('search').value = '';
      state.search = '';
      render();
    }
  });
  $('search').addEventListener('input', (ev) => { state.search = ev.target.value.trim(); render(); });
  $('pause').addEventListener('click', () => {
    state.paused = !state.paused;
    if (!state.paused) { state.held.splice(0).forEach(ingest); $('pause').textContent = 'Pause'; } else $('pause').textContent = 'Resume';
  });
  $('clear').addEventListener('click', async () => {
    await api('/api/logs/clear', { method: 'POST' });
    state.entries = []; state.held = []; state.seen.clear(); state.open.clear(); render();
  });
  $('level').addEventListener('change', async (ev) => {
    await api('/api/loglevel', { method: 'PATCH', body: JSON.stringify({ level: ev.target.value }) });
    refreshStatus();
  });

  setInterval(() => { if (!$('app').hidden) refreshStatus(); }, 10000);
  api('/api/status').then(() => showApp()).catch(() => showLogin());
})();
