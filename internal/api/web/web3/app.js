/* ============================================================
   TMD Web v3 - app.js
   Apple-inspired management UI for Twitter Media Downloader
   ============================================================ */

/* ---- Constants ---- */
const API_BASE = '';
const PAGES = ['dashboard', 'tasks', 'data', 'schedules', 'config', 'logs'];
const PAGE_TITLES = {
  dashboard: 'Dashboard', tasks: 'Tasks', data: 'Data',
  schedules: 'Schedules', config: 'Settings', logs: 'Logs'
};
const TASK_TYPE_LABELS = {
  user_download: 'User Download', list_download: 'List Download',
  following_download: 'Following Download', profile_download: 'Profile Download',
  mark_downloaded: 'Mark Downloaded', json_file_download: 'JSON File Import',
  json_folder_download: 'Folder Import', batch_download: 'Batch Download',
  list_profile: 'List Profile', retry_all_failed: 'Retry All Failed'
};
const SCHEDULE_LABELS = { list: 'List', user: 'User', following: 'Following', mixed: 'Mixed' };
const STATUS_CLASS = { completed: 'badge-completed', running: 'badge-running', queued: 'badge-queued', failed: 'badge-failed', cancelled: 'badge-cancelled' };

/* ---- State ---- */
const state = {
  currentPage: 'dashboard', tasks: [],
  taskStats: { queued: 0, running: 0, completed: 0, failed: 0, cancelled: 0, total: 0 },
  health: null, sseConnected: false,
  dataActiveTab: 'users', schedules: [], schedulerRunning: false
};

/* ---- API Client ---- */
const API = {
  async _fetch(url, opts) {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 30000);
    try {
      const r = await fetch(url, { ...opts, signal: ctrl.signal });
      if (!r.ok && r.headers.get('content-type')?.includes('json')) { const j = await r.json(); throw new Error(j.error || `HTTP ${r.status}`); }
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return r;
    } catch (e) { if (e.name === 'AbortError') throw new Error('Request timed out'); throw e; }
    finally { clearTimeout(timer); }
  },
  async _json(url, opts) { const r = await this._fetch(url, opts); const j = await r.json(); if (!j.success) throw new Error(j.error || 'Request failed'); return j.data; },
  get: url => API._json(API_BASE + url),
  post: (url, body) => API._json(API_BASE + url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined }),
  put: (url, body) => API._json(API_BASE + url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined }),
  patch: (url, body) => API._json(API_BASE + url, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined }),
  del: url => API._json(API_BASE + url, { method: 'DELETE' }),
};

/* ---- Utilities ---- */
const $ = id => document.getElementById(id);
const esc = s => { if (s == null) return ''; const d = document.createElement('div'); d.appendChild(document.createTextNode(String(s))); return d.innerHTML; };
const plural = (n, s) => n + ' ' + (n === 1 ? s : s + 's');
const relTime = iso => {
  if (!iso) return '-'; const d = new Date(iso); if (isNaN(d.getTime())) return '-';
  const diff = Date.now() - d.getTime(); const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now'; if (mins < 60) return plural(mins, 'min') + ' ago';
  const hrs = Math.floor(mins / 60); if (hrs < 24) return plural(hrs, 'hr') + ' ago';
  return plural(Math.floor(hrs / 24), 'day') + ' ago';
};
const fmtTime = iso => { if (!iso) return '-'; const d = new Date(iso); return isNaN(d.getTime()) ? '-' : d.toLocaleString(); };
const fmtDur = (s, e) => {
  if (!s) return '-'; const secs = Math.max(0, Math.floor(((e ? new Date(e) : new Date()) - new Date(s)) / 1000));
  if (secs < 60) return secs + 's'; return Math.floor(secs / 60) + 'm ' + (secs % 60) + 's';
};
const stripAnsi = s => s.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '');
const logLevel = l => { if (/ERRO\[|level=e(rror)?/.test(l)) return 'error'; if (/WARN\[|level=w(arn)?/.test(l)) return 'warn'; if (/INFO\[|level=i(nfo)?/.test(l)) return 'info'; if (/DEBU\[|level=d(ebug)?/.test(l)) return 'debug'; return ''; };
const tLabel = t => TASK_TYPE_LABELS[t] || t;

/* ---- Toast ---- */
function toast(msg, type) {
  const c = $('toastContainer'); const el = document.createElement('div');
  el.className = 'toast ' + (type || 'info');
  el.innerHTML = '<span class="toast-message">' + esc(msg) + '</span><button class="toast-close" onclick="this.parentElement.classList.add(\'out\');setTimeout(()=>this.parentElement.remove(),200)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>';
  c.appendChild(el);
  setTimeout(() => { el.classList.add('out'); setTimeout(() => el.remove(), 200); }, 3500);
}

/* ---- Modal ---- */
function openModal(title, bodyHTML, footHTML) {
  $('modalTitle').textContent = title; $('modalBody').innerHTML = bodyHTML; $('modalFoot').innerHTML = footHTML || '';
  $('modalMask').classList.add('open'); $('modalSheet').classList.add('open');
  $('modalSheet').setAttribute('aria-hidden', 'false'); $('modalMask').setAttribute('aria-hidden', 'false');
}
function closeModal() { $('modalMask').classList.remove('open'); $('modalSheet').classList.remove('open'); $('modalSheet').setAttribute('aria-hidden', 'true'); $('modalMask').setAttribute('aria-hidden', 'true'); }
$('modalClose').onclick = closeModal; $('modalMask').onclick = closeModal;
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeModal(); });

/* ---- Navigation ---- */
function navigate(page) {
  if (!PAGES.includes(page)) page = 'dashboard';
  state.currentPage = page;
  history.pushState(null, '', page === 'dashboard' ? '/' : '/' + page);
  updateNav();
  renderPage(page);
}
function updateNav() {
  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === state.currentPage);
    if (el.dataset.page === state.currentPage) el.setAttribute('aria-current', 'page');
    else el.removeAttribute('aria-current');
  });
  $('pageTitle').textContent = PAGE_TITLES[state.currentPage] || '';
  document.title = 'TMD - ' + (PAGE_TITLES[state.currentPage] || '');
}
function renderPage(page) {
  $('pageContent').innerHTML = '<div class="page-loader"><div class="spinner"></div></div>';
  const fns = { dashboard: renderDashboard, tasks: renderTasks, data: renderData, schedules: renderSchedules, config: renderConfig, logs: renderLogs };
  if (fns[page]) fns[page]();
}
document.querySelectorAll('.nav-item').forEach(el => { el.addEventListener('click', () => navigate(el.dataset.page)); el.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); navigate(el.dataset.page); } }); });
$('menuBtn').addEventListener('click', () => $('sidebar').classList.toggle('open'));
$('sidebarOverlay').addEventListener('click', () => $('sidebar').classList.remove('open'));
window.addEventListener('popstate', () => { const p = (location.pathname.replace(/^\//, '') || 'dashboard'); const page = PAGES.includes(p) ? p : 'dashboard'; state.currentPage = page; updateNav(); renderPage(page); });

/* ---- Theme ---- */
$('themeBtn').addEventListener('click', () => {
  const h = document.documentElement; const n = h.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
  h.setAttribute('data-theme', n); localStorage.setItem('tmd-theme', n);
});
if (localStorage.getItem('tmd-theme')) document.documentElement.setAttribute('data-theme', localStorage.getItem('tmd-theme'));

/* ============================================================
   PAGE: Dashboard
   ============================================================ */
async function renderDashboard() {
  try {
    const [health, stats] = await Promise.all([
      API.get('/api/v1/health').catch(() => null),
      API.get('/api/v1/tasks/stats').catch(() => null),
    ]);
    state.health = health; if (stats) state.taskStats = stats;
    updateHealth();
    const queue = await API.get('/api/v1/queue/status').catch(() => null);
    const tasksData = await API.get('/api/v1/tasks').catch(() => null);
    const recent = Array.isArray(tasksData?.tasks) ? tasksData.tasks.slice(0, 8) : [];
    const s = stats || {};

    $('pageContent').innerHTML = `
      <div class="stat-grid">
        <div class="stat-tile"><div class="stat-tile-icon accent"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="3" width="7" height="9"/><rect x="14" y="3" width="7" height="5"/><rect x="14" y="12" width="7" height="9"/><rect x="3" y="16" width="7" height="5"/></svg></div><div class="stat-tile-content"><div class="stat-tile-value">${s.total ?? '-'}</div><div class="stat-tile-label">Total Tasks</div></div></div>
        <div class="stat-tile"><div class="stat-tile-icon success"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div><div class="stat-tile-content"><div class="stat-tile-value">${s.completed ?? '-'}</div><div class="stat-tile-label">Completed</div></div></div>
        <div class="stat-tile"><div class="stat-tile-icon ${s.running > 0 ? 'accent' : 'success'}"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></div><div class="stat-tile-content"><div class="stat-tile-value">${s.running ?? '-'}</div><div class="stat-tile-label">Running${s.queued > 0 ? ' (' + s.queued + ' queued)' : ''}</div></div></div>
        <div class="stat-tile"><div class="stat-tile-icon ${s.failed > 0 ? 'danger' : 'success'}"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg></div><div class="stat-tile-content"><div class="stat-tile-value">${s.failed ?? '-'}</div><div class="stat-tile-label">Failed</div></div></div>
      </div>
      <div class="dash-cols">
        <div class="panel">
          <div class="panel-head"><span class="panel-title">System</span></div>
          <div class="panel-body padded" style="display:grid;grid-template-columns:auto 1fr;gap:6px 16px;font-size:12.5px">
            <span style="color:var(--text-tertiary)">Status</span><span>${health ? '<span class="badge badge-completed">Online</span>' : '<span class="badge badge-failed">Offline</span>'}</span>
            <span style="color:var(--text-tertiary)">Version</span><span class="text-mono">${health?.version || '-'}</span>
            <span style="color:var(--text-tertiary)">SSE</span><span id="dashSseStatus"><span class="badge ${state.sseConnected ? 'badge-completed' : 'badge-failed'}">${state.sseConnected ? 'Connected' : 'Disconnected'}</span></span>
          </div>
        </div>
        <div class="panel">
          <div class="panel-head"><span class="panel-title">Queue</span></div>
          <div class="panel-body padded">${queue ? `<div class="queue-grid">
            <div class="queue-stat"><div class="queue-stat-value">${queue.queue_depth}</div><div class="queue-stat-label">Depth</div></div>
            <div class="queue-stat"><div class="queue-stat-value">${queue.active_jobs}</div><div class="queue-stat-label">Active</div></div>
            <div class="queue-stat"><div class="queue-stat-value">${queue.pending_jobs}</div><div class="queue-stat-label">Pending</div></div>
            <div class="queue-stat"><div class="queue-stat-value">${queue.detached_jobs}</div><div class="queue-stat-label">Detached</div></div>
          </div>` : '<div class="empty-state"><div class="empty-desc">Unavailable</div></div>'}</div>
        </div>
      </div>
      ${recent.length ? `<div class="section"><div class="section-header"><span class="section-title">Recent Tasks</span><div class="section-actions"><button class="btn btn-sm btn-ghost" onclick="navigate('tasks')">View All</button></div></div><div class="panel">${renderTaskList(recent)}</div></div>`
        : `<div class="section"><div class="section-header"><span class="section-title">Tasks</span></div><div class="panel"><div class="empty-state"><div class="empty-icon"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div><div class="empty-title">No tasks yet</div><div class="empty-desc">Start a download from the Tasks page</div></div></div>`}
    `;
  } catch (e) {
    $('pageContent').innerHTML = `<div class="empty-state"><div class="empty-title">Unable to load dashboard</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

/* ============================================================
   PAGE: Tasks
   ============================================================ */
async function renderTasks() {
  try {
    const [td, stats] = await Promise.all([
      API.get('/api/v1/tasks'), API.get('/api/v1/tasks/stats').catch(() => null)
    ]);
    const tasks = Array.isArray(td?.tasks) ? td.tasks : [];
    if (stats) state.taskStats = stats; state.tasks = tasks; updateBadge();

    $('pageContent').innerHTML = `
      <div class="section">
        <div class="section-header">
          <span class="section-title">${plural(tasks.length, 'task')}</span>
          <div class="section-actions">
            <button class="btn btn-sm btn-primary" onclick="showNewTaskModal()"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> New</button>
            <button class="btn btn-sm btn-secondary" onclick="cancelAllQueued()">Cancel Queued</button>
            <button class="btn btn-sm btn-ghost" onclick="retryAllFailed()">Retry Failed</button>
          </div>
        </div>
        <div class="panel">${tasks.length ? renderTaskList(tasks) : `<div class="empty-state"><div class="empty-icon"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div><div class="empty-title">No tasks</div><div class="empty-desc">Create a new download task to get started</div></div></div>}
      </div>`;
    attachTaskEvents();
  } catch (e) { $('pageContent').innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load tasks</div><div class="empty-desc">${esc(e.message)}</div></div>`; }
}

function renderTaskList(tasks) {
  return '<div class="list">' + tasks.map(t => {
    const p = t.progress || {}; const pct = p.total > 0 ? Math.min(100, Math.round((p.completed||0)/p.total*100)) : 0;
    return `<div class="list-item" data-id="${esc(t.task_id)}">
      <span class="badge ${STATUS_CLASS[t.status]||'badge-queued'}" style="flex-shrink:0">${t.status}</span>
      <div class="list-item-content">
        <div class="list-item-title">${esc(tLabel(t.type))}${t.data?.screen_name ? ': ' + esc(t.data.screen_name) : ''}${t.data?.list_id ? ' (list)' : ''}</div>
        <div class="list-item-meta">
          <span class="text-mono">${esc(t.task_id.substring(0,20))}</span>
          ${t.created_at ? '<span>' + relTime(t.created_at) + '</span>' : ''}
          ${p.stage ? '<span>' + esc(p.stage) + '</span>' : ''}
          ${p.current ? '<span>' + esc(p.current) + '</span>' : ''}
          ${t.started_at && t.ended_at ? '<span>' + fmtDur(t.started_at, t.ended_at) + '</span>' : ''}
        </div>
        ${(t.status === 'running' || t.status === 'queued') && p.total > 0 ? `<div class="mt-2"><div class="progress-track"><div class="progress-fill" style="width:${pct}%"></div></div><div style="font-size:10px;color:var(--text-tertiary);margin-top:2px">${p.completed||0}/${p.total}${p.failed ? ' (' + p.failed + ' failed)' : ''}</div></div>` : ''}
      </div>
      <div class="list-item-actions">
        ${t.status === 'running' || t.status === 'queued' ? `<button class="btn btn-xs btn-ghost task-cancel" title="Cancel"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>` : ''}
        ${t.status === 'failed' || t.status === 'cancelled' ? `<button class="btn btn-xs btn-ghost task-retry" title="Retry"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg></button>` : ''}
        ${['completed','failed','cancelled'].includes(t.status) ? `<button class="btn btn-xs btn-ghost task-delete" title="Delete"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>` : ''}
        <button class="btn btn-xs btn-ghost task-detail" title="Info"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg></button>
      </div>
    </div>`;
  }).join('') + '</div>';
}

function attachTaskEvents() {
  document.querySelectorAll('.task-cancel').forEach(el => { el.addEventListener('click', async e => { e.stopPropagation(); const id = el.closest('.list-item')?.dataset.id; if (id) try { await API.post('/api/v1/tasks/' + id + '/cancel'); toast('Cancelled', 'warning'); renderTasks(); } catch (ex) { toast(ex.message, 'error'); } }); });
  document.querySelectorAll('.task-retry').forEach(el => { el.addEventListener('click', async e => { e.stopPropagation(); const id = el.closest('.list-item')?.dataset.id; if (id) try { await API.post('/api/v1/tasks/' + id + '/retry'); toast('Retry queued', 'success'); renderTasks(); } catch (ex) { toast(ex.message, 'error'); } }); });
  document.querySelectorAll('.task-delete').forEach(el => { el.addEventListener('click', async e => { e.stopPropagation(); const id = el.closest('.list-item')?.dataset.id; if (id && confirm('Delete?')) try { await API.del('/api/v1/tasks/' + id); toast('Deleted', 'info'); renderTasks(); } catch (ex) { toast(ex.message, 'error'); } }); });
  document.querySelectorAll('.task-detail').forEach(el => { el.addEventListener('click', async e => { e.stopPropagation(); const id = el.closest('.list-item')?.dataset.id; if (id) showTaskDetail(id); }); });
}

async function showTaskDetail(id) {
  try {
    const t = await API.get('/api/v1/tasks/' + id); const p = t.progress || {}; const r = t.result || {};
    const pct = p.total > 0 ? Math.min(100, Math.round((p.completed||0)/p.total*100)) : 0;
    openModal('Task: ' + esc(t.task_id), `
      <div class="detail-grid">
        <span class="detail-label">ID</span><span class="detail-value text-mono">${esc(t.task_id)}</span>
        <span class="detail-label">Type</span><span class="detail-value">${esc(tLabel(t.type))}</span>
        <span class="detail-label">Status</span><span class="detail-value"><span class="badge ${STATUS_CLASS[t.status]}">${t.status}</span></span>
        <span class="detail-label">Created</span><span class="detail-value">${fmtTime(t.created_at)}</span>
        ${t.started_at ? '<span class="detail-label">Started</span><span class="detail-value">' + fmtTime(t.started_at) + '</span>' : ''}
        ${t.ended_at ? '<span class="detail-label">Ended</span><span class="detail-value">' + fmtTime(t.ended_at) + '</span>' : ''}
        ${t.started_at ? '<span class="detail-label">Duration</span><span class="detail-value">' + fmtDur(t.started_at, t.ended_at) + '</span>' : ''}
        ${p.stage ? '<span class="detail-label">Stage</span><span class="detail-value">' + esc(p.stage) + '</span>' : ''}
      </div>
      ${p.total > 0 ? `<div class="mt-3"><div class="progress-track"><div class="progress-fill" style="width:${pct}%"></div></div><div class="text-sm text-muted mt-2">${p.completed||0}/${p.total}${p.failed ? ' (' + p.failed + ' failed)' : ''}</div></div>` : ''}
      ${r.main || r.profile ? `<div class="mt-3" style="padding:12px;background:var(--bg-secondary);border-radius:var(--radius);font-size:12px;display:grid;grid-template-columns:auto 1fr;gap:4px 16px">
        ${r.main ? '<span class="text-muted">Downloaded</span><span>' + (r.main.downloaded||0) + ' files</span>' : ''}
        ${r.main?.failed ? '<span class="text-muted">Failed</span><span style="color:var(--danger-text)">' + r.main.failed + '</span>' : ''}
        ${r.profile ? '<span class="text-muted">Profile</span><span>' + (r.profile.downloaded||0) + ' dl' + (r.profile.failed?', '+r.profile.failed+' fail':'') + (r.profile.versioned?', '+r.profile.versioned+' ver':'') + '</span>' : ''}
        ${r.message ? '<span class="text-muted">Note</span><span>' + esc(r.message) + '</span>' : ''}
      </div>` : ''}
      ${t.error ? '<div class="mt-3" style="padding:10px 12px;background:var(--danger-bg);border-radius:var(--radius);font-size:11.5px;font-family:var(--font-mono);color:var(--danger-text)">' + esc(t.error) + '</div>' : ''}
    `, `
      ${t.status === 'running' || t.status === 'queued' ? '<button class="btn btn-sm btn-danger" onclick="closeModal();cancelTask(\'' + esc(t.task_id) + '\')">Cancel</button>' : ''}
      ${t.status === 'failed' || t.status === 'cancelled' ? '<button class="btn btn-sm btn-primary" onclick="closeModal();retryTask(\'' + esc(t.task_id) + '\')">Retry</button>' : ''}
      <button class="btn btn-sm btn-ghost" onclick="closeModal()">Close</button>
    `);
  } catch (e) { toast(e.message, 'error'); }
}
async function cancelTask(id) { try { await API.post('/api/v1/tasks/' + id + '/cancel'); toast('Cancelled', 'warning'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }
async function retryTask(id) { try { await API.post('/api/v1/tasks/' + id + '/retry'); toast('Retry queued', 'success'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }
async function cancelAllQueued() { try { await API.post('/api/v1/tasks/cancel-queued'); toast('Queued tasks cancelled', 'warning'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }
async function retryAllFailed() { try { await API.post('/api/v1/errors/retry'); toast('Retrying failed items', 'info'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }

/* ---- New Task Modal ---- */
function showNewTaskModal() {
  openModal('New Task', `
    <div class="segmented" style="margin-bottom:16px">
      <button class="segmented-item active" data-stab="user" onclick="switchTaskTab('user')">User</button>
      <button class="segmented-item" data-stab="list" onclick="switchTaskTab('list')">List</button>
      <button class="segmented-item" data-stab="following" onclick="switchTaskTab('following')">Following</button>
      <button class="segmented-item" data-stab="batch" onclick="switchTaskTab('batch')">Batch</button>
      <button class="segmented-item" data-stab="profile" onclick="switchTaskTab('profile')">Profile</button>
      <button class="segmented-item" data-stab="json" onclick="switchTaskTab('json')">JSON</button>
      <button class="segmented-item" data-stab="mark" onclick="switchTaskTab('mark')">Mark</button>
    </div>
    <div id="newTaskForm">${ntForm('user')}</div>
  `, '<button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-sm btn-primary" onclick="submitNewTask()">Create</button>');
  window._nt = 'user';
}
function switchTaskTab(t) {
  window._nt = t;
  document.querySelectorAll('[data-stab]').forEach(el => el.classList.toggle('active', el.dataset.stab === t));
  $('newTaskForm').innerHTML = ntForm(t);
}
function ntForm(t) {
  const m = {
    user: '<div class="field"><label class="field-label">Screen Name</label><input class="field-input" id="ntSn" placeholder="elonmusk"></div><div class="field-row"><label class="field-checkbox"><input type="checkbox" id="ntAf"> Auto-follow protected</label><label class="field-checkbox"><input type="checkbox" id="ntSp"> Skip profile</label></div><label class="field-checkbox"><input type="checkbox" id="ntNr"> No retry</label>',
    list: '<div class="field"><label class="field-label">List ID</label><input class="field-input" id="ntLid" placeholder="1234567890"></div><div class="field-row"><label class="field-checkbox"><input type="checkbox" id="ntAf"> Auto-follow</label><label class="field-checkbox"><input type="checkbox" id="ntFm"> Follow members</label></div><div class="field-row"><label class="field-checkbox"><input type="checkbox" id="ntSp"> Skip profile</label><label class="field-checkbox"><input type="checkbox" id="ntNr"> No retry</label></div>',
    following: '<div class="field"><label class="field-label">Screen Name</label><input class="field-input" id="ntSn" placeholder="elonmusk"></div><div class="field-row"><label class="field-checkbox"><input type="checkbox" id="ntAf"> Auto-follow</label><label class="field-checkbox"><input type="checkbox" id="ntSp"> Skip profile</label></div><label class="field-checkbox"><input type="checkbox" id="ntNr"> No retry</label>',
    batch: '<div class="field"><label class="field-label">Users (comma separated)</label><input class="field-input" id="ntBu" placeholder="user1, user2"></div><div class="field-row"><div class="field"><label class="field-label">List IDs</label><input class="field-input" id="ntBl" placeholder="123, 456"></div><div class="field"><label class="field-label">Following</label><input class="field-input" id="ntBf" placeholder="elonmusk"></div></div><div class="field-row"><label class="field-checkbox"><input type="checkbox" id="ntAf"> Auto-follow</label><label class="field-checkbox"><input type="checkbox" id="ntFm"> Follow members</label></div><div class="field-row"><label class="field-checkbox"><input type="checkbox" id="ntSp"> Skip profile</label><label class="field-checkbox"><input type="checkbox" id="ntNr"> No retry</label></div>',
    profile: '<div class="field"><label class="field-label">Screen Names (comma separated)</label><input class="field-input" id="ntPu" placeholder="user1, user2"></div><div class="field"><label class="field-label">List ID (optional)</label><input class="field-input" id="ntPl" placeholder="1234567890"></div>',
    json: '<div class="field"><label class="field-label">Paths (one per line)</label><textarea class="field-textarea" id="ntJp" placeholder="/path/to/file.json" rows="3"></textarea></div><div class="field-hint">JSON files or .loongtweet folders</div><label class="field-checkbox"><input type="checkbox" id="ntNr"> No retry</label>',
    mark: '<div class="field"><label class="field-label">Users (comma separated)</label><input class="field-input" id="ntMu" placeholder="user1, user2"></div><div class="field"><label class="field-label">Lists (comma separated ID)</label><input class="field-input" id="ntMl" placeholder="123, 456"></div><div class="field"><label class="field-label">Following (comma separated)</label><input class="field-input" id="ntMf" placeholder="elonmusk"></div><div class="field"><label class="field-label">Mark Timestamp (optional)</label><input class="field-input" id="ntMt" placeholder="2024-01-01T00:00:00Z"></div><div class="field-hint">Leave timestamp empty to mark as "now"</div>'
  };
  return m[t] || '';
}
async function submitNewTask() {
  const t = window._nt; let url, body;
  const v = id => $(id)?.value?.trim();
  const c = id => $(id)?.checked || false;
  try {
    switch (t) {
      case 'user': {
        const sn = v('ntSn'); if (!sn) { toast('Screen name required', 'error'); return; }
        url = '/api/v1/users/' + encodeURIComponent(sn) + '/download'; body = { auto_follow: c('ntAf'), skip_profile: c('ntSp'), no_retry: c('ntNr'), follow_members: false }; break;
      }
      case 'list': {
        const lid = v('ntLid'); if (!lid) { toast('List ID required', 'error'); return; }
        url = '/api/v1/lists/' + lid + '/download'; body = { auto_follow: c('ntAf'), follow_members: c('ntFm'), skip_profile: c('ntSp'), no_retry: c('ntNr') }; break;
      }
      case 'following': {
        const sn = v('ntSn'); if (!sn) { toast('Screen name required', 'error'); return; }
        url = '/api/v1/users/' + encodeURIComponent(sn) + '/following/download'; body = { auto_follow: c('ntAf'), skip_profile: c('ntSp'), no_retry: c('ntNr'), follow_members: false }; break;
      }
      case 'batch': {
        const users = (v('ntBu')||'').split(',').map(s=>s.trim()).filter(Boolean);
        const lists = (v('ntBl')||'').split(',').map(s=>s.trim()).filter(Boolean);
        const following = (v('ntBf')||'').split(',').map(s=>s.trim()).filter(Boolean);
        if (!users.length && !lists.length && !following.length) { toast('Enter at least one target', 'error'); return; }
        url = '/api/v1/batch/download'; body = { users, lists, following_names: following, auto_follow: c('ntAf'), follow_members: c('ntFm'), skip_profile: c('ntSp'), no_retry: c('ntNr') }; break;
      }
      case 'profile': {
        const users = (v('ntPu')||'').split(',').map(s=>s.trim()).filter(Boolean);
        const listId = v('ntPl');
        if (!users.length && !listId) { toast('Enter names or a list ID', 'error'); return; }
        if (listId) { url = '/api/v1/lists/' + listId + '/profile'; body = {}; }
        else { url = '/api/v1/users/' + encodeURIComponent(users[0]) + '/profile'; body = { screen_name: users[0] }; }
        break;
      }
      case 'json': {
        const paths = (v('ntJp')||'').split('\n').map(s=>s.trim()).filter(Boolean);
        if (!paths.length) { toast('Enter at least one path', 'error'); return; }
        const isFolder = paths.some(p => p.includes('.loongtweet'));
        url = isFolder ? '/api/v1/json/folder/download' : '/api/v1/json/file/download'; body = { paths, no_retry: c('ntNr') }; break;
      }
      case 'mark': {
        const users = (v('ntMu')||'').split(',').map(s=>s.trim()).filter(Boolean);
        const lists = (v('ntMl')||'').split(',').map(s=>s.trim()).filter(Boolean);
        const following = (v('ntMf')||'').split(',').map(s=>s.trim()).filter(Boolean);
        if (!users.length && !lists.length && !following.length) { toast('Enter at least one target', 'error'); return; }
        const ts = v('ntMt');
        url = '/api/v1/batch/mark'; body = { users, lists, following_names: following };
        if (ts) body.timestamp = ts;
        break;
      }
    }
    const data = await API.post(url, body); closeModal(); toast('Task created', 'success');
    if (state.currentPage === 'tasks') renderTasks();
  } catch (e) { toast(e.message, 'error'); }
}

/* ============================================================
   PAGE: Data
   ============================================================ */
async function renderData() {
  $('pageContent').innerHTML = `<div class="segmented" style="margin-bottom:16px">
    <button class="segmented-item active" data-dtab="users" onclick="switchDataTab('users')">Users</button>
    <button class="segmented-item" data-dtab="lists" onclick="switchDataTab('lists')">Lists</button>
    <button class="segmented-item" data-dtab="entities" onclick="switchDataTab('entities')">User Entities</button>
    <button class="segmented-item" data-dtab="list-entities" onclick="switchDataTab('list-entities')">List Entities</button>
    <button class="segmented-item" data-dtab="links" onclick="switchDataTab('links')">Links</button>
    <button class="segmented-item" data-dtab="stats" onclick="switchDataTab('stats')">Stats</button>
  </div><div id="dataContent"><div class="page-loader"><div class="spinner"></div></div></div>`;
  state.dataActiveTab = 'users'; switchDataTab('users');
}
async function switchDataTab(tab) {
  state.dataActiveTab = tab; document.querySelectorAll('[data-dtab]').forEach(el => el.classList.toggle('active', el.dataset.dtab === tab));
  const c = $('dataContent'); c.innerHTML = '<div class="page-loader"><div class="spinner"></div></div>';
  try {
    const fns = { users: renderDU, lists: renderDL, entities: renderDE, 'list-entities': renderDLE, links: renderDLink, stats: renderDStats };
    if (fns[tab]) await fns[tab](c);
  } catch (e) { c.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load</div><div class="empty-desc">${esc(e.message)}</div></div>`; }
}
async function renderDU(c) {
  const data = await API.get('/api/v1/db/users'); const items = Array.isArray(data) ? data : [];
  c.innerHTML = `<div class="panel"><div class="panel-head"><span class="panel-title">${plural(items.length, 'user')}</span></div>${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>Screen Name</th><th>Name</th><th>Protected</th><th>Access</th><th class="table-right"></th></tr></thead><tbody>${items.map(u => `<tr><td class="table-mono">${esc(u.id)}</td><td><strong>${esc(u.screen_name)}</strong></td><td class="table-muted">${esc(u.name||'-')}</td><td>${u.protected ? '<span class="badge badge-cancelled">Yes</span>' : '<span class="badge badge-completed">No</span>'}</td><td>${u.is_accessible ? '<span class="badge badge-completed">Yes</span>' : '<span class="badge badge-failed">No</span>'}</td><td class="table-right"><button class="btn btn-xs btn-ghost" onclick="showUserDetail('${esc(u.id)}')">View</button></td></tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No users</div></div>'}</div>`;
}
async function showUserDetail(id) {
  try {
    const u = await API.get('/api/v1/db/users/' + id);
    const ents = Array.isArray(await API.get('/api/v1/db/users/' + id + '/entities').catch(() => [])) || [];
    const links = Array.isArray(await API.get('/api/v1/db/users/' + id + '/links').catch(() => [])) || [];
    const prev = Array.isArray(await API.get('/api/v1/db/users/' + id + '/previous-names').catch(() => [])) || [];
    openModal('User: ' + esc(u.screen_name), `
      <div class="detail-grid">
        <span class="detail-label">ID</span><span class="detail-value text-mono">${esc(u.id)}</span>
        <span class="detail-label">Screen Name</span><span class="detail-value"><strong>${esc(u.screen_name)}</strong></span>
        <span class="detail-label">Name</span><span class="detail-value">${esc(u.name||'-')}</span>
        <span class="detail-label">Protected</span><span class="detail-value">${u.protected ? 'Yes' : 'No'}</span>
        <span class="detail-label">Accessible</span><span class="detail-value">${u.is_accessible ? 'Yes' : 'No'}</span>
      </div>
      ${ents.length ? `<div class="mt-3"><div class="text-sm" style="color:var(--text-tertiary);margin-bottom:6px">Entities (${ents.length})</div>${ents.map(e => '<div class="text-sm" style="padding:3px 0"><span class="text-mono">' + esc(e.name) + '</span> <span class="text-muted">' + esc(e.parent_dir||'') + '</span></div>').join('')}</div>` : ''}
      ${links.length ? `<div class="mt-3"><div class="text-sm" style="color:var(--text-tertiary);margin-bottom:6px">Linked Lists (${links.length})</div>${links.map(l => '<div class="text-sm" style="padding:3px 0">' + esc(l.name) + ' <span class="text-muted">' + esc(l.parent_lst_entity_name||'') + '</span></div>').join('')}</div>` : ''}
      ${prev.length ? `<div class="mt-3"><div class="text-sm" style="color:var(--text-tertiary);margin-bottom:6px">Previous Names</div>${prev.map(p => '<div class="text-sm" style="padding:2px 0"><span class="text-mono">' + esc(p.screen_name) + '</span> <span class="text-muted">' + esc(p.record_date) + '</span></div>').join('')}</div>` : ''}
    `, `<button class="btn btn-sm btn-danger" onclick="deleteUser('${esc(u.id)}')">Delete</button><button class="btn btn-sm btn-ghost" onclick="closeModal()">Close</button>`);
  } catch (e) { toast(e.message, 'error'); }
}
async function deleteUser(id) { if (!confirm('Delete user ' + id + '?')) return; try { await API.del('/api/v1/db/users/' + id); toast('Deleted', 'info'); closeModal(); switchDataTab('users'); } catch (e) { toast(e.message, 'error'); } }
async function renderDL(c) {
  const data = await API.get('/api/v1/db/lists'); const items = Array.isArray(data) ? data : [];
  c.innerHTML = `<div class="panel"><div class="panel-head"><span class="panel-title">${plural(items.length, 'list')}</span></div>${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>Name</th><th>Owner</th><th class="table-right"></th></tr></thead><tbody>${items.map(l => `<tr><td class="table-mono">${esc(l.id)}</td><td><strong>${esc(l.name)}</strong></td><td class="table-mono">${esc(l.owner_user_id||'-')}</td><td class="table-right"><button class="btn btn-xs btn-ghost" onclick="showListDetail('${esc(l.id)}')">View</button></td></tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No lists</div></div>'}</div>`;
}
async function showListDetail(id) {
  try { const l = await API.get('/api/v1/db/lists/' + id); const ents = Array.isArray(await API.get('/api/v1/db/lists/' + id + '/entities').catch(() => [])) || [];
    openModal('List: ' + esc(l.name), `<div class="detail-grid"><span class="detail-label">ID</span><span class="detail-value text-mono">${esc(l.id)}</span><span class="detail-label">Name</span><span class="detail-value"><strong>${esc(l.name)}</strong></span><span class="detail-label">Owner</span><span class="detail-value text-mono">${esc(l.owner_user_id||'-')}</span></div>${ents.length ? `<div class="mt-3"><div class="text-sm" style="color:var(--text-tertiary);margin-bottom:6px">Entities (${ents.length})</div>${ents.map(e => '<div class="text-sm" style="padding:3px 0"><span class="text-mono">' + esc(e.name) + '</span> <span class="text-muted">' + esc(e.parent_dir||'') + '</span></div>').join('')}</div>` : ''}`,
    `<button class="btn btn-sm btn-danger" onclick="deleteList('${esc(l.id)}')">Delete</button><button class="btn btn-sm btn-ghost" onclick="closeModal()">Close</button>`);
  } catch (e) { toast(e.message, 'error'); }
}
async function deleteList(id) { if (!confirm('Delete list ' + id + '?')) return; try { await API.del('/api/v1/db/lists/' + id); toast('Deleted', 'info'); closeModal(); switchDataTab('lists'); } catch (e) { toast(e.message, 'error'); } }
async function renderDE(c) {
  const data = await API.get('/api/v1/db/user-entities'); const items = Array.isArray(data) ? data : [];
  c.innerHTML = `<div class="panel"><div class="panel-head"><span class="panel-title">${plural(items.length, 'entity')}</span></div>${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>User ID</th><th>Name</th><th>Dir</th><th>Media</th><th>Latest</th></tr></thead><tbody>${items.map(e => `<tr><td class="table-mono">${esc(e.id)}</td><td class="table-mono">${esc(e.user_id)}</td><td>${esc(e.name)}</td><td class="table-muted">${esc(e.parent_dir||'-')}</td><td>${e.media_count != null ? e.media_count : '-'}</td><td class="table-muted">${e.latest_release_time||'-'}</td></tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No entities</div></div>'}</div>`;
}
async function renderDLE(c) {
  const data = await API.get('/api/v1/db/list-entities'); const items = Array.isArray(data) ? data : [];
  c.innerHTML = `<div class="panel"><div class="panel-head"><span class="panel-title">${plural(items.length, 'entity')}</span></div>${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>List ID</th><th>Name</th><th>Dir</th><th>List Name</th></tr></thead><tbody>${items.map(e => `<tr><td class="table-mono">${esc(e.id)}</td><td class="table-mono">${esc(e.lst_id)}</td><td>${esc(e.name)}</td><td class="table-muted">${esc(e.parent_dir||'-')}</td><td class="table-muted">${esc(e.list_name||'-')}</td></tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No entities</div></div>'}</div>`;
}
async function renderDLink(c) {
  const data = await API.get('/api/v1/db/user-links'); const items = Array.isArray(data) ? data : [];
  c.innerHTML = `<div class="panel"><div class="panel-head"><span class="panel-title">${plural(items.length, 'link')}</span></div>${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>User ID</th><th>Name</th><th>Parent Entity</th></tr></thead><tbody>${items.map(l => `<tr><td class="table-mono">${esc(l.id)}</td><td class="table-mono">${esc(l.user_id)}</td><td>${esc(l.name)}</td><td class="table-muted">${esc(l.parent_lst_entity_name||l.parent_lst_entity_id||'-')}</td></tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No links</div></div>'}</div>`;
}
async function renderDStats(c) {
  const s = await API.get('/api/v1/db/stats');
  c.innerHTML = `<div class="data-stats">${['users','lists','user_entities','list_entities','user_links','user_previous_names'].map(k => `<div class="data-stat"><div class="data-stat-value">${s[k]??'-'}</div><div class="data-stat-label">${k.replace(/_/g,' ')}</div></div>`).join('')}</div>`;
}

/* ============================================================
   PAGE: Schedules
   ============================================================ */
async function renderSchedules() {
  try {
    const sd = await API.get('/api/v1/schedules').catch(() => ({ entries: [] }));
    const entries = Array.isArray(sd) ? sd : sd?.entries || []; state.schedules = entries;
    $('pageContent').innerHTML = `
      <div class="section-header" style="margin-bottom:12px">
        <span class="section-title">${plural(entries.length, 'schedule')}</span>
        <div class="section-actions">
          <button class="btn btn-sm btn-primary" onclick="showNewScheduleModal()"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> New</button>
          <button class="btn btn-sm btn-secondary" onclick="showScheduleRawEditor()">Edit Raw</button>
          <button class="btn btn-sm btn-ghost" onclick="triggerAllSchedules()">Trigger All</button>
        </div>
      </div>
      <div class="panel">${entries.length ? entries.map(s => {
        const hf = s.consecutive_failures > 0;
        return `<div class="schedule ${hf?'schedule-failure':''}">
          <div class="schedule-type"><span class="badge badge-mono ${s.enabled?'badge-completed':'badge-queued'}">${SCHEDULE_LABELS[s.type]||s.type}</span></div>
          <div class="schedule-body">
            <div class="schedule-title">${esc(s.name||s.target||'Unnamed')}</div>
            <div class="schedule-meta">
              ${s.target ? '<span>' + esc(s.target) + '</span>' : ''}
              ${s.schedule ? '<span>' + esc(s.schedule) + '</span>' : ''}
              ${s.next_run ? '<span>Next ' + relTime(s.next_run) + '</span>' : ''}
              ${s.last_run ? '<span>Last ' + relTime(s.last_run) + '</span>' : ''}
              ${hf ? '<span class="badge badge-failed">' + s.consecutive_failures + ' failures</span>' : ''}
            </div>
          </div>
          <div style="flex-shrink:0"><label class="toggle"><input type="checkbox" ${s.enabled?'checked':''} onchange="toggleSched('${esc(s.id||'')}',this.checked)"><span class="toggle-track"></span></label></div>
          <div class="schedule-actions">
            <button class="btn btn-xs btn-ghost" onclick="triggerSched('${esc(s.id||'')}')" title="Trigger"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polygon points="5 3 19 12 5 21 5 3"/></svg></button>
            <button class="btn btn-xs btn-ghost" onclick="deleteSched('${esc(s.id||'')}')" title="Delete"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>
          </div>
        </div>`;
      }).join('') : '<div class="empty-state"><div class="empty-icon"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></div><div class="empty-title">No schedules</div><div class="empty-desc">Create a scheduled download</div></div>'}</div>`;
  } catch (e) { $('pageContent').innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load</div><div class="empty-desc">${esc(e.message)}</div></div>`; }
}
async function toggleSched(id, en) { if (!id) return; try { await API.patch('/api/v1/schedules/' + id + '/enabled', { enabled: en }); toast(en ? 'Enabled' : 'Disabled', 'info'); renderSchedules(); } catch (e) { toast(e.message, 'error'); } }
async function triggerSched(id) { if (!id) return; try { await API.post('/api/v1/schedules/' + id + '/trigger'); toast('Triggered', 'success'); } catch (e) { toast(e.message, 'error'); } }
async function triggerAllSchedules() { try { const r = await API.post('/api/v1/schedules/trigger-all'); toast('Triggered ' + (r.succeeded||0) + ' schedules', 'info'); renderSchedules(); } catch (e) { toast(e.message, 'error'); } }
async function deleteSched(id) { if (!id || !confirm('Delete?')) return; try { await API.del('/api/v1/schedules/' + id); toast('Deleted', 'info'); renderSchedules(); } catch (e) { toast(e.message, 'error'); } }

function showNewScheduleModal() {
  openModal('New Schedule', `
    <div class="field"><label class="field-label">Type</label><select class="field-select" id="nsType"><option value="user">User</option><option value="list">List</option><option value="following">Following</option><option value="mixed">Mixed</option></select></div>
    <div class="field"><label class="field-label">Target</label><input class="field-input" id="nsTarget" placeholder="screen_name or list_id"><div class="field-hint">Screen name for user/following, list ID for list</div></div>
    <div class="field"><label class="field-label">Name</label><input class="field-input" id="nsName" placeholder="My Schedule"></div>
    <div class="field"><label class="field-label">Schedule</label><input class="field-input" id="nsSchedule" placeholder='daily 08:00,20:00 or interval 4h'><div class="field-hint">Format: "daily HH:MM" or "interval DURATION"</div></div>
    <div class="field-row"><label class="field-checkbox"><input type="checkbox" id="nsEnabled" checked> Enabled</label><label class="field-checkbox"><input type="checkbox" id="nsRunOnStart"> Run on start</label></div>
  `, '<button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-sm btn-primary" onclick="submitNewSchedule()">Create</button>');
}
async function submitNewSchedule() {
  const t = $('nsType')?.value, target = $('nsTarget')?.value?.trim(), name = $('nsName')?.value?.trim(), sched = $('nsSchedule')?.value?.trim();
  if (!target || !sched) { toast('Target and schedule required', 'error'); return; }
  try {
    await API.post('/api/v1/schedules', { entries: [{ type: t, target, name: name||target, schedule: sched, enabled: $('nsEnabled')?.checked||false, run_on_start: $('nsRunOnStart')?.checked||false }] });
    closeModal(); toast('Schedule created', 'success'); renderSchedules();
  } catch (e) { toast(e.message, 'error'); }
}
async function showScheduleRawEditor() {
  try { const raw = await API.get('/api/v1/schedules/raw');
    openModal('Schedule Config (Raw)', '<div class="field-hint mb-3">Edit schedules.yaml directly.</div><div class="code-block" id="schedulesEditor" contenteditable="true">' + esc(raw.content||'') + '</div>',
      '<button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-sm btn-primary" onclick="saveScheduleRaw()">Save & Reload</button>');
  } catch (e) { toast(e.message, 'error'); }
}
async function saveScheduleRaw() {
  const content = $('schedulesEditor')?.textContent||'';
  try {
    const validation = await API.post('/api/v1/schedules/validate', { raw: content });
    if (validation?.valid === false) {
      toast('Validation failed: ' + (validation.errors||['Invalid syntax']).join(', '), 'error');
      return;
    }
    await API.put('/api/v1/schedules/raw', { content }); closeModal(); toast('Schedules reloaded', 'success'); renderSchedules();
  } catch (e) { toast(e.message, 'error'); }
}

/* ============================================================
   PAGE: Config / Settings
   ============================================================ */
async function renderConfig() {
  $('pageContent').innerHTML = `<div class="segmented" style="margin-bottom:16px">
    <button class="segmented-item active" data-ctab="settings" onclick="switchConfigTab('settings')">Settings</button>
    <button class="segmented-item" data-ctab="cookies" onclick="switchConfigTab('cookies')">Cookies</button>
    <button class="segmented-item" data-ctab="raw" onclick="switchConfigTab('raw')">Raw Config</button>
  </div><div id="configContent"><div class="page-loader"><div class="spinner"></div></div></div>`;
  switchConfigTab('settings');
}
async function switchConfigTab(tab) {
  document.querySelectorAll('[data-ctab]').forEach(el => el.classList.toggle('active', el.dataset.ctab === tab));
  const c = $('configContent'); c.innerHTML = '<div class="page-loader"><div class="spinner"></div></div>';
  try {
    if (tab === 'settings') await renderCSettings(c);
    else if (tab === 'cookies') await renderCCookies(c);
    else await renderCRaw(c);
  } catch (e) { c.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load</div><div class="empty-desc">${esc(e.message)}</div></div>`; }
}
async function renderCSettings(c) {
  const fields = await API.get('/api/v1/config/fields').catch(() => ({ fields: [] }));
  const config = await API.get('/api/v1/config').catch(() => null);
  const fl = Array.isArray(fields?.fields) ? fields.fields : [];
  c.innerHTML = `<div class="panel"><div class="panel-head"><span class="panel-title">Configuration</span><div class="section-actions"><button class="btn btn-sm btn-primary" onclick="saveConfig()">Save</button></div></div><div class="panel-body padded">${fl.length ? fl.map(f => {
    const id = 'cfg_' + f.name;
    const val = f.value || f.default || '';
    if (f.type === 'number') return '<div class="field"><label class="field-label">' + esc(f.label||f.name) + '</label><input class="field-input" id="' + id + '" type="number" value="' + esc(val) + '" placeholder="' + esc(f.placeholder||'') + '"><div class="field-hint">' + esc(f.prompt||'') + '</div></div>';
    if (f.type === 'boolean') return '<div class="field"><label class="field-checkbox"><input type="checkbox" id="' + id + '" ' + (val==='true'?'checked':'') + '> ' + esc(f.label||f.name) + '</label></div>';
    return '<div class="field"><label class="field-label">' + esc(f.label||f.name) + '</label><input class="field-input" id="' + id + '" type="' + (f.name==='auth_token'||f.name==='ct0'?'password':'text') + '" value="' + esc(val) + '" placeholder="' + esc(f.placeholder||'') + '"><div class="field-hint">' + esc(f.prompt||'') + '</div></div>';
  }).join('') : '<div class="empty-desc">No fields available</div>'}</div></div>${config ? '<div class="panel mt-3"><div class="panel-head"><span class="panel-title">Current Settings</span></div><div class="panel-body padded"><div class="detail-grid"><span class="detail-label">Root Path</span><span class="detail-value text-mono">' + esc(config.root_path||'-') + '</span><span class="detail-label">Max Routine</span><span class="detail-value">' + (config.max_download_routine??'-') + '</span><span class="detail-label">Max Filename</span><span class="detail-value">' + (config.max_file_name_len??'-') + '</span></div></div></div><div class="panel mt-3"><div class="panel-head"><span class="panel-title" style="color:var(--danger)">Danger Zone</span></div><div class="panel-body padded"><button class="btn btn-sm btn-danger" onclick="shutdownServer()">Shutdown Server</button><div class="field-hint mt-2">Gracefully stops the TMD server</div></div></div>' : ''}`;
}
async function saveConfig() {
  const fields = {}; document.querySelectorAll('[id^="cfg_"]').forEach(el => { fields[el.id.replace('cfg_','')] = el.type === 'checkbox' ? (el.checked ? 'true' : 'false') : el.value; });
  try { await API.put('/api/v1/config/fields', { fields }); toast('Saved. Restart may be required.', 'success'); renderCSettings($('configContent')); } catch (e) { toast(e.message, 'error'); }
}
async function renderCCookies(c) {
  const [ck, raw] = await Promise.all([API.get('/api/v1/cookies').catch(()=>null), API.get('/api/v1/cookies/raw').catch(()=>null)]);
  const list = Array.isArray(ck?.cookies) ? ck.cookies : [];
  c.innerHTML = `<div class="panel"><div class="panel-head"><span class="panel-title">Cookies</span><div class="section-actions"><button class="btn btn-sm btn-secondary" onclick="showCookiesRaw()">Edit Raw</button><button class="btn btn-sm btn-primary" onclick="saveCookies()">Save</button></div></div><div class="panel-body padded"><div class="field"><label class="field-label">Main Auth Token</label><input class="field-input" id="ckMainAuth" type="password" placeholder="Leave empty to keep current"></div><div class="field"><label class="field-label">Main ct0</label><input class="field-input" id="ckMainCt0" type="password" placeholder="Leave empty to keep current"></div>${list.length ? '<div class="text-sm text-muted" style="border-top:1px solid var(--separator);padding-top:16px;margin-top:16px">Additional Accounts (' + list.length + ')</div>' + list.map((c,i) => `<div class="field-row mt-2"><div class="field"><label class="field-label">Account ${i+1} Auth</label><input class="field-input" id="ckExAuth_${i}" type="password" value="${esc(c.auth_token||'')}"></div><div class="field"><label class="field-label">Account ${i+1} ct0</label><input class="field-input" id="ckExCt0_${i}" type="password" value="${esc(c.ct0||'')}"></div></div>`).join('') : ''}</div></div>`;
}
async function saveCookies() {
  const cookies = [];
  const ma = $('ckMainAuth')?.value?.trim(), mc = $('ckMainCt0')?.value?.trim();
  if (ma && mc) cookies.push({ auth_token: ma, ct0: mc });
  let i = 0; while ($('ckExAuth_' + i)) { const a = $('ckExAuth_' + i)?.value?.trim(), c = $('ckExCt0_' + i)?.value?.trim(); if (a||c) { if (a&&c) cookies.push({ auth_token: a, ct0: c, index: i }); else toast('Account '+(i+1)+': both fields required', 'error'); } i++; }
  try { await API.put('/api/v1/cookies', { cookies }); toast('Cookies saved', 'success'); renderCCookies($('configContent')); } catch (e) { toast(e.message, 'error'); }
}
async function showCookiesRaw() {
  try { const raw = await API.get('/api/v1/cookies/raw'); openModal('Cookies (Raw)', '<div class="code-block" id="cookiesEditor" contenteditable="true">' + esc(raw.content||'') + '</div>', '<button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-sm btn-primary" onclick="saveCookiesRaw()">Save</button>'); } catch (e) { toast(e.message, 'error'); }
}
async function saveCookiesRaw() { try { await API.put('/api/v1/cookies/raw', { content: $('cookiesEditor')?.textContent||'' }); closeModal(); toast('Cookies updated', 'success'); renderCCookies($('configContent')); } catch (e) { toast(e.message, 'error'); } }
async function renderCRaw(c) {
  const [conf, cookies, sched] = await Promise.all([API.get('/api/v1/config/raw').catch(()=>null), API.get('/api/v1/cookies/raw').catch(()=>null), API.get('/api/v1/schedules/raw').catch(()=>null)]);
  c.innerHTML = `
    <div class="panel"><div class="panel-head"><span class="panel-title">conf.yaml</span><div class="section-actions"><button class="btn btn-sm btn-primary" onclick="saveRawFile('config')">Save</button></div></div><div class="code-block" id="rawConfig" contenteditable="true">${esc(conf?.content||'# No file')}</div></div>
    <div class="panel mt-3"><div class="panel-head"><span class="panel-title">additional_cookies.yaml</span><div class="section-actions"><button class="btn btn-sm btn-primary" onclick="saveRawFile('cookies')">Save</button></div></div><div class="code-block" id="rawCookies" contenteditable="true">${esc(cookies?.content||'# No file')}</div></div>
    <div class="panel mt-3"><div class="panel-head"><span class="panel-title">schedules.yaml</span><div class="section-actions"><button class="btn btn-sm btn-primary" onclick="saveRawFile('schedules')">Save</button></div></div><div class="code-block" id="rawSched" contenteditable="true">${esc(sched?.content||'# No file')}</div></div>`;
}
async function saveRawFile(which) {
  const editors = { config: 'rawConfig', cookies: 'rawCookies', schedules: 'rawSched' };
  const apis = { config: '/api/v1/config/raw', cookies: '/api/v1/cookies/raw', schedules: '/api/v1/schedules/raw' };
  const msgs = { config: 'Config saved. Restart to apply.', cookies: 'Cookies saved.', schedules: 'Schedules reloaded.' };
  try { await API.put(apis[which], { content: $(editors[which])?.textContent||'' }); toast(msgs[which], 'success'); } catch (e) { toast(e.message, 'error'); }
}
async function shutdownServer() {
  if (!confirm('Shutdown TMD server? This will stop all running tasks.')) return;
  try { await API.post('/api/v1/server/shutdown'); toast('Server shutting down...', 'warning'); } catch (e) { toast(e.message, 'error'); }
}

/* ============================================================
   PAGE: Logs
   ============================================================ */
let logStream = false, logReader = null, logScroll = true;
async function renderLogs() {
  try {
    const ld = await API.get('/api/v1/logs').catch(() => ({ logs: [] }));
    const logs = Array.isArray(ld?.logs) ? ld.logs : [];
    $('pageContent').innerHTML = `
      <div class="panel">
        <div class="log-bar">
          <div class="log-bar-status">
            <span class="sse-dot ${logStream?'connected':'disconnected'}" id="logDot" style="width:6px;height:6px;display:inline-block"></span>
            <span id="logStatusText">${logStream?'Streaming':'Paused'}</span>
            <span class="text-muted">&middot; ${ld.total||logs.length} entries</span>
          </div>
          <div class="log-bar-actions">
            <label class="field-checkbox" style="font-size:10px"><input type="checkbox" checked onchange="logScroll=this.checked"> Auto-scroll</label>
            <button class="btn btn-xs btn-ghost" onclick="clearLogs()">Clear</button>
            <button class="btn btn-xs btn-ghost" onclick="exportLogs()">Export</button>
            <button class="btn btn-xs ${logStream?'btn-danger':'btn-primary'}" id="logToggle" onclick="toggleLogs()">${logStream?'Stop':'Stream'}</button>
          </div>
        </div>
        <div class="log-viewer" id="logContainer">${logs.map(l => '<div class="log-line' + (logLevel(l)?' '+logLevel(l):'') + '">' + esc(stripAnsi(l)) + '</div>').join('\n')}</div>
      </div>`;
    if (logStream) startLogStream();
  } catch (e) { $('pageContent').innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load logs</div><div class="empty-desc">${esc(e.message)}</div></div>`; }
}
function clearLogs() { const c = $('logContainer'); if (c) c.innerHTML = ''; toast('Cleared', 'info'); }
function toggleLogs() { logStream ? stopLogs() : startLogStream(); }
function stopLogs() { if (logReader) { logReader.cancel(); logReader = null; } logStream = false; const d = $('logDot'), t = $('logStatusText'), b = $('logToggle'); if(d) d.className='sse-dot disconnected'; if(t) t.textContent='Paused'; if(b) {b.textContent='Stream';b.className='btn btn-xs btn-primary';} }
async function startLogStream() {
  if (logReader) return; logStream = true;
  const d = $('logDot'), t = $('logStatusText'), b = $('logToggle');
  if(d) d.className='sse-dot connecting'; if(t) t.textContent='Connecting...'; if(b) {b.textContent='Stop';b.className='btn btn-xs btn-danger';}
  try {
    const r = await fetch('/api/v1/logs/stream'); if (!r.ok) throw new Error('Connection failed');
    if(d) d.className='sse-dot connected'; if(t) t.textContent='Streaming';
    const reader = r.body.getReader(); logReader = reader; const dec = new TextDecoder(); let buf = '';
    while (true) { const {done,value} = await reader.read(); if (done) break; buf += dec.decode(value, {stream:true}); const lines = buf.split('\n'); buf = lines.pop()||''; for (const ln of lines) { if (ln.trim()) { const c = $('logContainer'); if (c) { const el = document.createElement('div'); el.className = 'log-line'+(logLevel(ln)?' '+logLevel(ln):''); el.textContent = stripAnsi(ln); c.appendChild(el); if (logScroll) c.scrollTop = c.scrollHeight; } } } }
  } catch (e) { if (logStream) { if(d) d.className='sse-dot disconnected'; if(t) t.textContent='Disconnected'; toast('Log stream: ' + e.message, 'warning'); } }
  finally { logStream = false; logReader = null; if(b) {b.textContent='Stream';b.className='btn btn-xs btn-primary';} }
}
async function exportLogs() { try { window.open('/api/v1/logs/export', '_blank'); toast('Exporting...', 'info'); } catch (e) { toast(e.message, 'error'); } }

/* ============================================================
   SSE
   ============================================================ */
let sseSource = null, sseTimer = null;
function connectSSE() {
  if (sseSource) { sseSource.close(); sseSource = null; }
  sseSource = new EventSource('/api/v1/sse/tasks');
  sseSource.onopen = () => { state.sseConnected = true; updateSSE(true); if (sseTimer) { clearTimeout(sseTimer); sseTimer = null; } };
  sseSource.addEventListener('tasks', e => { try {
    const d = JSON.parse(e.data);
    if (d.tasks) { state.tasks = d.tasks; updateBadge(); if (state.currentPage === 'tasks') renderTasks(); }
    if (state.currentPage === 'dashboard') { API.get('/api/v1/tasks/stats').then(s => { if (s) state.taskStats = s; updateBadge(); }).catch(() => {}); }
  } catch(_) {} });
  sseSource.addEventListener('schedules', e => { try { const d = JSON.parse(e.data); if (d.entries) { state.schedules = d.entries; state.schedulerRunning = d.scheduler_running; if (state.currentPage === 'schedules') renderSchedules(); } } catch(_) {} });
  sseSource.addEventListener('notification', e => { try { const d = JSON.parse(e.data); if (d.message) toast(d.message, d.level||'info'); } catch(_) {} });
  sseSource.addEventListener('server_shutdown', () => toast('Server shutting down...', 'error'));
  sseSource.onerror = () => { state.sseConnected = false; updateSSE(false); if (sseSource) sseSource.close(); sseSource = null; if (!sseTimer) sseTimer = setTimeout(() => { sseTimer = null; connectSSE(); }, 5000); };
}
function updateSSE(connected) {
  const dot = $('sseDot'), dash = $('dashSseStatus');
  if (dot) dot.className = 'sse-dot ' + (connected ? 'connected' : 'disconnected');
  if (dash) dash.innerHTML = '<span class="badge ' + (connected?'badge-completed':'badge-failed') + '">' + (connected?'Connected':'Disconnected') + '</span>';
}
function updateHealth() {
  const dot = $('healthDot'), text = $('healthText'), ver = $('sidebarVersion');
  if (!dot || !text) return;
  dot.className = 'health-indicator ' + (state.health?.status === 'ok' ? 'ok' : 'down');
  text.textContent = state.health?.status === 'ok' ? 'Online' : 'Offline';
  if (ver) ver.textContent = state.health?.version || '--';
}
function updateBadge() {
  const b = $('taskBadge'); if (!b) return;
  const n = (state.taskStats?.running||0) + (state.taskStats?.queued||0);
  if (n > 0) { b.textContent = n; b.style.display = ''; } else { b.style.display = 'none'; }
}

/* ---- Health check ---- */
async function checkHealth() { try { state.health = await API.get('/api/v1/health'); } catch (e) { state.health = null; } updateHealth(); }

/* ---- Init ---- */
async function init() {
  await checkHealth();
  connectSSE();
  const p = (location.pathname.replace(/^\//, '') || 'dashboard');
  state.currentPage = PAGES.includes(p) ? p : 'dashboard';
  updateNav();
  renderPage(state.currentPage);
  setInterval(checkHealth, 30000);
}
document.addEventListener('DOMContentLoaded', init);
