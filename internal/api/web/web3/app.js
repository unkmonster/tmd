/* ============================================================
   TMD Web v3 - app.js
   Twitter Media Downloader management UI
   SPA with 6 pages, SSE real-time updates
   ============================================================ */

/* ---- Constants ---- */
const API_BASE = '';
const PAGES = ['dashboard', 'tasks', 'data', 'schedules', 'config', 'logs'];
const PAGE_TITLES = {
  dashboard: 'Dashboard',
  tasks: 'Tasks',
  data: 'Data',
  schedules: 'Schedules',
  config: 'Configuration',
  logs: 'Logs'
};
const TASK_TYPE_LABELS = {
  user_download: 'User Download',
  list_download: 'List Download',
  following_download: 'Following Download',
  profile_download: 'Profile Download',
  mark_downloaded: 'Mark Downloaded',
  json_file_download: 'JSON File Import',
  json_folder_download: 'Folder Import',
  batch_download: 'Batch Download',
  list_profile: 'List Profile',
  retry_all_failed: 'Retry All Failed'
};
const SCHEDULE_LABELS = { list: 'List', user: 'User', following: 'Following', mixed: 'Mixed' };
const STATUS_TAG_CLASS = {
  completed: 'tag-success',
  running: 'tag-running',
  queued: 'tag-queued',
  failed: 'tag-failed',
  cancelled: 'tag-cancelled'
};

/* ---- State ---- */
const state = {
  currentPage: 'dashboard',
  tasks: [],
  taskStats: { queued: 0, running: 0, completed: 0, failed: 0, cancelled: 0, total: 0 },
  health: null,
  sseConnected: false,
  dataActiveTab: 'users',
  dataPage: {},
  dataSearch: {},
  schedules: [],
  schedulerRunning: false,
};

/* ---- API Client ---- */
const API = {
  _abortControllers: new Map(),
  async _fetch(url, opts) {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 30000);
    try {
      const r = await fetch(url, { ...opts, signal: ctrl.signal });
      if (!r.ok && r.headers.get('content-type')?.includes('json')) {
        const j = await r.json();
        throw new Error(j.error || j.message || `HTTP ${r.status}`);
      }
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return r;
    } catch (e) {
      if (e.name === 'AbortError') throw new Error('Request timed out');
      throw e;
    } finally { clearTimeout(timer); }
  },
  async _json(url, opts) {
    const r = await this._fetch(url, opts);
    const j = await r.json();
    if (!j.success) throw new Error(j.error || 'Request failed');
    return j.data;
  },
  get: (url) => API._json(API_BASE + url),
  post: (url, body) => API._json(API_BASE + url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined
  }),
  put: (url, body) => API._json(API_BASE + url, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined
  }),
  patch: (url, body) => API._json(API_BASE + url, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined
  }),
  del: (url) => API._json(API_BASE + url, { method: 'DELETE' }),
  text: async (url) => {
    const r = await API._fetch(API_BASE + url);
    return r.text();
  },
  rawResponse: async (url, opts) => {
    const r = await API._fetch(API_BASE + url, opts);
    const j = await r.json();
    return j;
  }
};

/* ---- Utility Functions ---- */
const $ = (id) => document.getElementById(id);
const esc = (s) => { if (s == null) return ''; const d = document.createElement('div'); d.appendChild(document.createTextNode(String(s))); return d.innerHTML; };
const plural = (n, s) => n + ' ' + (n === 1 ? s : s + 's');

function relativeTime(iso) {
  if (!iso) return '-';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '-';
  const diff = Date.now() - d.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return plural(mins, 'min') + ' ago';
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return plural(hrs, 'hr') + ' ago';
  const days = Math.floor(hrs / 24);
  if (days < 30) return plural(days, 'day') + ' ago';
  return d.toLocaleDateString();
}

function formatTime(iso) {
  if (!iso) return '-';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '-';
  return d.toLocaleString();
}

function formatDuration(startIso, endIso) {
  if (!startIso) return '-';
  const start = new Date(startIso);
  const end = endIso ? new Date(endIso) : new Date();
  const secs = Math.max(0, Math.floor((end - start) / 1000));
  if (secs < 60) return secs + 's';
  const mins = Math.floor(secs / 60);
  const rem = secs % 60;
  return mins + 'm ' + rem + 's';
}

function stripAnsi(str) {
  return str.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '');
}

function logLevel(line) {
  if (/ERRO\[|level=e(rror)?/.test(line)) return 'error';
  if (/WARN\[|level=w(arn)?/.test(line)) return 'warn';
  if (/INFO\[|level=i(nfo)?/.test(line)) return 'info';
  if (/DEBU\[|level=d(ebug)?/.test(line)) return 'debug';
  return '';
}

function taskTypeLabel(t) { return TASK_TYPE_LABELS[t] || t; }

function durationSeconds(start, end) {
  if (!start) return 0;
  return Math.max(0, Math.floor(((end ? new Date(end) : new Date()) - new Date(start)) / 1000));
}

/* ---- Toast ---- */
function toast(message, type) {
  const container = $('toastContainer');
  const el = document.createElement('div');
  el.className = 'toast ' + (type || 'info');
  el.innerHTML = '<div class="toast-message">' + esc(message) + '</div><button class="toast-close" onclick="this.parentElement.classList.add(\'out\');setTimeout(()=>this.parentElement.remove(),200)" type="button"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>';
  container.appendChild(el);
  setTimeout(() => { el.classList.add('out'); setTimeout(() => el.remove(), 200); }, 4000);
}

/* ---- Modal ---- */
function openModal(title, bodyHTML, footHTML) {
  $('modalTitle').textContent = title;
  $('modalBody').innerHTML = bodyHTML;
  $('modalFoot').innerHTML = footHTML || '';
  $('modalOverlay').classList.add('open');
  $('modal').classList.add('open');
  $('modal').setAttribute('aria-hidden', 'false');
  $('modalOverlay').setAttribute('aria-hidden', 'false');
}

function closeModal() {
  $('modalOverlay').classList.remove('open');
  $('modal').classList.remove('open');
  $('modal').setAttribute('aria-hidden', 'true');
  $('modalOverlay').setAttribute('aria-hidden', 'true');
}

$('modalClose').onclick = closeModal;
$('modalOverlay').onclick = closeModal;
document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeModal(); });

/* ---- Navigation / Router ---- */
function navigate(page) {
  if (!PAGES.includes(page)) page = 'dashboard';
  state.currentPage = page;
  history.pushState(null, '', page === 'dashboard' ? '/' : '/' + page);
  updateNavigation();
  renderPage(page);
}

function updateNavigation() {
  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === state.currentPage);
    if (el.dataset.page === state.currentPage) {
      el.setAttribute('aria-current', 'page');
    } else {
      el.removeAttribute('aria-current');
    }
  });
  $('pageTitle').textContent = PAGE_TITLES[state.currentPage] || 'Dashboard';
  document.title = 'TMD - ' + (PAGE_TITLES[state.currentPage] || 'Dashboard');
}

function renderPage(page) {
  const content = $('content');
  content.innerHTML = '<div class="page-loading"><div class="spinner"></div></div>';
  switch (page) {
    case 'dashboard': renderDashboard(); break;
    case 'tasks': renderTasks(); break;
    case 'data': renderData(); break;
    case 'schedules': renderSchedules(); break;
    case 'config': renderConfig(); break;
    case 'logs': renderLogs(); break;
  }
}

/* ---- Navigation Events ---- */
document.querySelectorAll('.nav-item').forEach(el => {
  el.addEventListener('click', () => navigate(el.dataset.page));
  el.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); navigate(el.dataset.page); } });
});

$('menuBtn').addEventListener('click', () => {
  $('sidebar').classList.toggle('open');
});
$('sidebarOverlay').addEventListener('click', () => {
  $('sidebar').classList.remove('open');
});

/* ---- Theme Toggle ---- */
$('themeBtn').addEventListener('click', () => {
  const html = document.documentElement;
  const current = html.getAttribute('data-theme') || 'dark';
  const next = current === 'dark' ? 'light' : 'dark';
  html.setAttribute('data-theme', next);
  localStorage.setItem('tmd-theme', next);
});
const savedTheme = localStorage.getItem('tmd-theme');
if (savedTheme) document.documentElement.setAttribute('data-theme', savedTheme);

/* ---- Page: Dashboard ---- */
async function renderDashboard() {
  const content = $('content');
  try {
    const [health, stats] = await Promise.all([
      API.get('/api/v1/health').catch(() => null),
      API.get('/api/v1/tasks/stats').catch(() => null),
    ]);
    state.health = health;
    if (stats) state.taskStats = stats;
    updateSidebarHealth();

    const queue = await API.get('/api/v1/queue/status').catch(() => null);
    const tasksList = await API.get('/api/v1/tasks').catch(() => null);
    const recent = Array.isArray(tasksList?.tasks) ? tasksList.tasks.slice(0, 8) : [];

    content.innerHTML = `
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="9"/><rect x="14" y="3" width="7" height="5"/><rect x="14" y="12" width="7" height="9"/><rect x="3" y="16" width="7" height="5"/></svg></div>
          <div class="stat-content"><div class="stat-value">${stats ? stats.total : '-'}</div><div class="stat-label">Total Tasks</div></div>
        </div>
        <div class="stat-card">
          <div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div>
          <div class="stat-content"><div class="stat-value">${stats ? stats.completed : '-'}</div><div class="stat-label">Completed</div></div>
        </div>
        <div class="stat-card">
          <div class="stat-icon ${stats?.running > 0 ? 'accent' : 'success'}">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
          </div>
          <div class="stat-content"><div class="stat-value">${stats ? stats.running : '-'}</div><div class="stat-label">Running${stats?.queued > 0 ? ' (+' + stats.queued + ' queued)' : ''}</div></div>
        </div>
        <div class="stat-card">
          <div class="stat-icon ${stats?.failed > 0 ? 'danger' : 'success'}">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
          </div>
          <div class="stat-content"><div class="stat-value">${stats ? stats.failed : '-'}</div><div class="stat-label">Failed</div></div>
        </div>
      </div>

      <div class="dashboard-grid">
        <div class="card">
          <div class="card-header"><span class="card-title">System Status</span></div>
          <div class="card-body">
            <div style="display:grid;grid-template-columns:auto 1fr;gap:8px 16px;font-size:13px">
              <span style="color:var(--text-muted)">Status</span>
              <span>${health ? '<span class="tag tag-success">Online</span>' : '<span class="tag tag-failed">Offline</span>'}</span>
              <span style="color:var(--text-muted)">Version</span>
              <span class="text-mono">${health?.version || '-'}</span>
              <span style="color:var(--text-muted)">Database</span>
              <span>${health ? '<span class="tag tag-success">Connected</span>' : '<span class="tag tag-failed">Disconnected</span>'}</span>
              <span style="color:var(--text-muted)">SSE</span>
              <span id="dashSseStatus"><span class="tag ${state.sseConnected ? 'tag-success' : 'tag-failed'}">${state.sseConnected ? 'Connected' : 'Disconnected'}</span></span>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><span class="card-title">Queue Status</span></div>
          <div class="card-body">
            ${queue ? `
            <div class="queue-status">
              <div class="queue-stat"><div class="queue-stat-value">${queue.queue_depth}</div><div class="queue-stat-label">Depth</div></div>
              <div class="queue-stat"><div class="queue-stat-value">${queue.active_jobs}</div><div class="queue-stat-label">Active</div></div>
              <div class="queue-stat"><div class="queue-stat-value">${queue.pending_jobs}</div><div class="queue-stat-label">Pending</div></div>
              <div class="queue-stat"><div class="queue-stat-value">${queue.detached_jobs}</div><div class="queue-stat-label">Detached</div></div>
            </div>` : '<div class="empty-state"><div class="empty-desc">Unable to fetch queue status</div></div>'}
          </div>
        </div>

        <div class="card" style="grid-column:1/-1">
          <div class="card-header">
            <span class="card-title">Recent Tasks</span>
            <div class="card-actions">
              <button class="btn btn-sm btn-ghost" onclick="navigate('tasks')">View All</button>
            </div>
          </div>
          ${recent.length ? renderTaskList(recent) : `<div class="empty-state"><div class="empty-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div><div class="empty-title">No tasks yet</div><div class="empty-desc">Start a download from the Tasks page</div></div>`}
        </div>
      </div>
    `;
  } catch (e) {
    content.innerHTML = `<div class="empty-state"><div class="empty-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg></div><div class="empty-title">Failed to load dashboard</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

/* ---- Page: Tasks ---- */
async function renderTasks() {
  const content = $('content');
  try {
    const [tasksData, stats] = await Promise.all([
      API.get('/api/v1/tasks'),
      API.get('/api/v1/tasks/stats').catch(() => null),
    ]);
    const tasks = Array.isArray(tasksData?.tasks) ? tasksData.tasks : [];
    if (stats) state.taskStats = stats;
    state.tasks = tasks;
    updateTaskBadge();

    content.innerHTML = `
      <div class="toolbar">
        <div class="toolbar-left">
          <button class="btn btn-sm btn-primary" onclick="showNewTaskModal()">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            New Task
          </button>
          <button class="btn btn-sm btn-secondary" onclick="cancelAllQueued()">Cancel Queued</button>
          <button class="btn btn-sm btn-ghost" onclick="retryAllFailed()">Retry Failed</button>
        </div>
        <div class="toolbar-right">
          <span class="text-sm text-muted">${plural(tasks.length, 'task')}${stats ? ' &middot; ' + stats.running + ' running' : ''}</span>
        </div>
      </div>
      <div class="card">
        ${tasks.length ? renderTaskList(tasks) : `<div class="empty-state"><div class="empty-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div><div class="empty-title">No tasks</div><div class="empty-desc">Create a new download task to get started</div></div>`}
      </div>
    `;
    attachTaskEvents();
  } catch (e) {
    content.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load tasks</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

function renderTaskList(tasks) {
  return '<div class="task-list">' + tasks.map(t => {
    const p = t.progress || {};
    const pct = p.total > 0 ? Math.min(100, Math.round((p.completed || 0) / p.total * 100)) : 0;
    const tagClass = STATUS_TAG_CLASS[t.status] || 'tag-queued';
    return `<div class="task-item" data-task-id="${esc(t.task_id)}">
      <div class="task-tag"><span class="tag ${tagClass}">${t.status}</span></div>
      <div class="task-info">
        <div class="task-title">${esc(taskTypeLabel(t.type))}${t.data?.screen_name ? ': ' + esc(t.data.screen_name) : ''}${t.data?.list_id ? ' (list)' : ''}</div>
        <div class="task-meta">
          <span>${esc(t.task_id)}</span>
          ${t.created_at ? '<span>&middot; ' + relativeTime(t.created_at) + '</span>' : ''}
          ${t.started_at && t.status === 'running' ? '<span>&middot; ' + formatDuration(t.started_at) + '</span>' : ''}
          ${t.started_at && t.ended_at ? '<span>&middot; ' + formatDuration(t.started_at, t.ended_at) + '</span>' : ''}
          ${p.stage ? '<span>&middot; ' + esc(p.stage) + '</span>' : ''}
          ${p.current ? '<span>&middot; ' + esc(p.current) + '</span>' : ''}
        </div>
      </div>
      ${(t.status === 'running' || t.status === 'queued') && p.total > 0 ? `
      <div class="task-progress-wrap">
        <div class="progress-bar"><div class="progress-fill" style="width:${pct}%"></div></div>
        <div class="task-progress-text">${p.completed || 0}/${p.total}${p.failed ? ' (' + p.failed + ' failed)' : ''}</div>
      </div>` : ''}
      <div class="task-actions">
        ${t.status === 'running' || t.status === 'queued' ? `<button class="btn btn-xs btn-ghost task-cancel" title="Cancel"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>` : ''}
        ${t.status === 'failed' || t.status === 'cancelled' ? `<button class="btn btn-xs btn-ghost task-retry" title="Retry"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg></button>` : ''}
        ${t.status === 'completed' || t.status === 'failed' || t.status === 'cancelled' ? `<button class="btn btn-xs btn-ghost task-delete" title="Delete"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>` : ''}
        <button class="btn btn-xs btn-ghost task-detail" title="Details"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg></button>
      </div>
    </div>`;
  }).join('') + '</div>';
}

function attachTaskEvents() {
  document.querySelectorAll('.task-cancel').forEach(el => {
    el.addEventListener('click', async (e) => {
      e.stopPropagation();
      const id = el.closest('.task-item')?.dataset.taskId;
      if (id) { try { await API.post('/api/v1/tasks/' + id + '/cancel'); toast('Task cancelled', 'warning'); renderTasks(); } catch (ex) { toast(ex.message, 'error'); } }
    });
  });
  document.querySelectorAll('.task-retry').forEach(el => {
    el.addEventListener('click', async (e) => {
      e.stopPropagation();
      const id = el.closest('.task-item')?.dataset.taskId;
      if (id) { try { await API.post('/api/v1/tasks/' + id + '/retry'); toast('Task queued for retry', 'success'); renderTasks(); } catch (ex) { toast(ex.message, 'error'); } }
    });
  });
  document.querySelectorAll('.task-delete').forEach(el => {
    el.addEventListener('click', async (e) => {
      e.stopPropagation();
      const id = el.closest('.task-item')?.dataset.taskId;
      if (id && confirm('Delete task ' + id + '?')) { try { await API.del('/api/v1/tasks/' + id); toast('Task deleted', 'info'); renderTasks(); } catch (ex) { toast(ex.message, 'error'); } }
    });
  });
  document.querySelectorAll('.task-detail').forEach(el => {
    el.addEventListener('click', async (e) => {
      e.stopPropagation();
      const id = el.closest('.task-item')?.dataset.taskId;
      if (id) showTaskDetail(id);
    });
  });
}

async function showTaskDetail(taskId) {
  try {
    const task = await API.get('/api/v1/tasks/' + taskId);
    const p = task.progress || {};
    const r = task.result || {};
    const pct = p.total > 0 ? Math.min(100, Math.round((p.completed || 0) / p.total * 100)) : 0;
    openModal('Task: ' + esc(task.task_id), `
      <div style="display:grid;grid-template-columns:auto 1fr;gap:8px 16px;font-size:13px">
        <span style="color:var(--text-muted)">ID</span><span class="text-mono">${esc(task.task_id)}</span>
        <span style="color:var(--text-muted)">Type</span><span>${esc(taskTypeLabel(task.type))}</span>
        <span style="color:var(--text-muted)">Status</span><span><span class="tag ${STATUS_TAG_CLASS[task.status] || 'tag-queued'}">${task.status}</span></span>
        <span style="color:var(--text-muted)">Created</span><span>${formatTime(task.created_at)}</span>
        ${task.started_at ? '<span style="color:var(--text-muted)">Started</span><span>' + formatTime(task.started_at) + '</span>' : ''}
        ${task.ended_at ? '<span style="color:var(--text-muted)">Ended</span><span>' + formatTime(task.ended_at) + '</span>' : ''}
        ${task.started_at ? '<span style="color:var(--text-muted)">Duration</span><span>' + formatDuration(task.started_at, task.ended_at) + '</span>' : ''}
        ${p.stage ? '<span style="color:var(--text-muted)">Stage</span><span>' + esc(p.stage) + '</span>' : ''}
        ${p.current ? '<span style="color:var(--text-muted)">Current</span><span>' + esc(p.current) + '</span>' : ''}
      </div>
      ${p.total > 0 ? `
      <div class="mt-3">
        <div class="progress-bar"><div class="progress-fill" style="width:${pct}%"></div></div>
        <div class="text-sm text-muted mt-2">${p.completed || 0} / ${p.total} completed${p.failed ? ' (' + p.failed + ' failed)' : ''}</div>
      </div>` : ''}
      ${r.main || r.profile ? `
      <div class="mt-3" style="display:grid;grid-template-columns:auto 1fr;gap:6px 16px;font-size:13px;padding:12px;background:var(--bg-raised);border-radius:var(--radius)">
        ${r.main ? '<span style="color:var(--text-muted)">Downloaded</span><span>' + (r.main.downloaded || 0) + ' files</span>' : ''}
        ${r.main && r.main.failed ? '<span style="color:var(--text-muted)">Failed</span><span style="color:var(--danger-text)">' + r.main.failed + ' files</span>' : ''}
        ${r.profile ? '<span style="color:var(--text-muted)">Profile</span><span>' + (r.profile.downloaded || 0) + ' downloaded' + (r.profile.failed ? ', ' + r.profile.failed + ' failed' : '') + (r.profile.versioned ? ', ' + r.profile.versioned + ' versioned' : '') + '</span>' : ''}
        ${r.message ? '<span style="color:var(--text-muted)">Message</span><span>' + esc(r.message) + '</span>' : ''}
      </div>` : ''}
      ${task.error ? '<div class="mt-3" style="padding:10px 12px;background:var(--danger-bg);border-radius:var(--radius);font-size:12px;font-family:var(--font-mono);color:var(--danger-text)">' + esc(task.error) + '</div>' : ''}
    `, `
      ${task.status === 'running' || task.status === 'queued' ? '<button class="btn btn-sm btn-danger" onclick="closeModal();cancelTask(\'' + esc(task.task_id) + '\')">Cancel</button>' : ''}
      ${task.status === 'failed' || task.status === 'cancelled' ? '<button class="btn btn-sm btn-primary" onclick="closeModal();retryTask(\'' + esc(task.task_id) + '\')">Retry</button>' : ''}
      <button class="btn btn-sm btn-ghost" onclick="closeModal()">Close</button>
    `);
  } catch (e) {
    toast(e.message, 'error');
  }
}

async function cancelTask(id) { try { await API.post('/api/v1/tasks/' + id + '/cancel'); toast('Task cancelled', 'warning'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }
async function retryTask(id) { try { await API.post('/api/v1/tasks/' + id + '/retry'); toast('Task queued for retry', 'success'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }
async function cancelAllQueued() { try { await API.post('/api/v1/tasks/cancel-queued'); toast('Queued tasks cancelled', 'warning'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }
async function retryAllFailed() { try { await API.post('/api/v1/errors/retry'); toast('Retrying all failed items', 'info'); renderTasks(); } catch (e) { toast(e.message, 'error'); } }

/* ---- New Task Modal ---- */
function showNewTaskModal() {
  openModal('Create New Task', `
    <div class="tabs" id="newTaskTabs">
      <div class="tab active" data-tab="user" onclick="switchTaskTab('user')">User</div>
      <div class="tab" data-tab="list" onclick="switchTaskTab('list')">List</div>
      <div class="tab" data-tab="following" onclick="switchTaskTab('following')">Following</div>
      <div class="tab" data-tab="batch" onclick="switchTaskTab('batch')">Batch</div>
      <div class="tab" data-tab="profile" onclick="switchTaskTab('profile')">Profile</div>
      <div class="tab" data-tab="json" onclick="switchTaskTab('json')">JSON Import</div>
    </div>
    <div id="newTaskForm">
      ${renderNewUserTaskForm()}
    </div>
  `, `
    <button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button>
    <button class="btn btn-sm btn-primary" id="submitNewTask" onclick="submitNewTask()">Create Task</button>
  `);
  window._newTaskType = 'user';
}

function switchTaskTab(tab) {
  window._newTaskType = tab;
  document.querySelectorAll('#newTaskTabs .tab').forEach(el => el.classList.toggle('active', el.dataset.tab === tab));
  const forms = {
    user: renderNewUserTaskForm,
    list: renderNewListTaskForm,
    following: renderNewFollowingTaskForm,
    batch: renderNewBatchTaskForm,
    profile: renderNewProfileTaskForm,
    json: renderNewJsonTaskForm
  };
  $('newTaskForm').innerHTML = forms[tab] ? forms[tab]() : '<div class="empty-desc">Unknown task type</div>';
}

function renderNewUserTaskForm() {
  return `
    <div class="form-group"><label class="form-label">Screen Name</label><input class="form-input" id="ntScreenName" placeholder="elonmusk" required></div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntAutoFollow"> Auto-follow protected</label>
      <label class="form-checkbox"><input type="checkbox" id="ntSkipProfile"> Skip profile</label>
    </div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntNoRetry"> No retry</label>
    </div>
  `;
}
function renderNewListTaskForm() {
  return `
    <div class="form-group"><label class="form-label">List ID</label><input class="form-input" id="ntListId" placeholder="1234567890" required></div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntAutoFollow"> Auto-follow protected</label>
      <label class="form-checkbox"><input type="checkbox" id="ntFollowMembers"> Follow members</label>
    </div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntSkipProfile"> Skip profile</label>
      <label class="form-checkbox"><input type="checkbox" id="ntNoRetry"> No retry</label>
    </div>
  `;
}
function renderNewFollowingTaskForm() {
  return `
    <div class="form-group"><label class="form-label">Screen Name</label><input class="form-input" id="ntScreenName" placeholder="elonmusk" required></div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntAutoFollow"> Auto-follow protected</label>
      <label class="form-checkbox"><input type="checkbox" id="ntSkipProfile"> Skip profile</label>
    </div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntNoRetry"> No retry</label>
    </div>
  `;
}
function renderNewBatchTaskForm() {
  return `
    <div class="form-group"><label class="form-label">Users (comma-separated)</label><input class="form-input" id="ntBatchUsers" placeholder="user1, user2"></div>
    <div class="form-group"><label class="form-label">List IDs (comma-separated)</label><input class="form-input" id="ntBatchLists" placeholder="123, 456"></div>
    <div class="form-group"><label class="form-label">Following (comma-separated)</label><input class="form-input" id="ntBatchFollowing" placeholder="elonmusk"></div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntAutoFollow"> Auto-follow protected</label>
      <label class="form-checkbox"><input type="checkbox" id="ntFollowMembers"> Follow members</label>
    </div>
    <div class="form-row">
      <label class="form-checkbox"><input type="checkbox" id="ntSkipProfile"> Skip profile</label>
      <label class="form-checkbox"><input type="checkbox" id="ntNoRetry"> No retry</label>
    </div>
  `;
}
function renderNewProfileTaskForm() {
  return `
    <div class="form-group"><label class="form-label">Screen Names (comma-separated)</label><input class="form-input" id="ntProfileUsers" placeholder="user1, user2" required></div>
    <div class="form-group"><label class="form-label">List ID (optional)</label><input class="form-input" id="ntProfileList" placeholder="1234567890"></div>
  `;
}
function renderNewJsonTaskForm() {
  return `
    <div class="form-group"><label class="form-label">File / Folder Paths (one per line)</label><textarea class="form-textarea" id="ntJsonPaths" placeholder="/path/to/file.json" rows="3"></textarea></div>
    <div class="form-hint">JSON files from third-party tools or .loongtweet folders</div>
    <label class="form-checkbox mt-2"><input type="checkbox" id="ntNoRetry"> No retry</label>
  `;
}

async function submitNewTask() {
  const type = window._newTaskType;
  let url, body;
  const getVal = (id) => $(id)?.value?.trim();
  const getChecked = (id) => $(id)?.checked || false;

  try {
    switch (type) {
      case 'user': {
        const sn = getVal('ntScreenName');
        if (!sn) { toast('Screen name is required', 'error'); return; }
        url = '/api/v1/users/' + encodeURIComponent(sn) + '/download';
        body = { auto_follow: getChecked('ntAutoFollow'), skip_profile: getChecked('ntSkipProfile'), no_retry: getChecked('ntNoRetry'), follow_members: false };
        break;
      }
      case 'list': {
        const lid = getVal('ntListId');
        if (!lid) { toast('List ID is required', 'error'); return; }
        url = '/api/v1/lists/' + lid + '/download';
        body = { auto_follow: getChecked('ntAutoFollow'), follow_members: getChecked('ntFollowMembers'), skip_profile: getChecked('ntSkipProfile'), no_retry: getChecked('ntNoRetry') };
        break;
      }
      case 'following': {
        const sn = getVal('ntScreenName');
        if (!sn) { toast('Screen name is required', 'error'); return; }
        url = '/api/v1/users/' + encodeURIComponent(sn) + '/following/download';
        body = { auto_follow: getChecked('ntAutoFollow'), skip_profile: getChecked('ntSkipProfile'), no_retry: getChecked('ntNoRetry'), follow_members: false };
        break;
      }
      case 'batch': {
        const users = getVal('ntBatchUsers')?.split(',').map(s => s.trim()).filter(Boolean) || [];
        const lists = getVal('ntBatchLists')?.split(',').map(s => s.trim()).filter(Boolean) || [];
        const following = getVal('ntBatchFollowing')?.split(',').map(s => s.trim()).filter(Boolean) || [];
        if (!users.length && !lists.length && !following.length) { toast('Enter at least one user, list ID, or following name', 'error'); return; }
        url = '/api/v1/batch/download';
        body = { users, lists, following_names: following, auto_follow: getChecked('ntAutoFollow'), follow_members: getChecked('ntFollowMembers'), skip_profile: getChecked('ntSkipProfile'), no_retry: getChecked('ntNoRetry') };
        break;
      }
      case 'profile': {
        const users = getVal('ntProfileUsers')?.split(',').map(s => s.trim()).filter(Boolean) || [];
        const listId = getVal('ntProfileList');
        if (!users.length && !listId) { toast('Enter screen names or a list ID', 'error'); return; }
        if (listId) {
          url = '/api/v1/lists/' + listId + '/profile';
        } else {
          url = '/api/v1/users/' + encodeURIComponent(users[0]) + '/profile';
          if (users.length > 1) { toast('Profile download supports one user at a time via this form. Use batch for multiple.', 'warning'); }
        }
        body = { screen_name: users[0] };
        break;
      }
      case 'json': {
        const paths = getVal('ntJsonPaths')?.split('\n').map(s => s.trim()).filter(Boolean) || [];
        if (!paths.length) { toast('Enter at least one path', 'error'); return; }
        const isFolder = paths[0].includes('.loongtweet') || paths.every(p => !p.endsWith('.json'));
        url = isFolder ? '/api/v1/json/folder/download' : '/api/v1/json/file/download';
        body = { paths, no_retry: getChecked('ntNoRetry') };
        break;
      }
    }

    const data = await API.post(url, body);
    closeModal();
    toast('Task created: ' + (data?.task_id || ''), 'success');
    if (state.currentPage === 'tasks') renderTasks();
  } catch (e) {
    toast(e.message, 'error');
  }
}

/* ---- Page: Data ---- */
async function renderData() {
  const content = $('content');
  content.innerHTML = `
    <div class="tabs" id="dataTabs">
      <div class="tab active" data-tab="users" onclick="switchDataTab('users')">Users</div>
      <div class="tab" data-tab="lists" onclick="switchDataTab('lists')">Lists</div>
      <div class="tab" data-tab="entities" onclick="switchDataTab('entities')">User Entities</div>
      <div class="tab" data-tab="list-entities" onclick="switchDataTab('list-entities')">List Entities</div>
      <div class="tab" data-tab="links" onclick="switchDataTab('links')">User Links</div>
      <div class="tab" data-tab="stats" onclick="switchDataTab('stats')">Stats</div>
    </div>
    <div id="dataContent"><div class="page-loading"><div class="spinner"></div></div></div>
  `;
  state.dataActiveTab = 'users';
  switchDataTab('users');
}

async function switchDataTab(tab) {
  state.dataActiveTab = tab;
  document.querySelectorAll('#dataTabs .tab').forEach(el => el.classList.toggle('active', el.dataset.tab === tab));
  const container = $('dataContent');
  container.innerHTML = '<div class="page-loading"><div class="spinner"></div></div>';
  try {
    switch (tab) {
      case 'users': await renderDataUsers(container); break;
      case 'lists': await renderDataLists(container); break;
      case 'entities': await renderDataEntities(container); break;
      case 'list-entities': await renderDataListEntities(container); break;
      case 'links': await renderDataLinks(container); break;
      case 'stats': await renderDataStats(container); break;
    }
  } catch (e) {
    container.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load data</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

async function renderDataUsers(container) {
  const data = await API.get('/api/v1/db/users');
  const items = Array.isArray(data) ? data : data?.users || data?.data || [];
  container.innerHTML = `
    <div class="card">
      <div class="card-header"><span class="card-title">Users (${items.length})</span></div>
      ${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>Screen Name</th><th>Name</th><th>Protected</th><th>Accessible</th><th class="table-cell-right">Actions</th></tr></thead><tbody>${items.map(u => `<tr>
        <td class="table-cell-mono">${esc(u.id)}</td>
        <td><strong>${esc(u.screen_name)}</strong></td>
        <td class="table-cell-muted">${esc(u.name || '-')}</td>
        <td>${u.protected ? '<span class="tag tag-warning">Yes</span>' : '<span class="tag tag-success">No</span>'}</td>
        <td>${u.is_accessible ? '<span class="tag tag-success">Yes</span>' : '<span class="tag tag-failed">No</span>'}</td>
        <td class="table-cell-right"><button class="btn btn-xs btn-ghost" onclick="showUserDetail('${esc(u.id)}')">View</button></td>
      </tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No users in database</div></div>'}
    </div>`;
}

async function showUserDetail(id) {
  try {
    const user = await API.get('/api/v1/db/users/' + id);
    const entities = await API.get('/api/v1/db/users/' + id + '/entities').catch(() => []);
    const links = await API.get('/api/v1/db/users/' + id + '/links').catch(() => []);
    const prevNames = await API.get('/api/v1/db/users/' + id + '/previous-names').catch(() => []);
    const ents = Array.isArray(entities) ? entities : [];
    const lks = Array.isArray(links) ? links : [];
    const prev = Array.isArray(prevNames) ? prevNames : [];

    openModal('User: ' + esc(user.screen_name), `
      <div class="data-detail-grid">
        <span class="data-detail-label">ID</span><span class="data-detail-value text-mono">${esc(user.id)}</span>
        <span class="data-detail-label">Screen Name</span><span class="data-detail-value"><strong>${esc(user.screen_name)}</strong></span>
        <span class="data-detail-label">Display Name</span><span class="data-detail-value">${esc(user.name || '-')}</span>
        <span class="data-detail-label">Protected</span><span class="data-detail-value">${user.protected ? 'Yes' : 'No'}</span>
        <span class="data-detail-label">Accessible</span><span class="data-detail-value">${user.is_accessible ? 'Yes' : 'No'}</span>
        <span class="data-detail-label">Friends</span><span class="data-detail-value">${user.friends_count ?? '-'}</span>
      </div>
      ${ents.length ? `<div class="mt-3"><div class="text-sm text-muted mb-2">Entities (${ents.length})</div><div class="table-wrap"><table><thead><tr><th>Name</th><th>Parent Dir</th><th>Media</th><th>Latest</th></tr></thead><tbody>${ents.map(e => `<tr><td class="text-mono">${esc(e.name)}</td><td class="table-cell-muted">${esc(e.parent_dir || '-')}</td><td>${e.media_count != null ? e.media_count : '-'}</td><td class="table-cell-muted">${e.latest_release_time || '-'}</td></tr>`).join('')}</tbody></table></div></div>` : ''}
      ${lks.length ? `<div class="mt-3"><div class="text-sm text-muted mb-2">Linked Lists (${lks.length})</div>${lks.map(l => '<div class="text-sm" style="padding:4px 0">' + esc(l.name) + ' <span class="text-muted">→ ' + esc(l.parent_lst_entity_name || '-') + '</span></div>').join('')}</div>` : ''}
      ${prev.length ? `<div class="mt-3"><div class="text-sm text-muted mb-2">Previous Names</div>${prev.map(p => '<div class="text-sm" style="padding:2px 0"><span class="text-mono">' + esc(p.screen_name) + '</span> <span class="text-muted">(' + esc(p.record_date) + ')</span></div>').join('')}</div>` : ''}
    `, `
      <button class="btn btn-sm btn-danger" onclick="deleteUser('${esc(user.id)}')">Delete User</button>
      <button class="btn btn-sm btn-ghost" onclick="closeModal()">Close</button>
    `);
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteUser(id) {
  if (!confirm('Delete user ' + id + ' and all associated data?')) return;
  try { await API.del('/api/v1/db/users/' + id); toast('User deleted', 'info'); closeModal(); switchDataTab('users'); } catch (e) { toast(e.message, 'error'); }
}

async function renderDataLists(container) {
  const data = await API.get('/api/v1/db/lists');
  const items = Array.isArray(data) ? data : [];
  container.innerHTML = `
    <div class="card">
      <div class="card-header"><span class="card-title">Lists (${items.length})</span></div>
      ${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>Name</th><th>Owner ID</th><th class="table-cell-right">Actions</th></tr></thead><tbody>${items.map(l => `<tr>
        <td class="table-cell-mono">${esc(l.id)}</td>
        <td><strong>${esc(l.name)}</strong></td>
        <td class="table-cell-mono">${esc(l.owner_user_id || '-')}</td>
        <td class="table-cell-right"><button class="btn btn-xs btn-ghost" onclick="showListDetail('${esc(l.id)}')">View</button></td>
      </tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No lists in database</div></div>'}
    </div>`;
}

async function showListDetail(id) {
  try {
    const list = await API.get('/api/v1/db/lists/' + id);
    const entities = await API.get('/api/v1/db/lists/' + id + '/entities').catch(() => []);
    const ents = Array.isArray(entities) ? entities : [];
    openModal('List: ' + esc(list.name), `
      <div class="data-detail-grid">
        <span class="data-detail-label">ID</span><span class="data-detail-value text-mono">${esc(list.id)}</span>
        <span class="data-detail-label">Name</span><span class="data-detail-value"><strong>${esc(list.name)}</strong></span>
        <span class="data-detail-label">Owner</span><span class="data-detail-value text-mono">${esc(list.owner_user_id || '-')}</span>
      </div>
      ${ents.length ? `<div class="mt-3"><div class="text-sm text-muted mb-2">Entities (${ents.length})</div>${ents.map(e => '<div class="text-sm" style="padding:4px 0"><span class="text-mono">' + esc(e.name) + '</span> <span class="text-muted">' + esc(e.parent_dir || '') + '</span></div>').join('')}</div>` : ''}
    `, `
      <button class="btn btn-sm btn-danger" onclick="deleteList('${esc(list.id)}')">Delete List</button>
      <button class="btn btn-sm btn-ghost" onclick="closeModal()">Close</button>
    `);
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteList(id) {
  if (!confirm('Delete list ' + id + ' and all associated data?')) return;
  try { await API.del('/api/v1/db/lists/' + id); toast('List deleted', 'info'); closeModal(); switchDataTab('lists'); } catch (e) { toast(e.message, 'error'); }
}

async function renderDataEntities(container) {
  const data = await API.get('/api/v1/db/user-entities');
  const items = Array.isArray(data) ? data : [];
  container.innerHTML = `
    <div class="card">
      <div class="card-header"><span class="card-title">User Entities (${items.length})</span></div>
      ${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>User ID</th><th>Name</th><th>Parent Dir</th><th>Media</th><th>Latest Release</th></tr></thead><tbody>${items.map(e => `<tr>
        <td class="table-cell-mono">${esc(e.id)}</td>
        <td class="table-cell-mono">${esc(e.user_id)}</td>
        <td>${esc(e.name)}</td>
        <td class="table-cell-muted">${esc(e.parent_dir || '-')}</td>
        <td>${e.media_count != null ? e.media_count : '-'}</td>
        <td class="table-cell-muted">${e.latest_release_time || '-'}</td>
      </tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No user entities</div></div>'}
    </div>`;
}

async function renderDataListEntities(container) {
  const data = await API.get('/api/v1/db/list-entities');
  const items = Array.isArray(data) ? data : [];
  container.innerHTML = `
    <div class="card">
      <div class="card-header"><span class="card-title">List Entities (${items.length})</span></div>
      ${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>List ID</th><th>Name</th><th>Parent Dir</th><th>List Name</th></tr></thead><tbody>${items.map(e => `<tr>
        <td class="table-cell-mono">${esc(e.id)}</td>
        <td class="table-cell-mono">${esc(e.lst_id)}</td>
        <td>${esc(e.name)}</td>
        <td class="table-cell-muted">${esc(e.parent_dir || '-')}</td>
        <td class="table-cell-muted">${esc(e.list_name || '-')}</td>
      </tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No list entities</div></div>'}
    </div>`;
}

async function renderDataLinks(container) {
  const data = await API.get('/api/v1/db/user-links');
  const items = Array.isArray(data) ? data : [];
  container.innerHTML = `
    <div class="card">
      <div class="card-header"><span class="card-title">User Links (${items.length})</span></div>
      ${items.length ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>User ID</th><th>Name</th><th>Parent Entity ID</th><th>List Entity</th></tr></thead><tbody>${items.map(l => `<tr>
        <td class="table-cell-mono">${esc(l.id)}</td>
        <td class="table-cell-mono">${esc(l.user_id)}</td>
        <td>${esc(l.name)}</td>
        <td class="table-cell-mono">${esc(l.parent_lst_entity_id || '-')}</td>
        <td class="table-cell-muted">${esc(l.parent_lst_entity_name || '-')}</td>
      </tr>`).join('')}</tbody></table></div>` : '<div class="empty-state"><div class="empty-desc">No user links</div></div>'}
    </div>`;
}

async function renderDataStats(container) {
  const stats = await API.get('/api/v1/db/stats');
  container.innerHTML = `
    <div class="stats-grid">
      <div class="stat-card"><div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg></div><div class="stat-content"><div class="stat-value">${stats.users ?? '-'}</div><div class="stat-label">Users</div></div></div>
      <div class="stat-card"><div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/><rect x="9" y="3" width="6" height="4" rx="1"/></svg></div><div class="stat-content"><div class="stat-value">${stats.lists ?? '-'}</div><div class="stat-label">Lists</div></div></div>
      <div class="stat-card"><div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg></div><div class="stat-content"><div class="stat-value">${stats.user_entities ?? '-'}</div><div class="stat-label">User Entities</div></div></div>
      <div class="stat-card"><div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg></div><div class="stat-content"><div class="stat-value">${stats.list_entities ?? '-'}</div><div class="stat-label">List Entities</div></div></div>
      <div class="stat-card"><div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="17" y1="10" x2="3" y2="10"/><line x1="21" y1="6" x2="3" y2="6"/><line x1="17" y1="14" x2="3" y2="14"/><line x1="21" y1="18" x2="3" y2="18"/></svg></div><div class="stat-content"><div class="stat-value">${stats.user_links ?? '-'}</div><div class="stat-label">User Links</div></div></div>
      <div class="stat-card"><div class="stat-icon accent"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></div><div class="stat-content"><div class="stat-value">${stats.user_previous_names ?? '-'}</div><div class="stat-label">Previous Names</div></div></div>
    </div>`;
}

/* ---- Page: Schedules ---- */
async function renderSchedules() {
  const content = $('content');
  try {
    const [schedulesData, statsData, rawData] = await Promise.all([
      API.get('/api/v1/schedules').catch(() => ({ entries: [] })),
      API.get('/api/v1/schedules/stats').catch(() => null),
      API.get('/api/v1/schedules/raw').catch(() => null),
    ]);
    const entries = Array.isArray(schedulesData) ? schedulesData : schedulesData?.entries || [];
    state.schedules = entries;

    content.innerHTML = `
      <div class="toolbar">
        <div class="toolbar-left">
          <button class="btn btn-sm btn-primary" onclick="showNewScheduleModal()">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            New Schedule
          </button>
          <button class="btn btn-sm btn-secondary" onclick="showScheduleRawEditor()">Edit Raw</button>
          <button class="btn btn-sm btn-ghost" onclick="triggerAllSchedules()">Trigger All</button>
        </div>
        <div class="toolbar-right">
          <span class="text-sm text-muted">${plural(entries.length, 'schedule')}${statsData ? ' &middot; ' + statsData.enabled + ' enabled' : ''}</span>
        </div>
      </div>
      <div class="card">
        ${entries.length ? entries.map(s => {
          const typeLabel = SCHEDULE_LABELS[s.type] || s.type;
          const statusClass = s.enabled ? 'tag-success' : 'tag-queued';
          const hasFailures = s.consecutive_failures > 0;
          return `<div class="schedule-item ${hasFailures ? 'has-failure' : ''}">
            <div class="schedule-type"><span class="tag tag-mono ${statusClass}">${typeLabel}</span></div>
            <div class="schedule-info">
              <div class="schedule-title">${esc(s.name || s.target || 'Unnamed')}</div>
              <div class="schedule-meta">
                <span>${s.target ? esc(s.target) : ''}</span>
                ${s.schedule ? '<span>&middot; ' + esc(s.schedule) + '</span>' : ''}
                ${s.next_run ? '<span>&middot; Next: ' + relativeTime(s.next_run) + '</span>' : ''}
                ${s.last_run ? '<span>&middot; Last: ' + relativeTime(s.last_run) + '</span>' : ''}
                ${hasFailures ? '<span class="tag tag-failed">' + s.consecutive_failures + ' failures</span>' : ''}
              </div>
            </div>
            <div class="schedule-toggle">
              <label class="toggle"><input type="checkbox" ${s.enabled ? 'checked' : ''} onchange="toggleSchedule('${esc(s.id || '')}', this.checked)"><span class="toggle-slider"></span></label>
            </div>
            <div class="schedule-actions">
              <button class="btn btn-xs btn-ghost" onclick="triggerSchedule('${esc(s.id || '')}')" title="Trigger now">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polygon points="5 3 19 12 5 21 5 3"/></svg>
              </button>
              <button class="btn btn-xs btn-ghost" onclick="deleteSchedule('${esc(s.id || '')}')" title="Delete">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
              </button>
            </div>
          </div>`;
        }).join('') : '<div class="empty-state"><div class="empty-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></div><div class="empty-title">No schedules</div><div class="empty-desc">Create a scheduled download task</div></div>'}
      </div>
    `;
  } catch (e) {
    content.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load schedules</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

async function toggleSchedule(id, enabled) {
  if (!id) return;
  try { await API.patch('/api/v1/schedules/' + id + '/enabled', { enabled }); toast(enabled ? 'Schedule enabled' : 'Schedule disabled', 'info'); renderSchedules(); } catch (e) { toast(e.message, 'error'); }
}
async function triggerSchedule(id) {
  if (!id) return;
  try { await API.post('/api/v1/schedules/' + id + '/trigger'); toast('Schedule triggered', 'success'); } catch (e) { toast(e.message, 'error'); }
}
async function triggerAllSchedules() {
  try { const r = await API.post('/api/v1/schedules/trigger-all'); toast('Triggered ' + (r.succeeded || 0) + ' schedules', 'info'); renderSchedules(); } catch (e) { toast(e.message, 'error'); }
}
async function deleteSchedule(id) {
  if (!id || !confirm('Delete this schedule?')) return;
  try { await API.del('/api/v1/schedules/' + id); toast('Schedule deleted', 'info'); renderSchedules(); } catch (e) { toast(e.message, 'error'); }
}

function showNewScheduleModal() {
  openModal('New Schedule', `
    <div class="form-group">
      <label class="form-label">Type</label>
      <select class="form-select" id="nsType">
        <option value="user">User</option>
        <option value="list">List</option>
        <option value="following">Following</option>
        <option value="mixed">Mixed</option>
      </select>
    </div>
    <div class="form-group">
      <label class="form-label">Target</label>
      <input class="form-input" id="nsTarget" placeholder="screen_name or list_id">
      <div class="form-hint">Screen name for user/following, list ID for list, or comma-separated for mixed</div>
    </div>
    <div class="form-group">
      <label class="form-label">Name</label>
      <input class="form-input" id="nsName" placeholder="My Schedule">
    </div>
    <div class="form-group">
      <label class="form-label">Schedule</label>
      <input class="form-input" id="nsSchedule" placeholder='daily 08:00,20:00 or interval 4h'>
      <div class="form-hint">Format: "daily HH:MM" or "interval DURATION" (e.g. "daily 08:00,20:00" or "interval 4h30m")</div>
    </div>
    <label class="form-checkbox"><input type="checkbox" id="nsEnabled" checked> Enabled</label>
    <label class="form-checkbox"><input type="checkbox" id="nsRunOnStart"> Run on start</label>
  `, `
    <button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button>
    <button class="btn btn-sm btn-primary" onclick="submitNewSchedule()">Create</button>
  `);
}

async function submitNewSchedule() {
  const type = $('nsType')?.value;
  const target = $('nsTarget')?.value?.trim();
  const name = $('nsName')?.value?.trim();
  const schedule = $('nsSchedule')?.value?.trim();
  if (!target || !schedule) { toast('Target and schedule are required', 'error'); return; }
  try {
    const entry = { type, target, name: name || target, schedule, enabled: $('nsEnabled')?.checked || false, run_on_start: $('nsRunOnStart')?.checked || false };
    await API.post('/api/v1/schedules', { entries: [entry] });
    closeModal();
    toast('Schedule created', 'success');
    renderSchedules();
  } catch (e) { toast(e.message, 'error'); }
}

async function showScheduleRawEditor() {
  try {
    const raw = await API.get('/api/v1/schedules/raw');
    openModal('Schedule Configuration (Raw)', `
      <div class="form-hint mb-3">Edit schedules.yaml contents directly. Use caution - invalid YAML will break scheduling.</div>
      <div class="code-editor" id="schedulesEditor" contenteditable="true">${esc(raw.content || '# No schedules')}</div>
    `, `
      <button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button>
      <button class="btn btn-sm btn-primary" onclick="saveScheduleRaw()">Save & Reload</button>
    `);
  } catch (e) { toast(e.message, 'error'); }
}

async function saveScheduleRaw() {
  const editor = $('schedulesEditor');
  const content = editor?.textContent || '';
  try {
    await API.put('/api/v1/schedules/raw', { content });
    closeModal();
    toast('Schedules updated and reloaded', 'success');
    renderSchedules();
  } catch (e) { toast(e.message, 'error'); }
}

/* ---- Page: Config ---- */
async function renderConfig() {
  const content = $('content');
  content.innerHTML = `
    <div class="tabs" id="configTabs">
      <div class="tab active" data-tab="settings" onclick="switchConfigTab('settings')">Settings</div>
      <div class="tab" data-tab="cookies" onclick="switchConfigTab('cookies')">Cookies</div>
      <div class="tab" data-tab="raw" onclick="switchConfigTab('raw')">Raw Config</div>
    </div>
    <div id="configContent"><div class="page-loading"><div class="spinner"></div></div></div>
  `;
  switchConfigTab('settings');
}

async function switchConfigTab(tab) {
  document.querySelectorAll('#configTabs .tab').forEach(el => el.classList.toggle('active', el.dataset.tab === tab));
  const container = $('configContent');
  container.innerHTML = '<div class="page-loading"><div class="spinner"></div></div>';
  try {
    switch (tab) {
      case 'settings': await renderConfigSettings(container); break;
      case 'cookies': await renderConfigCookies(container); break;
      case 'raw': await renderConfigRaw(container); break;
    }
  } catch (e) {
    container.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

async function renderConfigSettings(container) {
  let fields, config;
  try {
    [fields, config] = await Promise.all([
      API.get('/api/v1/config/fields'),
      API.get('/api/v1/config').catch(() => null)
    ]);
  } catch (e) {
    container.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load config</div><div class="empty-desc">${esc(e.message)}</div></div>`;
    return;
  }

  const fieldList = Array.isArray(fields?.fields) ? fields.fields : [];

  container.innerHTML = `
    <div class="card">
      <div class="card-header">
        <span class="card-title">Configuration</span>
        <div class="card-actions"><button class="btn btn-sm btn-primary" onclick="saveConfigSettings()">Save Settings</button></div>
      </div>
      <div class="card-body" id="configSettingsForm">
        ${fieldList.length ? fieldList.map(f => `
          <div class="form-group">
            <label class="form-label">${esc(f.label || f.name)}</label>
            ${f.type === 'number' ?
              `<input class="form-input" id="cfg_${esc(f.name)}" type="number" value="${esc(f.value || f.default || '')}" placeholder="${esc(f.placeholder || '')}" ${f.required ? 'required' : ''}>`
              : f.type === 'boolean' ?
              `<label class="form-checkbox"><input type="checkbox" id="cfg_${esc(f.name)}" ${f.value === 'true' ? 'checked' : ''}> ${esc(f.prompt || '')}</label>`
              : f.name === 'auth_token' || f.name === 'ct0' ?
              `<input class="form-input" id="cfg_${esc(f.name)}" type="password" value="${esc(f.value || '')}" placeholder="${esc(f.placeholder || '')}" ${f.required ? 'required' : ''}>`
              : `<input class="form-input" id="cfg_${esc(f.name)}" value="${esc(f.value || '')}" placeholder="${esc(f.placeholder || '')}" ${f.required ? 'required' : ''}>`
            }
            <div class="form-hint">${esc(f.prompt || '')}</div>
          </div>
        `).join('') : '<div class="empty-desc">No configuration fields available</div>'}
      </div>
    </div>
    ${config ? `
    <div class="card mt-3">
      <div class="card-header"><span class="card-title">Current Settings</span></div>
      <div class="card-body">
        <div style="display:grid;grid-template-columns:auto 1fr;gap:6px 16px;font-size:13px">
          <span style="color:var(--text-muted)">Root Path</span><span class="text-mono">${esc(config.root_path || '-')}</span>
          <span style="color:var(--text-muted)">Max Download Routine</span><span>${config.max_download_routine ?? '-'}</span>
          <span style="color:var(--text-muted)">Max File Name Length</span><span>${config.max_file_name_len ?? '-'}</span>
        </div>
      </div>
    </div>` : ''}
  `;
}

async function saveConfigSettings() {
  const fields = {};
  document.querySelectorAll('[id^="cfg_"]').forEach(el => {
    const name = el.id.replace('cfg_', '');
    if (el.type === 'checkbox') fields[name] = el.checked ? 'true' : 'false';
    else fields[name] = el.value;
  });
  try {
    await API.put('/api/v1/config/fields', { fields });
    toast('Configuration saved. Some changes may require a restart.', 'success');
    renderConfigSettings($('configContent'));
  } catch (e) { toast(e.message, 'error'); }
}

async function renderConfigCookies(container) {
  let cookies, raw;
  try {
    [cookies, raw] = await Promise.all([
      API.get('/api/v1/cookies').catch(() => null),
      API.get('/api/v1/cookies/raw').catch(() => null)
    ]);
  } catch (e) {
    container.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load cookies</div><div class="empty-desc">${esc(e.message)}</div></div>`;
    return;
  }

  const cookieList = Array.isArray(cookies?.cookies) ? cookies.cookies : [];
  container.innerHTML = `
    <div class="card">
      <div class="card-header">
        <span class="card-title">Twitter Cookies</span>
        <div class="card-actions">
          <button class="btn btn-sm btn-secondary" onclick="showCookiesRawEditor()">Edit Raw</button>
          <button class="btn btn-sm btn-primary" onclick="saveCookiesForm()">Save Cookies</button>
        </div>
      </div>
      <div class="card-body" id="cookiesForm">
        <div class="form-group">
          <label class="form-label">Main Account - Auth Token</label>
          <input class="form-input" id="cookieMainAuth" type="password" placeholder="auth_token...">
          <div class="form-hint">Leave empty to keep current value</div>
        </div>
        <div class="form-group">
          <label class="form-label">Main Account - ct0</label>
          <input class="form-input" id="cookieMainCt0" type="password" placeholder="ct0...">
          <div class="form-hint">Leave empty to keep current value</div>
        </div>
        ${cookieList.length ? `
        <div class="text-sm text-muted mb-3 mt-3" style="border-top:1px solid var(--border);padding-top:16px">Additional Accounts (${cookieList.length})</div>
        ${cookieList.map((c, i) => `
          <div class="form-row mb-3">
            <div class="form-group">
              <label class="form-label">Account ${i + 1} - Auth Token</label>
              <input class="form-input" id="cookieExtraAuth_${i}" type="password" placeholder="auth_token..." value="${esc(c.auth_token || '')}">
            </div>
            <div class="form-group">
              <label class="form-label">Account ${i + 1} - ct0</label>
              <input class="form-input" id="cookieExtraCt0_${i}" type="password" placeholder="ct0..." value="${esc(c.ct0 || '')}">
            </div>
          </div>
        `).join('')}` : '<div class="text-sm text-muted mt-3">No additional accounts configured</div>'}
      </div>
    </div>
  `;
}

async function saveCookiesForm() {
  const cookies = [];
  const mainAuth = $('cookieMainAuth')?.value?.trim();
  const mainCt0 = $('cookieMainCt0')?.value?.trim();

  if (mainAuth && mainCt0) {
    cookies.push({ auth_token: mainAuth, ct0: mainCt0 });
  }

  let i = 0;
  while ($('cookieExtraAuth_' + i)) {
    const auth = $('cookieExtraAuth_' + i)?.value?.trim();
    const ct0 = $('cookieExtraCt0_' + i)?.value?.trim();
    if (auth || ct0) {
      if (auth && ct0) cookies.push({ auth_token: auth, ct0: ct0, index: i });
      else toast('Account ' + (i + 1) + ': both auth_token and ct0 are required', 'error');
    }
    i++;
  }

  try {
    await API.put('/api/v1/cookies', { cookies });
    toast('Cookies saved', 'success');
    renderConfigCookies($('configContent'));
  } catch (e) { toast(e.message, 'error'); }
}

async function showCookiesRawEditor() {
  try {
    const raw = await API.get('/api/v1/cookies/raw');
    openModal('Cookies Configuration (Raw)', `
      <div class="form-hint mb-3">Edit additional_cookies.yaml contents directly.</div>
      <div class="code-editor" id="cookiesEditor" contenteditable="true">${esc(raw.content || '# No cookies')}</div>
    `, `
      <button class="btn btn-sm btn-ghost" onclick="closeModal()">Cancel</button>
      <button class="btn btn-sm btn-primary" onclick="saveCookiesRaw()">Save</button>
    `);
  } catch (e) { toast(e.message, 'error'); }
}

async function saveCookiesRaw() {
  const editor = $('cookiesEditor');
  const content = editor?.textContent || '';
  try {
    await API.put('/api/v1/cookies/raw', { content });
    closeModal();
    toast('Cookies updated', 'success');
    renderConfigCookies($('configContent'));
  } catch (e) { toast(e.message, 'error'); }
}

async function renderConfigRaw(container) {
  try {
    const [configRaw, cookiesRaw, schedulesRaw] = await Promise.all([
      API.get('/api/v1/config/raw').catch(() => null),
      API.get('/api/v1/cookies/raw').catch(() => null),
      API.get('/api/v1/schedules/raw').catch(() => null),
    ]);

    container.innerHTML = `
      <div class="card">
        <div class="card-header">
          <span class="card-title">conf.yaml</span>
          <div class="card-actions"><button class="btn btn-sm btn-primary" onclick="saveRawConfigFile()">Save</button></div>
        </div>
        <div class="code-editor" id="rawConfigEditor" contenteditable="true">${esc(configRaw?.content || '# No configuration file')}</div>
      </div>
      <div class="card mt-3">
        <div class="card-header">
          <span class="card-title">additional_cookies.yaml</span>
          <div class="card-actions"><button class="btn btn-sm btn-primary" onclick="saveRawCookiesFile()">Save</button></div>
        </div>
        <div class="code-editor" id="rawCookiesEditor" contenteditable="true">${esc(cookiesRaw?.content || '# No cookies file')}</div>
      </div>
      <div class="card mt-3">
        <div class="card-header">
          <span class="card-title">schedules.yaml</span>
          <div class="card-actions"><button class="btn btn-sm btn-primary" onclick="saveRawSchedulesFile()">Save</button></div>
        </div>
        <div class="code-editor" id="rawSchedulesEditor" contenteditable="true">${esc(schedulesRaw?.content || '# No schedules file')}</div>
      </div>
    `;
  } catch (e) {
    container.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load raw config</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

async function saveRawConfigFile() {
  const content = $('rawConfigEditor')?.textContent || '';
  try { await API.put('/api/v1/config/raw', { content }); toast('Configuration saved. Restart server to apply.', 'success'); } catch (e) { toast(e.message, 'error'); }
}
async function saveRawCookiesFile() {
  const content = $('rawCookiesEditor')?.textContent || '';
  try { await API.put('/api/v1/cookies/raw', { content }); toast('Cookies saved', 'success'); } catch (e) { toast(e.message, 'error'); }
}
async function saveRawSchedulesFile() {
  const content = $('rawSchedulesEditor')?.textContent || '';
  try { await API.put('/api/v1/schedules/raw', { content }); toast('Schedules saved and reloaded', 'success'); } catch (e) { toast(e.message, 'error'); }
}

/* ---- Page: Logs ---- */
let logStreamActive = false;
let logStreamReader = null;
let logAutoScroll = true;

async function renderLogs() {
  const content = $('content');
  try {
    const [logsData, statsData] = await Promise.all([
      API.get('/api/v1/logs').catch(() => ({ logs: [] })),
      API.get('/api/v1/logs/stats').catch(() => null),
    ]);
    const logs = Array.isArray(logsData?.logs) ? logsData.logs : [];
    const total = logsData?.total || logs.length;

    content.innerHTML = `
      <div class="card">
        <div class="log-bar">
          <div class="log-status">
            <span class="sse-dot ${logStreamActive ? 'connected' : 'disconnected'}" id="logStreamDot"></span>
            <span id="logStreamText">${logStreamActive ? 'Streaming' : 'Paused'}</span>
            <span class="text-muted">&middot; ${total} entries</span>
            ${statsData ? '<span class="text-muted">&middot; ' + esc(statsData.file || '') + '</span>' : ''}
            ${statsData ? '<span class="text-muted">&middot; ' + (statsData.size || '') + '</span>' : ''}
          </div>
          <div class="log-actions">
            <label class="form-checkbox" style="font-size:11px"><input type="checkbox" id="logAutoScroll" checked onchange="logAutoScroll=this.checked"> Auto-scroll</label>
            <button class="btn btn-xs btn-ghost" onclick="clearLogDisplay()">Clear</button>
            <button class="btn btn-xs btn-secondary" onclick="exportLogs()">Export</button>
            <button class="btn btn-xs ${logStreamActive ? 'btn-danger' : 'btn-primary'}" id="logToggleBtn" onclick="toggleLogStream()">${logStreamActive ? 'Stop' : 'Stream'}</button>
          </div>
        </div>
        <div class="log-container" id="logContainer">${logs.map(l => formatLogLine(l)).join('\n')}</div>
      </div>
    `;

    if (logStreamActive) {
      startLogStream();
    }
  } catch (e) {
    content.innerHTML = `<div class="empty-state"><div class="empty-title">Failed to load logs</div><div class="empty-desc">${esc(e.message)}</div></div>`;
  }
}

function formatLogLine(line) {
  const clean = esc(stripAnsi(line));
  const level = logLevel(line);
  return '<div class="log-line' + (level ? ' ' + level : '') + '">' + clean + '</div>';
}

function appendLogLine(line) {
  const container = $('logContainer');
  if (!container) return;
  const div = document.createElement('div');
  div.className = 'log-line' + (logLevel(line) ? ' ' + logLevel(line) : '');
  div.textContent = stripAnsi(line);
  container.appendChild(div);
  if (logAutoScroll) container.scrollTop = container.scrollHeight;
}

function clearLogDisplay() {
  const container = $('logContainer');
  if (container) container.innerHTML = '';
  toast('Log display cleared', 'info');
}

async function toggleLogStream() {
  if (logStreamActive) {
    stopLogStream();
  } else {
    startLogStream();
  }
}

function stopLogStream() {
  if (logStreamReader) {
    logStreamReader.cancel();
    logStreamReader = null;
  }
  logStreamActive = false;
  const dot = $('logStreamDot');
  const text = $('logStreamText');
  const btn = $('logToggleBtn');
  if (dot) dot.className = 'sse-dot disconnected';
  if (text) text.textContent = 'Paused';
  if (btn) { btn.textContent = 'Stream'; btn.className = 'btn btn-xs btn-primary'; }
  toast('Log stream stopped', 'info');
}

async function startLogStream() {
  if (logStreamReader) return;
  logStreamActive = true;
  const dot = $('logStreamDot');
  const text = $('logStreamText');
  const btn = $('logToggleBtn');
  if (dot) dot.className = 'sse-dot connecting';
  if (text) text.textContent = 'Connecting...';
  if (btn) { btn.textContent = 'Stop'; btn.className = 'btn btn-xs btn-danger'; }

  try {
    const response = await fetch('/api/v1/logs/stream');
    if (!response.ok) throw new Error('Failed to connect to log stream');
    if (dot) dot.className = 'sse-dot connected';
    if (text) text.textContent = 'Streaming';

    const reader = response.body.getReader();
    logStreamReader = reader;
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        if (line.trim()) appendLogLine(line);
      }
    }
  } catch (e) {
    if (logStreamActive) {
      if (dot) dot.className = 'sse-dot disconnected';
      if (text) text.textContent = 'Disconnected';
      toast('Log stream disconnected: ' + e.message, 'warning');
    }
  } finally {
    logStreamActive = false;
    logStreamReader = null;
  }
}

async function exportLogs() {
  try {
    const blob = await API._fetch('/api/v1/logs/export');
    const text = await blob.text();
    const a = document.createElement('a');
    a.href = 'data:text/plain;charset=utf-8,' + encodeURIComponent(text);
    a.download = 'tmd-logs-' + new Date().toISOString().slice(0, 10) + '.log';
    a.click();
    toast('Logs exported', 'success');
  } catch (e) { toast(e.message, 'error'); }
}

/* ---- SSE ---- */
let sseSource = null;
let sseReconnectTimer = null;

function connectSSE() {
  if (sseSource) { sseSource.close(); sseSource = null; }
  const url = '/api/v1/sse/tasks';
  sseSource = new EventSource(url);

  sseSource.onopen = () => {
    state.sseConnected = true;
    updateSSEIndicator(true);
    if (sseReconnectTimer) { clearTimeout(sseReconnectTimer); sseReconnectTimer = null; }
  };

  sseSource.addEventListener('tasks', (e) => {
    try {
      const data = JSON.parse(e.data);
      if (data.tasks) {
        state.tasks = data.tasks;
        updateTaskBadge();
        if (state.currentPage === 'tasks') {
          const cardBody = document.querySelector('.card .task-list');
          if (cardBody) {
            // Live update: only refresh if on tasks page to avoid flash
            renderTasks();
          }
        }
        if (state.currentPage === 'dashboard') {
          const dashSection = document.querySelector('.dashboard-grid');
          if (dashSection) {
            // Refresh just the stats on dashboard
            API.get('/api/v1/tasks/stats').then(stats => {
              if (stats) state.taskStats = stats;
              updateTaskBadge();
            }).catch(() => {});
          }
        }
      }
    } catch (err) { /* ignore parse errors */ }
  });

  sseSource.addEventListener('schedules', (e) => {
    try {
      const data = JSON.parse(e.data);
      if (data.entries) {
        state.schedules = data.entries;
        state.schedulerRunning = data.scheduler_running;
        if (state.currentPage === 'schedules') renderSchedules();
      }
    } catch (err) { /* ignore */ }
  });

  sseSource.addEventListener('notification', (e) => {
    try {
      const data = JSON.parse(e.data);
      if (data.message) toast(data.message, data.level || 'info');
    } catch (err) { /* ignore */ }
  });

  sseSource.addEventListener('server_shutdown', (e) => {
    toast('Server is shutting down...', 'error');
  });

  sseSource.onerror = () => {
    state.sseConnected = false;
    updateSSEIndicator(false);
    sseSource.close();
    sseSource = null;
    // Reconnect after 5s
    if (!sseReconnectTimer) {
      sseReconnectTimer = setTimeout(() => {
        sseReconnectTimer = null;
        connectSSE();
      }, 5000);
    }
  };
}

function updateSSEIndicator(connected) {
  const dot = $('sseDot');
  const dashStatus = $('dashSseStatus');
  if (dot) dot.className = 'sse-dot ' + (connected ? 'connected' : 'disconnected');
  if (dashStatus) {
    dashStatus.innerHTML = `<span class="tag ${connected ? 'tag-success' : 'tag-failed'}">${connected ? 'Connected' : 'Disconnected'}</span>`;
  }
}

function updateSidebarHealth() {
  const dot = $('healthDot');
  const text = $('healthText');
  if (!dot || !text) return;
  if (state.health && state.health.status === 'ok') {
    dot.className = 'health-dot ok';
    text.textContent = 'Online';
  } else {
    dot.className = 'health-dot down';
    text.textContent = 'Offline';
  }
}

function updateTaskBadge() {
  const badge = $('taskBadge');
  if (!badge) return;
  const running = state.taskStats?.running || 0;
  const queued = state.taskStats?.queued || 0;
  const total = running + queued;
  if (total > 0) {
    badge.textContent = total;
    badge.style.display = '';
  } else {
    badge.style.display = 'none';
  }
}

/* ---- Health Check ---- */
async function checkHealth() {
  try {
    const health = await API.get('/api/v1/health');
    state.health = health;
  } catch (e) {
    state.health = null;
  }
  updateSidebarHealth();
}

/* ---- Popstate (browser back/forward) ---- */
window.addEventListener('popstate', () => {
  const path = location.pathname.replace(/^\//, '') || 'dashboard';
  const page = PAGES.includes(path) ? path : 'dashboard';
  state.currentPage = page;
  updateNavigation();
  renderPage(page);
});

/* ---- Init ---- */
async function init() {
  // Check health
  await checkHealth();

  // Start SSE
  connectSSE();

  // Determine initial page from URL
  const path = location.pathname.replace(/^\//, '') || 'dashboard';
  const page = PAGES.includes(path) ? path : 'dashboard';
  state.currentPage = page;
  updateNavigation();

  // Load page content
  renderPage(page);

  // Periodic health check
  setInterval(checkHealth, 30000);
}

document.addEventListener('DOMContentLoaded', init);
