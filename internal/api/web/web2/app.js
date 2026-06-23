/* ============================================================
   TMD Web UI - app.js
   SPA client for Twitter Media Downloader API
   ============================================================ */

/* ---- API Client ---- */
const API_BASE = '';
const API_TIMEOUT = 30000; // 30s timeout for all API requests

function apiBase() { return API_BASE; }

const API = {
  async _fetch(url, options) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), API_TIMEOUT);
    try {
      const r = await fetch(url, { ...options, signal: controller.signal });
      return r;
    } catch(e) {
      if (e.name === 'AbortError') throw new Error('Request timed out');
      throw e;
    } finally {
      clearTimeout(timer);
    }
  },
  get: async (url) => {
    const r = await API._fetch(apiBase() + url);
    const j = await r.json();
    if (!j.success) throw new Error(j.error || 'Request failed');
    return j.data;
  },
  post: async (url, body) => {
    const r = await API._fetch(apiBase() + url, { method: 'POST', headers: {'Content-Type':'application/json'}, body: body ? JSON.stringify(body) : undefined });
    const j = await r.json();
    if (!j.success) throw new Error(j.error || 'Request failed');
    return j.data;
  },
  put: async (url, body) => {
    const r = await API._fetch(apiBase() + url, { method: 'PUT', headers: {'Content-Type':'application/json'}, body: body ? JSON.stringify(body) : undefined });
    const j = await r.json();
    if (!j.success) throw new Error(j.error || 'Request failed');
    return j.data;
  },
  patch: async (url, body) => {
    const r = await API._fetch(apiBase() + url, { method: 'PATCH', headers: {'Content-Type':'application/json'}, body: body ? JSON.stringify(body) : undefined });
    const j = await r.json();
    if (!j.success) throw new Error(j.error || 'Request failed');
    return j.data;
  },
  del: async (url) => {
    const r = await API._fetch(apiBase() + url, { method: 'DELETE' });
    const j = await r.json();
    if (!j.success) throw new Error(j.error || 'Request failed');
    return j.data;
  }
};

/* ---- Utility ---- */
const esc = (s) => { if (s == null) return ''; const d = document.createElement('div'); d.appendChild(document.createTextNode(String(s))); return d.innerHTML; };
const jsEsc = (s) => { if (s == null) return ''; return String(s).replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/\\/g,'\\\\').replace(/'/g,"\\'").replace(/\n/g,'\\n').replace(/\r/g,''); };

// Log helpers
function stripAnsi(str) { return str.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, ''); }

// 提取日志行末尾的推文 ID（行首必须是 [...] 格式）
function getTweetId(text) {
  if (!text.startsWith('[')) return null;
  const m = text.match(/_(\d{16,20})\s*$/);
  return m ? m[1] : null;
}

function getLogLineColor(line) {
  if (line.startsWith('ERRO[') || line.includes('level=error')) return 'var(--red)';
  if (line.startsWith('WARN[') || line.includes('level=warn') || line.includes('level=warning')) return 'var(--amber)';
  if (line.startsWith('INFO[') || line.includes('level=info')) return 'var(--blue)';
  if (line.startsWith('DEBU[') || line.includes('level=debug')) return 'var(--text-muted)';
  return 'var(--text-secondary)';
}
function highlightLogTimestamp(line) {
  line = line.replace(/time=(&quot;)(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[-+]\d{2}:\d{2})(&quot;)/g, 'time=<span class="log-timestamp">$2</span>');
  line = line.replace(/(ERRO|WARN|INFO|DEBU)\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})\]/g, '$1[<span class="log-timestamp">$2</span>]');
  return line;
}

const relativeTime = (iso) => {
  if (!iso) return '-';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '-';
  const diff = Date.now() - d.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return mins + 'm ago';
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return hrs + 'h ' + (mins % 60) + 'm ago';
  return new Date(iso).toLocaleDateString();
};

const formatTime = (iso) => {
  if (!iso) return '-';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '-';
  return d.toLocaleString();
};

const formatDuration = (start, end) => {
  if (!start || !end) return '-';
  const s = new Date(start), e = new Date(end);
  if (isNaN(s.getTime()) || isNaN(e.getTime())) return '-';
  const d = e - s;
  if (d < 1000) return d + 'ms';
  if (d < 60000) return (d / 1000).toFixed(1) + 's';
  const m = Math.floor(d / 60000);
  const secs = Math.floor((d % 60000) / 1000);
  return m + 'm ' + secs + 's';
};

function getTaskProgressPercent(task) {
  if (task.status === 'completed') return 100;
  const p = task.progress || {};
  const total = p.total || 0;
  const completed = p.completed || 0;
  const ratio = total > 0 ? Math.min(completed / total, 1) : 0;
  if (task.status === 'failed' || task.status === 'cancelled') return total > 0 ? Math.round(ratio * 100) : 0;
  switch (p.stage) {
    case 'syncing': return 5;
    case 'preparing': return 10;
    case 'downloading': return Math.round(10 + ratio * 70);
    case 'retrying': return Math.round(80 + ratio * 10);
    case 'profile': return total > 0 ? Math.round(90 + ratio * 9) : 90;
    case 'profile_warning': return 99;
    case 'marking': return total > 0 ? Math.round(10 + ratio * 85) : 10;
    default: return 0;
  }
}

function getStageText(stage) {
  const m = { preparing:'Preparing', syncing:'Syncing', downloading:'Downloading', retrying:'Retrying', profile:'Profile', profile_warning:'Profile Warning', marking:'Marking', completed:'' };
  return m[stage] ? ' · ' + m[stage] : (stage ? ' · ' + stage : '');
}

function getTaskTarget(task) {
  const d = task.data || {};
  if (d.screen_name) return '@' + d.screen_name;
  if (d.list_id) return 'List ' + d.list_id;
  const parts = [];
  if (Array.isArray(d.users) && d.users.length) parts.push(d.users.length + ' users');
  if (Array.isArray(d.lists) && d.lists.length) parts.push(d.lists.length + ' lists');
  if (Array.isArray(d.following_names) && d.following_names.length) parts.push(d.following_names.length + ' following');
  return parts.length ? parts.join(' · ') : '';
}

function taskTypeName(type) {
  const names = {
    user_download:'User Download', list_download:'List Download',
    following_download:'Following Download', profile_download:'Profile Download',
    mark_downloaded:'Mark Downloaded', json_file_download:'JSON File Download',
    json_folder_download:'Folder Download', batch_download:'Batch Download',
    list_profile:'List Profile', retry_all_failed:'Retry All Failed'
  };
  return names[type] || type;
}

function taskTypeIcon(type) {
  const icons = {
    user_download: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>',
    list_download: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>',
    following_download: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>',
    profile_download: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/><path d="M12 3l-4 4h3v5h2V7h3z"/></svg>',
    mark_downloaded: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>',
    json_file_download: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M9 15l3-3 3 3"/><path d="M12 12v6"/></svg>',
    json_folder_download: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><path d="M9 15l3-3 3 3"/><path d="M12 12v6"/></svg>',
    batch_download: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8"/><path d="M12 17v4"/></svg>',
    list_profile: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>',
    retry_all_failed: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></svg>'
  };
  return icons[type] || '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/></svg>';
}

/* ---- Toast ---- */
let toastId = 0;
function toast(msg, type) {
  type = type || 'info';
  const container = document.getElementById('toast-container');
  if (!container) return;

  // Dedup: skip if same message already visible
  for (const el of container.children) {
    const msgEl = el.querySelector('.toast-msg');
    if (msgEl && msgEl.textContent === msg) return;
  }

  // Max 3 concurrent toasts, remove oldest
  while (container.children.length >= 3) container.firstChild.remove();

  const id = ++toastId;
  const el = document.createElement('div');
  el.className = 'toast toast-' + type;
  el.id = 'toast-' + id;
  const icons = { success:'✓', error:'✕', warning:'!', info:'i' };
  el.innerHTML = '<span class="toast-icon">' + icons[type] + '</span><span class="toast-msg">' + esc(msg) + '</span><button class="toast-close" onclick="dismissToast(' + id + ')">✕</button>';
  el.querySelector('.toast-close').onclick = () => el.remove();
  container.appendChild(el);
  setTimeout(() => { const e = document.getElementById('toast-'+id); if (e) e.remove(); }, 5000);
}
function dismissToast(id) {
  const el = document.getElementById('toast-'+id);
  if (el) el.remove();
}

/* ---- Modal ---- */
let currentModal = null;
function openModal(html) {
  closeModal();
  const overlay = document.createElement('div');
  overlay.className = 'modal-overlay';
  overlay.innerHTML = '<div class="modal">' + html + '</div>';
  overlay.addEventListener('click', (e) => { if (e.target === overlay) closeModal(); });
  document.body.appendChild(overlay);
  currentModal = overlay;
}
function closeModal() {
  if (currentModal) { currentModal.remove(); currentModal = null; }
}

/* ---- Global State ---- */
let pageTasks = [];
let sseConnected = false;
let pageRenderers = {};
let currentPage = 'tasks';

// Debounce utility to batch rapid updates
function debounce(fn, delay) {
  let timer = null;
  return function(...args) {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => { timer = null; fn.apply(this, args); }, delay);
  };
}

/* ---- API Endpoint Mappings ---- */
const ENDPOINTS = {
  // Health
  health:      () => API.get('/api/v1/health'),
  queueStatus: () => API.get('/api/v1/queue/status'),

  // Tasks
  tasks:       () => API.get('/api/v1/tasks'),
  taskStats:   () => API.get('/api/v1/tasks/stats'),
  getTask:     (id) => API.get('/api/v1/tasks/' + encodeURIComponent(id)),
  cancelTask:  (id) => API.post('/api/v1/tasks/' + encodeURIComponent(id) + '/cancel'),
  cancelQueued:() => API.post('/api/v1/tasks/cancel-queued'),
  retryTask:   (id) => API.post('/api/v1/tasks/' + encodeURIComponent(id) + '/retry'),
  deleteTask:  (id) => API.del('/api/v1/tasks/' + encodeURIComponent(id)),

  // Downloads
  userDownload:        (sn, opts) => API.post('/api/v1/users/' + encodeURIComponent(sn) + '/download', opts),
  userProfile:         (sn) => API.post('/api/v1/users/' + encodeURIComponent(sn) + '/profile', {}),
  userMark:            (sn, ts) => API.post('/api/v1/users/' + encodeURIComponent(sn) + '/mark', ts ? {timestamp:ts} : {}),
  userFollowingDL:     (sn, opts) => API.post('/api/v1/users/' + encodeURIComponent(sn) + '/following/download', opts),
  userFollowingMark:   (sn, ts) => API.post('/api/v1/users/' + encodeURIComponent(sn) + '/following/mark', ts ? {timestamp:ts} : {}),
  listDownload:        (id, opts) => API.post('/api/v1/lists/' + encodeURIComponent(id) + '/download', opts),
  listProfile:         (id) => API.post('/api/v1/lists/' + encodeURIComponent(id) + '/profile', {}),
  listMark:            (id, ts) => API.post('/api/v1/lists/' + encodeURIComponent(id) + '/mark', ts ? {timestamp:ts} : {}),
  batchDownload:       (data) => API.post('/api/v1/batch/download', data),
  batchMark:           (data) => API.post('/api/v1/batch/mark', data),

  // JSON
  jsonFileDownload:    (data) => API.post('/api/v1/json/file/download', data),
  jsonFolderDownload:  (data) => API.post('/api/v1/json/folder/download', data),

  // Errors
  errors:      () => API.get('/api/v1/errors'),
  retryErrors: () => API.post('/api/v1/errors/retry'),
  clearErrors: () => API.del('/api/v1/errors'),

  // DB
  dbUsers:             (p) => API.get('/api/v1/db/users' + qs(p)),
  dbUser:              (id) => API.get('/api/v1/db/users/' + encodeURIComponent(id)),
  dbUserUpdate:        (id,b) => API.patch('/api/v1/db/users/' + encodeURIComponent(id), b),
  dbUserDelete:        (id) => API.del('/api/v1/db/users/' + encodeURIComponent(id)),
  dbUserPrevNames:     (id) => API.get('/api/v1/db/users/' + encodeURIComponent(id) + '/previous-names'),
  dbUserEntities:      (id) => API.get('/api/v1/db/users/' + encodeURIComponent(id) + '/entities'),
  dbUserLinks:         (id) => API.get('/api/v1/db/users/' + encodeURIComponent(id) + '/links'),
  dbLists:             (p) => API.get('/api/v1/db/lists' + qs(p)),
  dbList:              (id) => API.get('/api/v1/db/lists/' + encodeURIComponent(id)),
  dbListUpdate:        (id,b) => API.patch('/api/v1/db/lists/' + encodeURIComponent(id), b),
  dbListDelete:        (id) => API.del('/api/v1/db/lists/' + encodeURIComponent(id)),
  dbListEntities:      (id) => API.get('/api/v1/db/lists/' + encodeURIComponent(id) + '/entities'),
  dbUserEntitiesAll:   (p) => API.get('/api/v1/db/user-entities' + qs(p)),
  dbListEntitiesAll:   (p) => API.get('/api/v1/db/list-entities' + qs(p)),
  dbUserLinksAll:      (p) => API.get('/api/v1/db/user-links' + qs(p)),
  dbPrevNamesAll:      (p) => API.get('/api/v1/db/user-previous-names' + qs(p)),
  dbStats:             () => API.get('/api/v1/db/stats'),

  // Config
  config:        () => API.get('/api/v1/config'),
  configRaw:     () => API.get('/api/v1/config/raw'),
  configFields:  () => API.get('/api/v1/config/fields'),
  saveConfigRaw: (c) => API.put('/api/v1/config/raw', {content:c}),
  saveConfigFields: (f) => API.put('/api/v1/config/fields', {fields:f}),

  // Cookies
  cookies:       () => API.get('/api/v1/cookies'),
  cookiesRaw:    () => API.get('/api/v1/cookies/raw'),
  saveCookies:   (c) => API.put('/api/v1/cookies', {cookies:c}),
  saveCookiesRaw:(c) => API.put('/api/v1/cookies/raw', {content:c}),

  // Schedules
  schedules:       () => API.get('/api/v1/schedules'),
  schedulesRaw:    () => API.get('/api/v1/schedules/raw'),
  scheduleStats:   () => API.get('/api/v1/schedules/stats'),
  createSchedule:  (e) => API.post('/api/v1/schedules', e),
  saveSchedulesRaw:(c) => API.put('/api/v1/schedules/raw', {content:c}),
  replaceSchedules:(e) => API.put('/api/v1/schedules', {entries:e}),
  reloadSchedules: () => API.post('/api/v1/schedules/reload'),
  validateSchedule:(b) => API.post('/api/v1/schedules/validate', b),
  triggerAll:      () => API.post('/api/v1/schedules/trigger-all'),
  updateSchedule:  (id,e) => API.put('/api/v1/schedules/' + encodeURIComponent(id), e),
  deleteSchedule:  (id) => API.del('/api/v1/schedules/' + encodeURIComponent(id)),
  setScheduleEnabled: (id,e) => API.patch('/api/v1/schedules/' + encodeURIComponent(id) + '/enabled', {enabled:e}),
  triggerSchedule: (id) => API.post('/api/v1/schedules/' + encodeURIComponent(id) + '/trigger'),

  // Logs
  logs:      (p) => API.get('/api/v1/logs' + qs(p)),
  logStats:  () => API.get('/api/v1/logs/stats'),

  // Server
  shutdown:  () => API.post('/api/v1/server/shutdown'),
};

function qs(params) {
  if (!params) return '';
  const filtered = {};
  for (const k of Object.keys(params)) if (params[k] !== undefined && params[k] !== null && params[k] !== '') filtered[k] = params[k];
  const keys = Object.keys(filtered);
  if (!keys.length) return '';
  return '?' + keys.map(k => encodeURIComponent(k) + '=' + encodeURIComponent(filtered[k])).join('&');
}

/* ---- Routing ---- */
function navigateTo(page) {
  closeModal();
  // Clean up log SSE when leaving logs page
  if (currentPage === 'logs' && page !== 'logs') {
    disconnectLogSSE();
  }
  if (page === currentPage) return;
  currentPage = page;
  history.pushState({page}, '', page === 'tasks' ? '/' : '/' + page);
  renderPage(page);
}

window.addEventListener('popstate', (e) => {
  const page = location.pathname.replace(/^\//, '') || 'tasks';
  currentPage = page;
  renderPage(page);
});

function renderPage(page) {
  // Update sidebar
  document.querySelectorAll('.nav-item').forEach(el => el.classList.toggle('active', el.dataset.page === page));
  const titles = {tasks:'Tasks', data:'Data', schedules:'Schedules', system:'System', logs:'Logs'};
  document.getElementById('page-title').textContent = titles[page] || 'Tasks';

  const container = document.getElementById('page-content');
  if (pageRenderers[page]) {
    pageRenderers[page](container);
  } else {
    container.innerHTML = '<div class="loading"><div class="spinner"></div> Loading...</div>';
    loadPageModule(page, container);
  }
}

function loadPageModule(page, container) {
  const loaders = {
    tasks:     renderTasksPage,
    data:      renderDataPage,
    schedules: renderSchedulesPage,
    system:    renderSystemPage,
    logs:      renderLogsPage,
  };
  if (loaders[page]) {
    loaders[page](container);
    pageRenderers[page] = loaders[page];
  }
}

/* ---- SSE ---- */
let sseSource = null;
let sseReconnectTimer = null;
let sseReconnectDelay = 1000;

// Debounce rapid SSE updates to avoid excessive re-renders
const debouncedTasksUpdate = debounce(function(tasks) {
  pageTasks = tasks;
  if (currentPage === 'tasks' && pageRenderers.tasks) {
    try { updateTasksView(); } catch(err) { console.warn('SSE tasks update error:', err); }
  }
}, 100);
const debouncedSchedulesUpdate = debounce(function(data) {
  window._lastSchedulesData = data;
  if (currentPage === 'schedules' && pageRenderers.schedules) {
    try { updateSchedulesView(); } catch(err) { /* ignore */ }
  }
}, 100);

function connectSSE() {
  if (sseReconnectTimer) { clearTimeout(sseReconnectTimer); sseReconnectTimer = null; }
  if (sseSource) { sseSource.close(); sseSource = null; }
  sseSource = new EventSource(apiBase() + '/api/v1/sse/tasks');

  sseSource.addEventListener('tasks', (e) => {
    try {
      const tasks = JSON.parse(e.data);
      if (Array.isArray(tasks)) debouncedTasksUpdate(tasks);
    } catch(err) { /* ignore parse errors */ }
  });

  sseSource.addEventListener('schedules', (e) => {
    try {
      const data = JSON.parse(e.data);
      debouncedSchedulesUpdate(data);
    } catch(err) { /* ignore */ }
  });

  sseSource.addEventListener('notification', (e) => {
    try {
      const n = JSON.parse(e.data);
      if (n && n.message) {
        const type = n.type === 'task_completed' ? 'success' :
                     n.type === 'task_failed' ? 'error' :
                     n.type === 'task_cancelled' ? 'warning' :
                     n.type === 'schedule_warning' ? 'warning' : 'info';
        // Delay toast to align with SSE debounce
        setTimeout(() => toast(n.message, type), 100);
      }
    } catch(err) { /* ignore */ }
  });

  sseSource.addEventListener('server_shutdown', (e) => {
    toast('Server is shutting down...', 'error');
  });

  sseSource.onopen = () => {
    sseConnected = true;
    sseReconnectDelay = 1000;
    // Refresh current page data after reconnect
    if (currentPage === 'data') {
      const activeTab = document.querySelector('#db-tabs .tab.active');
      loadDBTab(activeTab ? activeTab.dataset.dbtab : 'users');
    }
    document.querySelector('.health-dot') && (document.querySelector('.health-dot').style.background = 'var(--green)');
  };

  sseSource.onerror = () => {
    sseConnected = false;
    document.querySelector('.health-dot') && (document.querySelector('.health-dot').style.background = 'var(--red)');
    sseSource.close();
    sseReconnectDelay = Math.min(sseReconnectDelay * 2, 30000);
    sseReconnectTimer = setTimeout(connectSSE, sseReconnectDelay);
  };
}

/* ---- Health Check ---- */
async function checkHealth() {
  try {
    const h = await ENDPOINTS.health();
    const dot = document.getElementById('health-dot');
    const text = document.getElementById('health-text');
    if (dot) dot.className = 'health-dot';
    if (text) text.textContent = h.status || 'OK';
    const vi = document.getElementById('version-info');
    if (vi) vi.innerHTML = '<a href="https://github.com/leeexx2001/tmd" target="_blank" rel="noopener" style="color:inherit;text-decoration:none">' + esc(h.version || 'v2') + ' &middot; Go + SQLite</a>';
  } catch(e) {
    const dot = document.getElementById('health-dot');
    if (dot) dot.className = 'health-dot error';
    document.getElementById('health-text').textContent = 'Offline';
  }
}

/* ============================================================
   PAGE RENDERERS
   ============================================================ */

/* ---- Tasks Page ---- */
function renderTasksPage(container) {
  container.innerHTML = `
    <div class="stats-grid" id="task-stats"></div>
    <div class="card mb-4">
      <div class="card-body" style="padding:12px 20px">
        <div class="flex gap-2 items-center" style="flex-wrap:wrap">
          <input type="text" id="quick-dl-input" placeholder="Twitter URL or @username ... paste link or type name" style="flex:1;min-width:200px">
          <button class="btn btn-primary btn-sm" onclick="handleQuickDownload()">Quick Download</button>
        </div>
      </div>
    </div>
    <div class="section">
      <div class="section-header">
        <h2>Tasks</h2>
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-sm" onclick="showBatchForm()">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            New Download
          </button>
          <button class="btn btn-ghost btn-sm" onclick="cancelAllQueued()">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            Cancel Queued
          </button>
        </div>
      </div>
      <div class="card">
        <div class="table-wrap">
          <table class="task-table">
            <colgroup>
              <col style="width:48px">
              <col>
              <col style="width:100px">
              <col style="width:220px">
              <col style="width:130px">
              <col style="width:100px">
            </colgroup>
            <thead>
              <tr>
                <th style="width:32px">Type</th>
                <th>ID</th>
                <th style="width:100px">Status</th>
                <th style="width:220px">Progress</th>
                <th style="width:130px">Time</th>
                <th style="width:100px">Actions</th>
              </tr>
            </thead>
            <tbody id="task-table-body"></tbody>
          </table>
        </div>
        <div class="card-body" id="task-empty" style="display:none">
          <div class="empty-state">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>
            <p>No tasks yet. Start a download to see tasks here.</p>
          </div>
        </div>
      </div>
    </div>`;

  pageRenderers.tasks = renderTasksPage;
  updateTasksView();
}

function updateTasksView() {
  const tasks = pageTasks;
  // Stats
  const stats = {queued:0, running:0, completed:0, failed:0, cancelled:0, total:tasks.length};
  tasks.forEach(t => { if (stats[t.status] !== undefined) stats[t.status]++; });

  const statsHtml = Object.entries(stats).map(([k,v]) =>
    `<div class="stat-card ${k}"><div class="stat-value">${v}</div><div class="stat-label">${k}</div></div>`
  ).join('');
  const statsEl = document.getElementById('task-stats');
  if (statsEl) statsEl.innerHTML = statsHtml;

  // Table
  const tbody = document.getElementById('task-table-body');
  const empty = document.getElementById('task-empty');
  if (!tbody) return;

  if (!tasks.length) {
    tbody.innerHTML = '';
    if (empty) empty.style.display = '';
    return;
  }
  if (empty) empty.style.display = 'none';

  tbody.innerHTML = tasks.map(t => {
    const p = t.progress || {};
    const r = t.result || {};
    const pct = getTaskProgressPercent(t);
    const barClass = p.stage === 'completed' ? 'completed' : (t.status === 'failed' ? 'failed' : (t.status === 'running' ? 'pulsing' : ''));
    const stageText = getStageText(p.stage);
    const target = getTaskTarget(t);
    const canCancel = t.status === 'queued' || t.status === 'running';
    const canRetry = t.status === 'failed' || t.status === 'cancelled';
    const canDelete = t.status === 'completed' || t.status === 'failed' || t.status === 'cancelled';

    return `<tr data-status="${t.status}">
      <td>${taskTypeIcon(t.type)}</td>
      <td><span class="mono">${esc(t.task_id || t.id)}</span><div class="text-sm text-muted">${taskTypeName(t.type)}${target ? ' - ' + esc(target) : ''}</div></td>
      <td><span class="badge badge-${t.status}">${t.status}</span></td>
      <td>
        <div class="progress-bar-wrap"><div class="progress-bar-fill ${barClass}" style="width:${pct}%"></div></div>
        <div class="progress-detail">
          <span>${pct}%${stageText}</span>
          ${p.current ? '<span> &middot; ' + esc(p.current) + '</span>' : ''}
          ${p.failed ? '<span class="fail"> &middot; ' + esc(p.failed) + ' failed</span>' : ''}
        </div>
      </td>
      <td>
        <div class="text-sm">${relativeTime(t.created_at)}</div>
        ${t.started_at && t.ended_at ? '<div class="text-sm text-muted">' + formatDuration(t.started_at, t.ended_at) + '</div>' : ''}
      </td>
      <td>
        <div class="task-actions">
          ${(t.status === 'completed' || t.status === 'failed' || t.status === 'cancelled') ? '<button class="btn btn-xs btn-ghost" onclick="showTaskDetail(\'' + jsEsc(t.task_id) + '\')" title="View"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg></button>' : ''}
          ${canCancel ? '<button class="btn btn-xs btn-ghost" onclick="doCancelTask(\'' + jsEsc(t.task_id) + '\')" title="Cancel"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>' : ''}
          ${canRetry ? '<button class="btn btn-xs btn-ghost" onclick="doRetryTask(\'' + jsEsc(t.task_id) + '\')" title="Retry"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></svg></button>' : ''}
          ${canDelete ? '<button class="btn btn-xs btn-ghost" onclick="doDeleteTask(\'' + jsEsc(t.task_id) + '\')" title="Delete"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>' : ''}
        </div>
      </td>
    </tr>`;
  }).join('');
}

/* ---- Download Forms ---- */

function showBatchForm() {
  openModal(`
    <div class="modal-header">
      <h2>New Download</h2>
      <button class="btn btn-ghost btn-sm" onclick="closeModal()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
    </div>
    <div class="modal-body">
      <div class="tabs" id="dl-tabs">
        <button class="tab active" data-dltab="user">User</button>
        <button class="tab" data-dltab="list">List</button>
        <button class="tab" data-dltab="following">Following</button>
        <button class="tab" data-dltab="batch">Batch</button>
        <button class="tab" data-dltab="json">JSON</button>
      </div>

      <!-- User Download -->
      <div class="dl-tab-content" id="dl-tab-user">
        <div class="form-group">
          <label>Screen Name</label>
          <input type="text" id="dl-user-name" placeholder="elonmusk">
        </div>
        <div class="form-row">
          <label class="checkbox-label"><input type="checkbox" id="dl-user-autofollow"> Auto-follow</label>
          <label class="checkbox-label"><input type="checkbox" id="dl-user-noretry"> No retry</label>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" onclick="doUserDownload()">Download User</button>
          <button class="btn btn-secondary" onclick="doUserProfile()">Profile Only</button>
          <button class="btn btn-secondary" onclick="doUserMark()">Mark Downloaded</button>
        </div>
      </div>

      <!-- List Download -->
      <div class="dl-tab-content hidden" id="dl-tab-list">
        <div class="form-group">
          <label>List ID</label>
          <input type="text" id="dl-list-id" placeholder="1234567890">
        </div>
        <div class="form-row">
          <label class="checkbox-label"><input type="checkbox" id="dl-list-autofollow"> Auto-follow</label>
          <label class="checkbox-label"><input type="checkbox" id="dl-list-noretry"> No retry</label>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" onclick="doListDownload()">Download List</button>
          <button class="btn btn-secondary" onclick="doListProfile()">Profile Only</button>
          <button class="btn btn-secondary" onclick="doListMark()">Mark Downloaded</button>
        </div>
      </div>

      <!-- Following Download -->
      <div class="dl-tab-content hidden" id="dl-tab-following">
        <div class="form-group">
          <label>User's Followings</label>
          <input type="text" id="dl-foll-name" placeholder="elonmusk">
        </div>
        <div class="form-row">
          <label class="checkbox-label"><input type="checkbox" id="dl-foll-autofollow"> Auto-follow</label>
          <label class="checkbox-label"><input type="checkbox" id="dl-foll-noretry"> No retry</label>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" onclick="doFollowingDownload()">Download</button>
          <button class="btn btn-secondary" onclick="doFollowingMark()">Mark Downloaded</button>
        </div>
      </div>

      <!-- Batch Download -->
      <div class="dl-tab-content hidden" id="dl-tab-batch">
        <div class="form-group">
          <label>Users (one per line)</label>
          <textarea id="dl-batch-users" rows="3" placeholder="elonmusk"></textarea>
        </div>
        <div class="form-group">
          <label>List IDs (one per line)</label>
          <textarea id="dl-batch-lists" rows="2" placeholder="1234567890"></textarea>
        </div>
        <div class="form-group">
          <label>Following (one per line)</label>
          <textarea id="dl-batch-foll" rows="2" placeholder="jack"></textarea>
        </div>
        <div class="form-row">
          <label class="checkbox-label"><input type="checkbox" id="dl-batch-autofollow"> Auto-follow</label>
          <label class="checkbox-label"><input type="checkbox" id="dl-batch-followmembers"> Follow members</label>
          <label class="checkbox-label"><input type="checkbox" id="dl-batch-noretry"> No retry</label>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" onclick="doBatchDownload()">Batch Download</button>
          <button class="btn btn-secondary" onclick="doBatchMark()">Batch Mark</button>
        </div>
      </div>

      <!-- JSON Download -->
      <div class="dl-tab-content hidden" id="dl-tab-json">
        <div class="form-group">
          <label>JSON File Paths (one per line)</label>
          <textarea id="dl-json-paths" rows="3" placeholder="/path/to/tweets.json"></textarea>
        </div>
        <label class="checkbox-label"><input type="checkbox" id="dl-json-noretry"> No retry</label>
        <div class="form-actions">
          <button class="btn btn-primary" onclick="doJSONFileDownload()">Download from Files</button>
          <button class="btn btn-secondary" onclick="doJSONFolderDownload()">Download from Folders</button>
        </div>
      </div>
    </div>`);

  // Tab switching
  document.querySelectorAll('[data-dltab]').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('[data-dltab]').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      document.querySelectorAll('.dl-tab-content').forEach(c => c.classList.add('hidden'));
      document.getElementById('dl-tab-' + tab.dataset.dltab).classList.remove('hidden');
    });
  });
}

// Download action functions
async function doUserDownload() {
  const name = document.getElementById('dl-user-name').value.trim();
  if (!name) return toast('Enter a screen name', 'warning');
  closeModal();
  try {
    const r = await ENDPOINTS.userDownload(name, {
      auto_follow: document.getElementById('dl-user-autofollow').checked,
      no_retry: document.getElementById('dl-user-noretry').checked
    });
    toast('Task created: ' + r.task_id, 'success');
    document.getElementById('dl-user-name').value = '';
  } catch(e) { toast(e.message, 'error'); }
}

async function doUserProfile() {
  const name = document.getElementById('dl-user-name').value.trim();
  if (!name) return toast('Enter a screen name', 'warning');
  closeModal();
  try { const r = await ENDPOINTS.userProfile(name); toast('Task created: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function doUserMark() {
  const name = document.getElementById('dl-user-name').value.trim();
  if (!name) return toast('Enter a screen name', 'warning');
  closeModal();
  try { const r = await ENDPOINTS.userMark(name); toast('Marked: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function doListDownload() {
  const id = document.getElementById('dl-list-id').value.trim();
  if (!id) return toast('Enter a list ID', 'warning');
  closeModal();
  try {
    const r = await ENDPOINTS.listDownload(id, {
      auto_follow: document.getElementById('dl-list-autofollow').checked,
      no_retry: document.getElementById('dl-list-noretry').checked
    });
    toast('Task created: ' + r.task_id, 'success');
  } catch(e) { toast(e.message, 'error'); }
}

async function doListProfile() {
  const id = document.getElementById('dl-list-id').value.trim();
  if (!id) return toast('Enter a list ID', 'warning');
  closeModal();
  try { const r = await ENDPOINTS.listProfile(id); toast('Task created: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function doListMark() {
  const id = document.getElementById('dl-list-id').value.trim();
  if (!id) return toast('Enter a list ID', 'warning');
  closeModal();
  try { const r = await ENDPOINTS.listMark(id); toast('Marked: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function doFollowingDownload() {
  const name = document.getElementById('dl-foll-name').value.trim();
  if (!name) return toast('Enter a screen name', 'warning');
  closeModal();
  try {
    const r = await ENDPOINTS.userFollowingDL(name, {
      auto_follow: document.getElementById('dl-foll-autofollow').checked,
      no_retry: document.getElementById('dl-foll-noretry').checked
    });
    toast('Task created: ' + r.task_id, 'success');
  } catch(e) { toast(e.message, 'error'); }
}

async function doFollowingMark() {
  const name = document.getElementById('dl-foll-name').value.trim();
  if (!name) return toast('Enter a screen name', 'warning');
  closeModal();
  try { const r = await ENDPOINTS.userFollowingMark(name); toast('Marked: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function doBatchDownload() {
  const users = document.getElementById('dl-batch-users').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  const lists = document.getElementById('dl-batch-lists').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  const foll = document.getElementById('dl-batch-foll').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  if (!users.length && !lists.length && !foll.length) return toast('Enter at least one target', 'warning');
  closeModal();
  try {
    const r = await ENDPOINTS.batchDownload({
      users, lists, following_names: foll,
      auto_follow: document.getElementById('dl-batch-autofollow').checked,
      follow_members: document.getElementById('dl-batch-followmembers').checked,
      no_retry: document.getElementById('dl-batch-noretry').checked
    });
    toast('Batch task: ' + r.task_id, 'success');
  } catch(e) { toast(e.message, 'error'); }
}

async function doBatchMark() {
  const users = document.getElementById('dl-batch-users').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  const lists = document.getElementById('dl-batch-lists').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  const foll = document.getElementById('dl-batch-foll').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  if (!users.length && !lists.length && !foll.length) return toast('Enter at least one target', 'warning');
  closeModal();
  try {
    const r = await ENDPOINTS.batchMark({ users, lists, following_names: foll });
    toast('Batch mark: ' + r.task_id, 'success');
  } catch(e) { toast(e.message, 'error'); }
}

async function doJSONFileDownload() {
  const paths = document.getElementById('dl-json-paths').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  if (!paths.length) return toast('Enter at least one path', 'warning');
  closeModal();
  try {
    const r = await ENDPOINTS.jsonFileDownload({ paths, no_retry: document.getElementById('dl-json-noretry').checked });
    toast('Task created: ' + r.task_id, 'success');
  } catch(e) { toast(e.message, 'error'); }
}

async function doJSONFolderDownload() {
  const paths = document.getElementById('dl-json-paths').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
  if (!paths.length) return toast('Enter at least one path', 'warning');
  closeModal();
  try {
    const r = await ENDPOINTS.jsonFolderDownload({ paths, no_retry: document.getElementById('dl-json-noretry').checked });
    toast('Task created: ' + r.task_id, 'success');
  } catch(e) { toast(e.message, 'error'); }
}

// Task actions
async function doCancelTask(id) {
  if (!confirm('Cancel this task?')) return;
  try { await ENDPOINTS.cancelTask(id); toast('Task cancelled', 'info'); }
  catch(e) { toast(e.message, 'error'); }
}

async function doRetryTask(id) {
  try { const r = await ENDPOINTS.retryTask(id); toast('Retry created: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function doDeleteTask(id) {
  if (!confirm('Delete this task?')) return;
  try { await ENDPOINTS.deleteTask(id); toast('Task deleted', 'info'); }
  catch(e) { toast(e.message, 'error'); }
}

// Task detail view
async function showTaskDetail(id) {
  let task;
  try {
    task = await ENDPOINTS.getTask(id);
  } catch(e) {
    return toast('Failed to load task: ' + e.message, 'error');
  }
  if (!task) return toast('Task not found', 'error');

  const statusColors = { queued:'#8b949e', running:'#58a6ff', completed:'#3fb950', failed:'#f85149', cancelled:'#6e7681' };
  const bgColors = { queued:'rgba(139,148,158,0.1)', running:'rgba(88,166,255,0.1)', completed:'rgba(63,185,80,0.1)', failed:'rgba(248,81,73,0.1)', cancelled:'rgba(110,118,129,0.1)' };
  const sc = statusColors[task.status] || '#8b949e';
  const bg = bgColors[task.status] || 'rgba(139,148,158,0.1)';
  const target = getTaskTarget(task);
  const pct = getTaskProgressPercent(task);

  // Timeline
  const fmt = (t) => t ? new Date(t).toLocaleString() : '-';
  const started = task.started_at ? fmt(task.started_at) : null;
  const ended = task.ended_at ? fmt(task.ended_at) : null;
  const dur = (task.started_at && task.ended_at) ? formatDuration(task.started_at, task.ended_at) : null;

  // Result
  let resultHtml = '';
  const res = task.result;
  if (res) {
    const parts = [];
    if (res.main) parts.push('<div><strong>Main:</strong> ' + (res.main.downloaded||0) + ' downloaded' + (res.main.failed ? ', ' + res.main.failed + ' failed' : '') + '</div>');
    if (res.profile) parts.push('<div><strong>Profile:</strong> ' + (res.profile.downloaded||0) + ' downloaded' + (res.profile.failed ? ', ' + res.profile.failed + ' failed' : '') + (res.profile.versioned ? ', ' + res.profile.versioned + ' versioned' : '') + '</div>');
    if (res.message) parts.push('<div class="text-sm text-muted">' + esc(res.message) + '</div>');
    if (parts.length) resultHtml = '<div class="section-header mt-4"><h3>Result</h3></div><div class="card" style="padding:12px 16px">' + parts.join('') + '</div>';
  }

  openModal(`
    <div class="modal-header">
      <h2>Task Detail</h2>
      <button class="btn btn-ghost btn-sm" onclick="closeModal()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
    </div>
    <div class="modal-body">
      <div style="background:${bg};border-radius:var(--radius);padding:12px 16px;margin-bottom:16px">
        <div style="font-weight:600;font-size:15px">${esc(target || task.task_id)}</div>
        <div class="text-sm text-muted" style="margin-top:4px">${esc(task.task_id)}</div>
        <div style="margin-top:8px"><span class="badge badge-${task.status}">${task.status}</span> <span class="text-sm text-muted">${taskTypeName(task.type)}</span></div>
      </div>

      <div class="section-header"><h3>Progress</h3></div>
      <div class="progress-bar-wrap mb-2"><div class="progress-bar-fill" style="width:${pct}%"></div></div>
      <div class="progress-detail"><span>${pct}%${getStageText(task.progress?.stage)}</span></div>

      <div class="section-header mt-4"><h3>Timeline</h3></div>
      <div class="card" style="padding:12px 16px">
        <div class="text-sm">Created: ${fmt(task.created_at)}</div>
        ${started ? '<div class="text-sm">Started: ' + started + '</div>' : ''}
        ${ended ? '<div class="text-sm">Ended: ' + ended + '</div>' : ''}
        ${dur ? '<div class="text-sm">Duration: ' + dur + '</div>' : ''}
      </div>

      ${resultHtml}

      ${task.error ? '<div class="section-header mt-4"><h3 style="color:var(--red)">Error</h3></div><div class="card" style="padding:12px 16px;color:var(--red)">' + esc(task.error) + '</div>' : ''}
    </div>
    <div class="modal-footer">
      <button class="btn btn-ghost" onclick="closeModal()">Close</button>
      ${task.status === 'queued' || task.status === 'running' ? '<button class="btn btn-danger btn-sm" onclick="closeModal();doCancelTask(\'' + jsEsc(task.task_id) + '\')">Cancel</button>' : ''}
      ${task.status === 'failed' || task.status === 'cancelled' ? '<button class="btn btn-primary btn-sm" onclick="closeModal();doRetryTask(\'' + jsEsc(task.task_id) + '\')">Retry</button>' : ''}
      ${task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled' ? '<button class="btn btn-danger btn-sm" onclick="closeModal();doDeleteTask(\'' + jsEsc(task.task_id) + '\')">Delete</button>' : ''}
    </div>`);
}

async function cancelAllQueued() {
  try { const r = await ENDPOINTS.cancelQueued(); toast('Cancelled ' + (r.cancelled_count || 0) + ' tasks', 'info'); }
  catch(e) { toast(e.message, 'error'); }
}

// Quick download with Twitter URL parsing
async function handleQuickDownload() {
  const input = document.getElementById('quick-dl-input');
  if (!input) return;
  let value = input.value.trim();
  if (!value) return toast('Enter a Twitter username or URL', 'warning');

  // Parse list link: twitter.com/i/lists/123 or x.com/i/lists/123
  const listMatch = value.match(/https?:\/\/(?:twitter\.com|x\.com)\/i\/lists\/(\d+)/);
  if (listMatch) {
    try {
      await ENDPOINTS.listDownload(listMatch[1], { auto_follow: true });
      toast('List download task created', 'success');
      input.value = '';
    } catch(e) { toast(e.message, 'error'); }
    return;
  }

  // Parse user link: twitter.com/username or x.com/username
  const userMatch = value.match(/https?:\/\/(?:twitter\.com|x\.com)\/([^/\s?]+)/);
  if (userMatch) {
    const pathPart = userMatch[1];
    const reserved = ['i','search','status','home','explore','notifications','messages','settings','compose','bookmarks','lists'];
    if (!reserved.includes(pathPart.toLowerCase())) {
      value = pathPart;
    }
  }

  // Strip @ prefix
  if (value.startsWith('@')) value = value.slice(1);
  if (!value) return toast('Could not extract username from URL', 'warning');

  try {
    await ENDPOINTS.userDownload(value, { auto_follow: true });
    toast('Download task created for @' + value, 'success');
    input.value = '';
  } catch(e) { toast(e.message, 'error'); }
}

/* ---- Data Page ---- */
function renderDataPage(container) {
  container.innerHTML = `
    <div class="section">
      <div class="section-header">
        <h2>Database Browser</h2>
      </div>
      <div class="card">
        <div class="card-header">
          <div class="tabs" id="db-tabs">
            <button class="tab active" data-dbtab="users">Users</button>
            <button class="tab" data-dbtab="lists">Lists</button>
            <button class="tab" data-dbtab="entities">User Entities</button>
            <button class="tab" data-dbtab="list-entities">List Entities</button>
            <button class="tab" data-dbtab="links">Links</button>
            <button class="tab" data-dbtab="prevnames">Previous Names</button>
            <button class="tab" data-dbtab="stats">Stats</button>
          </div>
        </div>
        <div class="card-body" id="db-content">
          <div class="loading"><div class="spinner"></div> Loading...</div>
        </div>
      </div>
    </div>`;

  pageRenderers.data = renderDataPage;
  loadDBTab('users');

  // Tab switching for Data page
  const dbTabs = document.getElementById('db-tabs');
  if (dbTabs) {
    dbTabs.addEventListener('click', (e) => {
      const tab = e.target.closest('[data-dbtab]');
      if (!tab) return;
      document.querySelectorAll('[data-dbtab]').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      loadDBTab(tab.dataset.dbtab);
    });
  }
}

let dbPageState = { users:0, lists:0, entities:0, 'list-entities':0, links:0, prevnames:0 };
let dbSearchState = { users:'', lists:'', entities:'', 'list-entities':'', links:'', prevnames:'' };
const DB_PAGE_SIZE = 20;

async function loadDBTab(tab, page) {
  const content = document.getElementById('db-content');
  if (!content) return;
  if (page != null) dbPageState[tab] = page;
  else dbPageState[tab] = 0;
  const p = dbPageState[tab];
  const q = dbSearchState[tab] || '';

  content.innerHTML = '<div class="loading"><div class="spinner"></div> Loading...</div>';

  try {
    switch (tab) {
      case 'users': await renderDBUsers(content, p, q); break;
      case 'lists': await renderDBLists(content, p, q); break;
      case 'entities': await renderDBEntities(content, p, q, 'user-entities', 'user_entities'); break;
      case 'list-entities': await renderDBEntities(content, p, q, 'list-entities', 'list_entities'); break;
      case 'links': await renderDBEntities(content, p, q, 'user-links', 'user_links'); break;
      case 'prevnames': await renderDBPrevNames(content, p, q); break;
      case 'stats': await renderDBStats(content); break;
    }
  } catch(e) {
    content.innerHTML = '<div class="empty-state"><p>Error loading data: ' + esc(e.message) + '</p></div>';
  }
}

async function renderDBUsers(content, page, search) {
  const params = { page: page + 1, pageSize: DB_PAGE_SIZE };
  if (search) params.search = search;
  const r = await ENDPOINTS.dbUsers(params);
  const users = r.data || r || [];
  const total = r.total || users.length;
  const totalPages = r.totalPages || 1;

  content.innerHTML = `
    <div class="filter-bar">
      <input type="text" id="db-search-input" placeholder="Search screen name..." value="${esc(search)}">
      <button class="btn btn-primary btn-sm" onclick="dbSearch('users')">Search</button>
      <button class="btn btn-ghost btn-sm" onclick="dbSearchClear('users')">Clear</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead><tr><th>ID</th><th>Screen Name</th><th>Display Name</th><th>Protected</th><th>Friends</th><th>Accessible</th><th>Actions</th></tr></thead>
        <tbody>${users.map(u => `<tr>
          <td><span class="mono">${esc(u.id||'')}</span></td>
          <td>${esc(u.screen_name||'')}</td>
          <td>${esc(u.name||'')}</td>
          <td>${u.protected ? 'Yes' : 'No'}</td>
          <td>${u.friends_count||0}</td>
          <td>${u.is_accessible ? 'Yes' : 'No'}</td>
          <td><button class="btn btn-xs btn-ghost" onclick="viewUserDetail('${jsEsc(u.id)}')">View</button></td>
        </tr>`).join('')}</tbody>
      </table>
    </div>
    ${renderPagination(page, totalPages, total, 'users')}`;
}

async function renderDBLists(content, page, search) {
  const params = { page: page + 1, pageSize: DB_PAGE_SIZE };
  if (search) params.search = search;
  const r = await ENDPOINTS.dbLists(params);
  const lists = r.data || r || [];
  const total = r.total || lists.length;
  const totalPages = r.totalPages || 1;

  content.innerHTML = `
    <div class="filter-bar">
      <input type="text" id="db-search-input" placeholder="Search list name..." value="${esc(search)}">
      <button class="btn btn-primary btn-sm" onclick="dbSearch('lists')">Search</button>
      <button class="btn btn-ghost btn-sm" onclick="dbSearchClear('lists')">Clear</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead><tr><th>ID</th><th>Name</th><th>Owner ID</th><th>Actions</th></tr></thead>
        <tbody>${lists.map(l => `<tr>
          <td><span class="mono">${esc(l.id||'')}</span></td>
          <td>${esc(l.name||'')}</td>
          <td>${esc(l.owner_user_id||'')}</td>
          <td><button class="btn btn-xs btn-ghost" onclick="viewListDetail('${jsEsc(l.id)}')">View</button></td>
        </tr>`).join('')}</tbody>
      </table>
    </div>
    ${renderPagination(page, totalPages, total, 'lists')}`;
}

async function renderDBEntities(content, page, search, ep, label) {
  const params = { page: page + 1, pageSize: DB_PAGE_SIZE };
  if (search) params.search = search;
  const r = await ENDPOINTS['db' + ep.charAt(0).toUpperCase() + ep.slice(1).replace(/-([a-z])/g, (_,c) => c.toUpperCase()) + 'All'](params);
  const items = r.data || r || [];
  const total = r.total || items.length;
  const totalPages = r.totalPages || 1;

  const cols = label === 'user_entities'
    ? '<th>ID</th><th>User ID</th><th>Name</th><th>Parent Dir</th><th>Media</th><th>Latest Release</th>'
    : label === 'list_entities'
    ? '<th>ID</th><th>List ID</th><th>Name</th><th>Parent Dir</th><th>List Name</th>'
    : '<th>ID</th><th>User ID</th><th>Name</th><th>Parent Entity</th>';

  const rows = items.map(i => {
    if (label === 'user_entities')
      return `<tr><td><span class="mono">${esc(i.id||'')}</span></td><td>${esc(i.user_id||'')}</td><td>${esc(i.name||'')}</td><td>${esc(i.parent_dir||'')}</td><td>${i.media_count||0}</td><td class="text-sm">${esc(i.latest_release_time||'')}</td></tr>`;
    else if (label === 'list_entities')
      return `<tr><td><span class="mono">${esc(i.id||'')}</span></td><td>${esc(i.lst_id||'')}</td><td>${esc(i.name||'')}</td><td>${esc(i.parent_dir||'')}</td><td>${esc(i.list_name||'')}</td></tr>`;
    else
      return `<tr><td><span class="mono">${esc(i.id||'')}</span></td><td>${esc(i.user_id||'')}</td><td>${esc(i.name||'')}</td><td>${esc(i.parent_lst_entity_name||i.parent_lst_entity_id||'')}</td></tr>`;
  }).join('');

  content.innerHTML = `
    <div class="filter-bar">
      <input type="text" id="db-search-input" placeholder="Search..." value="${esc(search)}">
      <button class="btn btn-primary btn-sm" onclick="dbSearch('${ep}')">Search</button>
      <button class="btn btn-ghost btn-sm" onclick="dbSearchClear('${ep}')">Clear</button>
    </div>
    <div class="table-wrap">
      <table><thead><tr>${cols}</tr></thead><tbody>${rows || '<tr><td colspan="8"><div class="empty-state"><p>No records found</p></div></td></tr>'}</tbody></table>
    </div>
    ${renderPagination(page, totalPages, total, ep)}`;
}

async function renderDBPrevNames(content, page, search) {
  const params = { page: page + 1, pageSize: DB_PAGE_SIZE };
  if (search) params.search = search;
  const r = await ENDPOINTS.dbPrevNamesAll(params);
  const items = r.data || r || [];
  const total = r.total || items.length;
  const totalPages = r.totalPages || 1;

  content.innerHTML = `
    <div class="filter-bar">
      <input type="text" id="db-search-input" placeholder="Search..." value="${esc(search)}">
      <button class="btn btn-primary btn-sm" onclick="dbSearch('prevnames')">Search</button>
      <button class="btn btn-ghost btn-sm" onclick="dbSearchClear('prevnames')">Clear</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead><tr><th>ID</th><th>User ID</th><th>Screen Name</th><th>Name</th><th>Record Date</th><th>Current</th></tr></thead>
        <tbody>${items.map(i => `<tr><td><span class="mono">${esc(i.id||'')}</span></td><td>${esc(i.user_id||'')}</td><td>${esc(i.screen_name||'')}</td><td>${esc(i.name||'')}</td><td class="text-sm">${esc(i.record_date||'')}</td><td>${i.current_screen_name ? esc(i.current_screen_name) : '-'}</td></tr>`).join('')}</tbody>
      </table>
    </div>
    ${renderPagination(page, totalPages, total, 'prevnames')}`;
}

async function renderDBStats(content) {
  const s = await ENDPOINTS.dbStats();
  const items = [
    ['Total Users', s.users || 0],
    ['Total Lists', s.lsts || 0],
    ['User Entities', s.user_entities || 0],
    ['List Entities', s.lst_entities || 0],
    ['Links', s.user_links || 0],
    ['Previous Names', s.user_previous_names || 0],
  ];
  content.innerHTML = `<div class="stats-grid">${items.map(([k,v]) => `<div class="stat-card total"><div class="stat-value">${v}</div><div class="stat-label">${k}</div></div>`).join('')}</div>`;
}

function renderPagination(page, totalPages, total, tabId) {
  if (totalPages <= 1) return '';
  let html = '<div class="pagination">';
  html += `<button class="btn btn-xs btn-ghost" ${page === 0 ? 'disabled' : ''} onclick="loadDBTab('${tabId}', 0)">First</button>`;
  html += `<button class="btn btn-xs btn-ghost" ${page === 0 ? 'disabled' : ''} onclick="loadDBTab('${tabId}', ${page - 1})">Prev</button>`;
  // Page numbers with ellipsis
  const pages = [];
  if (totalPages <= 7) {
    for (let i = 0; i < totalPages; i++) pages.push(i);
  } else {
    if (page < 4) {
      pages.push(0, 1, 2, 3, -1, totalPages - 1);
    } else if (page > totalPages - 5) {
      pages.push(0, -1, totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1);
    } else {
      pages.push(0, -1, page - 1, page, page + 1, -1, totalPages - 1);
    }
  }
  pages.forEach(p => {
    if (p === -1) { html += '<span class="pagination-dots">...</span>'; return; }
    html += `<button class="btn btn-xs ${p === page ? 'btn-primary' : 'btn-ghost'}" onclick="loadDBTab('${tabId}', ${p})">${p + 1}</button>`;
  });
  html += `<button class="btn btn-xs btn-ghost" ${page >= totalPages - 1 ? 'disabled' : ''} onclick="loadDBTab('${tabId}', ${page + 1})">Next</button>`;
  html += `<button class="btn btn-xs btn-ghost" ${page >= totalPages - 1 ? 'disabled' : ''} onclick="loadDBTab('${tabId}', ${totalPages - 1})">Last</button>`;
  html += ` <span class="pagination-info">Page ${page + 1}/${totalPages} (${total})</span>`;
  html += '</div>';
  return html;
}

window.dbSearch = (tab) => {
  const input = document.getElementById('db-search-input');
  if (input) dbSearchState[tab] = input.value;
  loadDBTab(tab, 0);
};
window.dbSearchClear = (tab) => {
  dbSearchState[tab] = '';
  loadDBTab(tab, 0);
};

async function viewUserDetail(id) {
  try {
    const u = await ENDPOINTS.dbUser(id);
    if (!u) return toast('User not found', 'error');
    const prev = await ENDPOINTS.dbUserPrevNames(id);
    const ents = await ENDPOINTS.dbUserEntities(id);
    const links = await ENDPOINTS.dbUserLinks(id);

    openModal(`
      <div class="modal-header"><h2>User: ${esc(u.screen_name)}</h2><button class="btn btn-ghost btn-sm" onclick="closeModal()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div>
      <div class="modal-body">
        <div class="form-row">
          <div class="form-group"><label>ID</label><code>${esc(u.id)}</code></div>
          <div class="form-group"><label>Screen Name</label><code>${esc(u.screen_name)}</code></div>
        </div>
        <div class="form-row">
          <div class="form-group"><label>Name</label><code>${esc(u.name)}</code></div>
          <div class="form-group"><label>Protected</label><code>${u.protected ? 'Yes' : 'No'}</code></div>
        </div>
        ${prev.length ? `<div class="section-header mt-4"><h3>Previous Names (${prev.length})</h3></div>
        <table><thead><tr><th>Screen Name</th><th>Name</th><th>Date</th></tr></thead><tbody>${prev.map(p => `<tr><td>${esc(p.screen_name)}</td><td>${esc(p.name)}</td><td class="text-sm">${esc(p.record_date)}</td></tr>`).join('')}</tbody></table>` : ''}
        ${ents.length ? `<div class="section-header mt-4"><h3>Entities (${ents.length})</h3></div>
        <table><thead><tr><th>Name</th><th>Parent Dir</th><th>Media</th></tr></thead><tbody>${ents.map(e => `<tr><td>${esc(e.name)}</td><td>${esc(e.parent_dir)}</td><td>${e.media_count||0}</td></tr>`).join('')}</tbody></table>` : ''}
        ${links.length ? `<div class="section-header mt-4"><h3>Links (${links.length})</h3></div>
        <table><thead><tr><th>Name</th><th>Parent Entity</th></tr></thead><tbody>${links.map(l => `<tr><td>${esc(l.name)}</td><td>${esc(l.parent_lst_entity_name||l.parent_lst_entity_id||'-')}</td></tr>`).join('')}</tbody></table>` : ''}
      </div>
      <div class="modal-footer">
        <button class="btn btn-ghost" onclick="closeModal()">Close</button>
        <button class="btn btn-danger" onclick="if(confirm('Delete user ${jsEsc(u.screen_name)}?')){ENDPOINTS.dbUserDelete('${jsEsc(u.id)}').then(()=>{closeModal();loadDBTab('users');}).catch(e=>toast(e.message,'error'))}">Delete</button>
      </div>`);
  } catch(e) { toast(e.message, 'error'); }
}

async function viewListDetail(id) {
  try {
    const l = await ENDPOINTS.dbList(id);
    const ents = await ENDPOINTS.dbListEntities(id);

    openModal(`
      <div class="modal-header"><h2>List: ${esc(l.name)}</h2><button class="btn btn-ghost btn-sm" onclick="closeModal()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div>
      <div class="modal-body">
        <div class="form-row">
          <div class="form-group"><label>ID</label><code>${esc(l.id)}</code></div>
          <div class="form-group"><label>Owner ID</label><code>${esc(l.owner_user_id)}</code></div>
        </div>
        ${ents.length ? `<div class="section-header mt-4"><h3>Entities (${ents.length})</h3></div>
        <table><thead><tr><th>Name</th><th>Parent Dir</th></tr></thead><tbody>${ents.map(e => `<tr><td>${esc(e.name)}</td><td>${esc(e.parent_dir)}</td></tr>`).join('')}</tbody></table>` : ''}
      </div>
      <div class="modal-footer">
        <button class="btn btn-ghost" onclick="closeModal()">Close</button>
      </div>`);
  } catch(e) { toast(e.message, 'error'); }
}

/* ---- Schedules Page ---- */
function renderSchedulesPage(container) {
  container.innerHTML = `
    <div class="section">
      <div class="section-header">
        <h2>Schedules</h2>
        <div class="flex gap-2">
          <button class="btn btn-primary btn-sm" onclick="showNewScheduleForm()">+ Add</button>
          <button class="btn btn-ghost btn-sm" onclick="triggerAllSchedules()">Trigger All</button>
          <button class="btn btn-ghost btn-sm" onclick="reloadSchedules()">Reload</button>
        </div>
      </div>
      <div class="stats-grid" id="sched-stats"></div>
      <div id="sched-warning" class="hidden"></div>
      <div id="sched-list"></div>
    </div>`;

  pageRenderers.schedules = renderSchedulesPage;
  updateSchedulesView();
  loadSchedules();
}

async function loadSchedules() {
  try {
    const r = await ENDPOINTS.schedules();
    window._lastSchedulesData = { scheduler_running: r.scheduler_running, entries: r.entries || [] };
    updateSchedulesView();
  } catch(e) { toast(e.message, 'error'); }
}

function updateSchedulesView() {
  const d = window._lastSchedulesData;
  if (!d) return;

  const entries = d.entries || [];
  const running = d.scheduler_running;

  // ScheduleStatus wraps ScheduleEntry in .entry; flatten for templates
  const flatEntries = entries.map(e => ({
    ...(e.entry || e),
    last_run_at: e.last_run_at || e.last_run,
    next_run_at: e.next_run_at || e.next_run,
    last_error: e.last_error,
    consecutive_failures: e.consecutive_failures
  }));

  const stats = {
    total: flatEntries.length,
    enabled: flatEntries.filter(e => e.enabled).length,
    disabled: flatEntries.filter(e => !e.enabled).length,
    failures: entries.reduce((s, e) => s + (e.consecutive_failures || 0), 0)
  };

  const statsEl = document.getElementById('sched-stats');
  if (statsEl) {
    statsEl.innerHTML = [
      ['Total', stats.total, 'total'],
      ['Enabled', stats.enabled, 'completed'],
      ['Disabled', stats.disabled, 'cancelled'],
      ['Failures', stats.failures, 'failed']
    ].map(([k, v, cls]) => `<div class="stat-card ${cls}"><div class="stat-value">${v}</div><div class="stat-label">${k}</div></div>`).join('');
  }

  // Warning when scheduler not running
  const warnEl = document.getElementById('sched-warning');
  if (warnEl) {
    if (!running) {
      warnEl.className = 'alert alert-warning';
      warnEl.innerHTML = 'Scheduler is not running - scheduled downloads will not execute automatically.';
    } else {
      warnEl.className = 'hidden';
    }
  }

  const listEl = document.getElementById('sched-list');
  if (!listEl) return;

  if (!flatEntries.length) {
    listEl.innerHTML = '<div class="empty-state"><p>No schedules configured.</p></div>';
    return;
  }

  listEl.innerHTML = flatEntries.map(e => `
    <div class="schedule-item${e.consecutive_failures > 0 ? ' has-failure' : ''}">
      <div class="schedule-item-header">
        <div class="schedule-item-title">
          <span class="schedule-status-dot ${e.enabled ? 'enabled' : 'disabled'}"></span>
          <strong>${esc(e.name || e.target || 'Unnamed')}</strong>
          <span class="badge badge-${e.type === 'list' ? 'running' : 'queued'}">${esc(e.type)}</span>
          ${e.consecutive_failures > 0 ? `<span class="badge badge-failed">${e.consecutive_failures} failures</span>` : ''}
        </div>
        <div class="flex gap-2">
          <label class="toggle">
            <input type="checkbox" name="sched-toggle-${esc(e.id)}" ${e.enabled ? 'checked' : ''} onchange="toggleSchedule('${jsEsc(e.id)}', this.checked)">
            <span class="toggle-slider"></span>
          </label>
          <button class="btn btn-xs btn-ghost" onclick="triggerSchedule('${jsEsc(e.id)}')" title="Run now">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          </button>
          <button class="btn btn-xs btn-ghost" onclick="editSchedule('${jsEsc(e.id)}')" title="Edit">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
          </button>
          <button class="btn btn-xs btn-ghost" onclick="deleteSchedule('${jsEsc(e.id)}')" title="Delete">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
          </button>
        </div>
      </div>
      <div class="schedule-meta">
        <span>Schedule: ${esc(e.schedule)}</span>
        <span> &middot; ${e.type === 'mixed' ? ('Users: ' + (e.users||[]).length + ', Lists: ' + (e.lists||[]).length + ', Following: ' + (e.following_names||[]).length) : 'Target: ' + esc(e.target)}</span>
        ${e.last_run_at ? '<span> &middot; Last: ' + relativeTime(e.last_run_at) + '</span>' : ''}
        ${e.next_run_at ? '<span> &middot; Next: ' + relativeTime(e.next_run_at) + '</span>' : ''}
        ${e.last_error ? '<span class="fail"> &middot; Error: ' + esc(e.last_error) + '</span>' : ''}
      </div>
    </div>`).join('');
}

function showNewScheduleForm() {
  openModal(`
    <div class="modal-header"><h2>Add Schedule</h2><button class="btn btn-ghost btn-sm" onclick="closeModal()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div>
    <div class="modal-body">
      <div class="form-group">
        <label>Type</label>
        <select id="sched-type" onchange="toggleSchedTargetFields()">
          <option value="user">User</option>
          <option value="list">List</option>
          <option value="following">Following</option>
          <option value="mixed">Mixed</option>
        </select>
      </div>
      <div id="sched-target-fields">
        <div class="form-group" id="sched-target-single">
          <label>Target</label>
          <input type="text" id="sched-target" placeholder="screen_name or list_id">
        </div>
        <div class="form-group hidden" id="sched-target-mixed">
          <label>Users (one per line)</label>
          <textarea id="sched-mixed-users" rows="2" placeholder="elonmusk"></textarea>
        </div>
        <div class="form-group hidden" id="sched-target-mixed-lists">
          <label>Lists (one per line)</label>
          <textarea id="sched-mixed-lists" rows="2" placeholder="1234567890"></textarea>
        </div>
        <div class="form-group hidden" id="sched-target-mixed-foll">
          <label>Following (one per line)</label>
          <textarea id="sched-mixed-foll" rows="2" placeholder="jack"></textarea>
        </div>
      </div>
      <div class="form-group">
        <label>Name</label>
        <input type="text" id="sched-name" placeholder="My Schedule">
      </div>
      <div class="form-group">
        <label>Schedule</label>
        <input type="text" id="sched-schedule" placeholder="daily 08:00,20:00 or interval 4h">
        <div class="hint">Format: "daily HH:MM" or "interval 1h30m"</div>
      </div>
      <label class="checkbox-label"><input type="checkbox" id="sched-runonstart"> Run on start</label>
    </div>
    <div class="modal-footer">
      <button class="btn btn-ghost" onclick="closeModal()">Cancel</button>
      <button class="btn btn-primary" onclick="saveNewSchedule()">Create</button>
    </div>`);
}

function toggleSchedTargetFields() {
  const type = document.getElementById('sched-type').value;
  document.getElementById('sched-target-single').classList.toggle('hidden', type === 'mixed');
  document.getElementById('sched-target-mixed').classList.toggle('hidden', type !== 'mixed');
  document.getElementById('sched-target-mixed-lists').classList.toggle('hidden', type !== 'mixed');
  document.getElementById('sched-target-mixed-foll').classList.toggle('hidden', type !== 'mixed');
}

async function saveNewSchedule() {
  const type = document.getElementById('sched-type').value;
  const schedule = document.getElementById('sched-schedule').value.trim();
  if (!schedule) return toast('Schedule pattern required', 'warning');
  const name = document.getElementById('sched-name').value.trim();
  const runOnStart = document.getElementById('sched-runonstart').checked;
  closeModal();
  try {
    const base = { type, name, schedule, enabled: true, run_on_start: runOnStart };
    if (type === 'mixed') {
      const users = document.getElementById('sched-mixed-users').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
      const lists = document.getElementById('sched-mixed-lists').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
      const foll = document.getElementById('sched-mixed-foll').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
      if (!users.length && !lists.length && !foll.length) return toast('Enter at least one target', 'warning');
      await ENDPOINTS.createSchedule({ ...base, users, lists, following_names: foll });
    } else {
      const target = document.getElementById('sched-target').value.trim();
      if (!target) return toast('Target required', 'warning');
      await ENDPOINTS.createSchedule({ ...base, target });
    }
    toast('Schedule created', 'success');
    loadSchedules();
  } catch(e) { toast(e.message, 'error'); }
}

async function toggleSchedule(id, enabled) {
  try { await ENDPOINTS.setScheduleEnabled(id, enabled); loadSchedules(); }
  catch(e) { toast(e.message, 'error'); }
}

async function triggerSchedule(id) {
  try { const r = await ENDPOINTS.triggerSchedule(id); toast('Triggered: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function triggerAllSchedules() {
  if (!confirm('Trigger all enabled schedules?')) return;
  const btn = document.querySelector('[onclick="triggerAllSchedules()"]');
  if (btn) { btn.disabled = true; btn.textContent = 'Triggering...'; }
  try {
    const r = await ENDPOINTS.triggerAll();
    if (r.failed > 0) {
      const errMsgs = (r.results || []).filter(x => x.error).map(x => x.entry_id + ': ' + x.error).join('; ');
      toast('Triggered ' + r.succeeded + '/' + r.total + ' schedules (' + r.failed + ' failed): ' + errMsgs, 'warning');
    } else {
      toast('All ' + r.succeeded + ' schedules triggered successfully', 'success');
    }
  } catch(e) { toast(e.message, 'error'); }
  finally { if (btn) { btn.disabled = false; btn.textContent = 'Trigger All'; } }
}

async function reloadSchedules() {
  try { await ENDPOINTS.reloadSchedules(); toast('Schedules reloaded', 'success'); loadSchedules(); }
  catch(e) { toast(e.message, 'error'); }
}

async function editSchedule(id) {
  const d = window._lastSchedulesData;
  const entry = d && d.entries ? d.entries.find(e => (e.entry && e.entry.id === id) || e.id === id) : null;
  if (!entry) return toast('Schedule not found', 'error');
  const ent = entry.entry || entry;

  openModal(`
    <div class="modal-header"><h2>Edit Schedule</h2><button class="btn btn-ghost btn-sm" onclick="closeModal()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div>
    <div class="modal-body">
      <div class="form-group">
        <label>Type</label>
        <select id="sched-edit-type" onchange="toggleEditSchedTargetFields()">
          <option value="user" ${ent.type === 'user' ? 'selected' : ''}>User</option>
          <option value="list" ${ent.type === 'list' ? 'selected' : ''}>List</option>
          <option value="following" ${ent.type === 'following' ? 'selected' : ''}>Following</option>
          <option value="mixed" ${ent.type === 'mixed' ? 'selected' : ''}>Mixed</option>
        </select>
      </div>
      <div id="sched-edit-target-fields">
        <div class="form-group" id="sched-edit-target-single" ${ent.type === 'mixed' ? 'style="display:none"' : ''}>
          <label>Target</label>
          <input type="text" id="sched-edit-target" value="${esc(ent.target||'')}">
        </div>
        <div class="form-group" id="sched-edit-target-mixed" ${ent.type !== 'mixed' ? 'style="display:none"' : ''}>
          <label>Users (one per line)</label>
          <textarea id="sched-edit-mixed-users" rows="2">${esc((ent.users||[]).join('\n'))}</textarea>
        </div>
        <div class="form-group" id="sched-edit-mixed-lists" ${ent.type !== 'mixed' ? 'style="display:none"' : ''}>
          <label>Lists (one per line)</label>
          <textarea id="sched-edit-mixed-lists" rows="2">${esc((ent.lists||[]).join('\n'))}</textarea>
        </div>
        <div class="form-group" id="sched-edit-mixed-foll" ${ent.type !== 'mixed' ? 'style="display:none"' : ''}>
          <label>Following (one per line)</label>
          <textarea id="sched-edit-mixed-foll" rows="2">${esc((ent.following_names||[]).join('\n'))}</textarea>
        </div>
      </div>
      <div class="form-group">
        <label>Name</label>
        <input type="text" id="sched-edit-name" value="${esc(ent.name||'')}">
      </div>
      <div class="form-group">
        <label>Schedule</label>
        <input type="text" id="sched-edit-schedule" value="${esc(ent.schedule||'')}">
      </div>
      <label class="checkbox-label"><input type="checkbox" id="sched-edit-runonstart" ${ent.run_on_start ? 'checked' : ''}> Run on start</label>
    </div>
    <div class="modal-footer">
      <button class="btn btn-ghost" onclick="closeModal()">Cancel</button>
      <button class="btn btn-primary" onclick="saveScheduleEdit('${jsEsc(id)}')">Save</button>
    </div>`);
}

async function saveScheduleEdit(id) {
  const type = document.getElementById('sched-edit-type').value;
  const name = document.getElementById('sched-edit-name').value.trim();
  const schedule = document.getElementById('sched-edit-schedule').value.trim();
  if (!schedule) return toast('Schedule pattern required', 'warning');
  const runOnStart = document.getElementById('sched-edit-runonstart').checked;
  closeModal();
  try {
    const base = { type, name, schedule, run_on_start: runOnStart };
    if (type === 'mixed') {
      const users = document.getElementById('sched-edit-mixed-users').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
      const lists = document.getElementById('sched-edit-mixed-lists').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
      const foll = document.getElementById('sched-edit-mixed-foll').value.trim().split('\n').map(s => s.trim()).filter(Boolean);
      await ENDPOINTS.updateSchedule(id, { ...base, users, lists, following_names: foll });
    } else {
      const target = document.getElementById('sched-edit-target').value.trim();
      if (!target) return toast('Target required', 'warning');
      await ENDPOINTS.updateSchedule(id, { ...base, target });
    }
    toast('Schedule updated', 'success');
    loadSchedules();
  } catch(e) { toast(e.message, 'error'); }
}

function toggleEditSchedTargetFields() {
  const type = document.getElementById('sched-edit-type').value;
  document.getElementById('sched-edit-target-single').style.display = type === 'mixed' ? 'none' : '';
  document.getElementById('sched-edit-target-mixed').style.display = type !== 'mixed' ? 'none' : '';
  document.getElementById('sched-edit-mixed-lists').style.display = type !== 'mixed' ? 'none' : '';
  document.getElementById('sched-edit-mixed-foll').style.display = type !== 'mixed' ? 'none' : '';
}

async function deleteSchedule(id) {
  if (!confirm('Delete this schedule?')) return;
  try { await ENDPOINTS.deleteSchedule(id); toast('Schedule deleted', 'info'); loadSchedules(); }
  catch(e) { toast(e.message, 'error'); }
}

/* ---- System Page ---- */
function renderSystemPage(container) {
  container.innerHTML = `
    <div class="section">
      <div class="section-header"><h2>System</h2></div>
      <div class="stats-grid" id="sys-queue"></div>
    </div>

    <div class="section">
      <div class="section-header"><h2>Configuration</h2></div>
      <div class="card">
        <div class="card-header">
          <div class="tabs" id="config-tabs">
            <button class="tab active" data-configtab="fields">Fields</button>
            <button class="tab" data-configtab="raw">Raw YAML</button>
            <button class="tab" data-configtab="cookies">Cookies</button>
            <button class="tab" data-configtab="cookies-raw">Raw Cookies</button>
          </div>
          <button class="btn btn-danger btn-sm" style="flex-shrink:0" onclick="if(confirm('Shut down the server?')){ENDPOINTS.shutdown().then(r=>toast(r.message||'Shutting down...','warning')).catch(e=>toast(e.message,'error'))}">Shut Down Server</button>
        </div>
        <div class="card-body" id="config-content">
          <div class="loading"><div class="spinner"></div> Loading...</div>
        </div>
      </div>
    </div>

    <div class="section">
      <div class="section-header"><h2>Errors</h2></div>
      <div class="card">
        <div class="card-body" id="errors-content">
          <div class="loading"><div class="spinner"></div> Loading...</div>
        </div>
      </div>
    </div>`;

  pageRenderers.system = renderSystemPage;
  loadSystemData();
  loadConfigTab('fields');
  loadErrors();

  // Tab switching for System page
  const configTabs = document.getElementById('config-tabs');
  if (configTabs) {
    configTabs.addEventListener('click', (e) => {
      const tab = e.target.closest('[data-configtab]');
      if (!tab) return;
      document.querySelectorAll('[data-configtab]').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      loadConfigTab(tab.dataset.configtab);
    });
  }
}

async function loadSystemData() {
  try {
    const [health, queue] = await Promise.all([ENDPOINTS.health(), ENDPOINTS.queueStatus()]);
    const qEl = document.getElementById('sys-queue');
    if (qEl) {
      qEl.innerHTML = [
        ['Status', health.status || 'ok', health.status === 'ok' ? 'completed' : 'failed'],
        ['Queue Depth', queue.queue_depth || 0, 'total'],
        ['Active Jobs', queue.active_jobs || 0, 'running'],
        ['Pending', queue.pending_jobs || 0, 'queued'],
        ['Detached', queue.detached_jobs || 0, 'cancelled'],
      ].map(([k, v, cls]) => `<div class="stat-card ${cls}"><div class="stat-value">${v}</div><div class="stat-label">${k}</div></div>`).join('');
    }
  } catch(e) { /* ignore */ }
}

async function loadConfigTab(tab) {
  const content = document.getElementById('config-content');
  if (!content) return;
  content.innerHTML = '<div class="loading"><div class="spinner"></div> Loading...</div>';

  try {
    switch (tab) {
      case 'fields': await renderConfigFields(content); break;
      case 'raw': await renderConfigRaw(content); break;
      case 'cookies': await renderCookies(content); break;
      case 'cookies-raw': await renderCookiesRaw(content); break;
    }
  } catch(e) {
    content.innerHTML = '<div class="empty-state"><p>Error: ' + esc(e.message) + '</p></div>';
  }
}

async function renderConfigFields(content) {
  const r = await ENDPOINTS.configFields();
  const fields = r.fields || [];
  content.innerHTML = `
    <div id="config-fields-form">
      ${fields.map(f => `
        <div class="form-group">
          <label>${esc(f.label || f.name)}</label>
          ${f.type === 'number'
            ? `<input type="number" id="cf-${esc(f.name)}" value="${esc(f.value||f.default||'')}" placeholder="${esc(f.placeholder||'')}">`
            : `<input type="text" id="cf-${esc(f.name)}" value="${esc(f.value||f.default||'')}" placeholder="${esc(f.placeholder||'')}">`
          }
          ${f.prompt ? '<div class="hint">' + esc(f.prompt) + '</div>' : ''}
        </div>`).join('')}
      <div class="form-actions">
        <button class="btn btn-primary" onclick="saveConfigFields()">Save</button>
      </div>
    </div>`;
}

async function saveConfigFields() {
  try {
    const r = await ENDPOINTS.configFields();
    const fields = r.fields || [];
    const data = {};
    fields.forEach(f => {
      const el = document.getElementById('cf-' + f.name);
      if (el) data[f.name] = el.value;
    });
    await ENDPOINTS.saveConfigFields(data);
    toast('Configuration saved (restart to apply)', 'success');
  } catch(e) { toast(e.message, 'error'); }
}

async function renderConfigRaw(content) {
  const r = await ENDPOINTS.configRaw();
  content.innerHTML = `
    <div class="form-group">
      <label>Raw YAML Configuration</label>
      <textarea id="config-raw-text" rows="15">${esc(r.content||'')}</textarea>
      <div class="hint">Path: ${esc(r.path)}</div>
    </div>
    <div class="form-actions">
      <button class="btn btn-primary" onclick="saveConfigRaw()">Save</button>
    </div>`;
}

async function saveConfigRaw() {
  const el = document.getElementById('config-raw-text');
  if (!el) return toast('Configuration form not found', 'error');
  const text = el.value;
  try { await ENDPOINTS.saveConfigRaw(text); toast('Configuration saved (restart to apply)', 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function renderCookies(content) {
  let cookies;
  try { cookies = await ENDPOINTS.cookies(); } catch(e) { cookies = []; }
  const cArr = cookies && Array.isArray(cookies) ? cookies : (cookies && cookies.items) || [];
  content.innerHTML = `
    <div id="cookies-form">
      ${cArr.length === 0 ? '<p class="text-muted">No additional cookies configured.</p>' : ''}
      ${cArr.map((c, i) => `
        <div class="form-row" style="margin-bottom:8px">
          <input type="text" id="cookie-at-${i}" value="${esc(c.auth_token||'')}" placeholder="auth_token" style="font-family:var(--font-mono);font-size:12px">
          <input type="text" id="cookie-ct0-${i}" value="${esc(c.ct0||'')}" placeholder="ct0" style="font-family:var(--font-mono);font-size:12px">
        </div>`).join('')}
      <div class="form-actions">
        <button class="btn btn-ghost btn-sm" onclick="addCookieRow()">+ Add Account</button>
        <button class="btn btn-primary" onclick="saveCookies()">Save</button>
      </div>
    </div>`;
}

async function saveCookies() {
  const cArr = document.querySelectorAll('[id^="cookie-at-"]');
  const cookies = Array.from(cArr).map((el, i) => ({
    auth_token: el.value,
    ct0: document.getElementById('cookie-ct0-' + i) ? document.getElementById('cookie-ct0-' + i).value : ''
  }));
  try { await ENDPOINTS.saveCookies(cookies); toast('Cookies saved', 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

function addCookieRow() {
  const form = document.getElementById('cookies-form');
  if (!form) return;
  const existing = form.querySelectorAll('[id^="cookie-at-"]');
  const idx = existing.length;
  const row = document.createElement('div');
  row.className = 'form-row';
  row.style.marginBottom = '8px';
  row.innerHTML = '<input type="text" id="cookie-at-' + idx + '" value="" placeholder="auth_token" style="font-family:var(--font-mono);font-size:12px">' +
    '<input type="text" id="cookie-ct0-' + idx + '" value="" placeholder="ct0" style="font-family:var(--font-mono);font-size:12px">';
  const actions = form.querySelector('.form-actions');
  if (actions) form.insertBefore(row, actions);
}

async function renderCookiesRaw(content) {
  const r = await ENDPOINTS.cookiesRaw();
  content.innerHTML = `
    <div class="form-group">
      <label>Raw Cookies YAML</label>
      <textarea id="cookies-raw-text" rows="12">${esc(r.content||'')}</textarea>
      <div class="hint">Path: ${esc(r.path)}</div>
    </div>
    <div class="form-actions">
      <button class="btn btn-primary" onclick="saveCookiesRaw()">Save</button>
    </div>`;
}

async function saveCookiesRaw() {
  const el = document.getElementById('cookies-raw-text');
  if (!el) return toast('Cookies form not found', 'error');
  const text = el.value;
  try { await ENDPOINTS.saveCookiesRaw(text); toast('Cookies saved', 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function loadErrors() {
  const content = document.getElementById('errors-content');
  if (!content) return;
  try {
    const r = await ENDPOINTS.errors();
    const regular = r.regular || {};
    const json = r.json || [];
    const regKeys = Object.keys(regular);

    content.innerHTML = `
      ${regKeys.length || json.length ? `<div class="form-actions"><button class="btn btn-primary btn-sm" onclick="retryAllErrors()">Retry All Failed</button><button class="btn btn-danger btn-sm" onclick="clearAllErrors()">Clear Errors</button></div>` : ''}
      ${regKeys.length ? `<div class="section-header mt-2"><h3>Regular errors (${regKeys.length} entities)</h3></div>
      <table><thead><tr><th>Entity ID</th><th>Failed Tweets</th></tr></thead><tbody>${regKeys.map(k => `<tr><td>${esc(k)}</td><td>${regular[k]}</td></tr>`).join('')}</tbody></table>` : ''}
      ${json.length ? `<div class="section-header mt-2"><h3>JSON errors (${json.length} sources)</h3></div>
      <table><thead><tr><th>Source</th><th>Count</th></tr></thead><tbody>${json.map(j => `<tr><td class="mono">${esc(j.source_path||'')}</td><td>${j.count||0}</td></tr>`).join('')}</tbody></table>` : ''}
      ${!regKeys.length && !json.length ? '<p class="text-muted">No errors recorded.</p>' : ''}`;
  } catch(e) {
    content.innerHTML = '<div class="empty-state"><p>Error loading errors: ' + esc(e.message) + '</p></div>';
  }
}

async function retryAllErrors() {
  try { const r = await ENDPOINTS.retryErrors(); toast('Retry task: ' + r.task_id, 'success'); }
  catch(e) { toast(e.message, 'error'); }
}

async function clearAllErrors() {
  if (!confirm('Clear all error records?')) return;
  try { await ENDPOINTS.clearErrors(); toast('Errors cleared', 'info'); loadErrors(); }
  catch(e) { toast(e.message, 'error'); }
}

/* ---- Logs Page ---- */
function renderLogsPage(container) {
  container.innerHTML = `
    <div class="section">
      <div class="section-header">
        <h2>Logs</h2>
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-sm" onclick="refreshLogs()">Refresh</button>
          <button class="btn btn-ghost btn-sm" onclick="exportLogs()">Export</button>
          <label class="checkbox-label" style="font-size:12px"><input type="checkbox" id="log-auto-scroll-toggle" checked onchange="toggleLogAutoScroll()"> Auto-scroll</label>
        </div>
      </div>
      <div class="card">
        <div class="card-header">
          <div class="flex gap-2 items-center" style="flex-wrap:wrap">
            <input type="text" id="log-level" placeholder="level (info/warn/error)" style="width:140px" onkeydown="if(event.key==='Enter')setLogLevel()">
            <input type="text" id="log-search-input" placeholder="search text..." style="width:140px" onkeydown="if(event.key==='Enter')doLogSearch()">
            <button class="btn btn-primary btn-sm" onclick="setLogLevel()">Filter</button>
            <button class="btn btn-ghost btn-sm" onclick="doLogSearch()">Search</button>
            <span id="log-stats-inline" class="text-sm text-muted" style="margin-left:8px"></span>
          </div>
        </div>
        <div class="card-body" style="padding:0;position:relative">
          <div class="log-stream" id="log-stream">
            <div class="loading"><div class="spinner"></div> Loading logs...</div>
          </div>
          <button class="log-scroll-to-top-btn" id="log-new-arrived-btn"
            style="display:none" onclick="scrollLogToBottom()">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/></svg>
            New logs arrived
          </button>
        </div>
      </div>
    </div>`;

  pageRenderers.logs = renderLogsPage;
  refreshLogs();
  loadLogStats();
  connectLogSSE();

  // Auto-uncheck only when user scrolls UP from bottom (scrolling down never unchecks)
  let _lastScrollTop = 0;
  const logStream = document.getElementById('log-stream');
  logStream.addEventListener('scroll', () => {
    const atBottom = logStream.scrollTop + logStream.clientHeight >= logStream.scrollHeight - 10;
    const scrolledUp = logStream.scrollTop < _lastScrollTop;
    _lastScrollTop = logStream.scrollTop;
    if (atBottom) {
      // User scrolled to bottom → hide button if visible
      const btn = document.getElementById('log-new-arrived-btn');
      if (btn) btn.style.display = 'none';
    } else if (scrolledUp && logAutoScroll) {
      // User scrolled up → uncheck
      logAutoScroll = false;
      const cb = document.getElementById('log-auto-scroll-toggle');
      if (cb) cb.checked = false;
    }
  });
}

let logSSESource = null;
let logAutoScroll = true;
let _logReconnectAttempts = 0;
let _logIntentionalDisconnect = false;

function toggleLogAutoScroll() {
  logAutoScroll = document.getElementById('log-auto-scroll-toggle').checked;
  if (logAutoScroll) {
    const stream = document.getElementById('log-stream');
    if (stream) stream.scrollTop = stream.scrollHeight;
  }
}
function exportLogs() { window.open(apiBase() + '/api/v1/logs/export'); }

function setLogLevel() {
  refreshLogs();
  disconnectLogSSE();
  connectLogSSE();
}

function doLogSearch() {
  refreshLogs();
  disconnectLogSSE();
  connectLogSSE();
}

function scrollLogToBottom() {
  const stream = document.getElementById('log-stream');
  if (stream) { stream.scrollTop = stream.scrollHeight; }
  const btn = document.getElementById('log-new-arrived-btn');
  if (btn) btn.style.display = 'none';
  logAutoScroll = true;
  const cb = document.getElementById('log-auto-scroll-toggle');
  if (cb) cb.checked = true;
}

async function refreshLogs() {
  const stream = document.getElementById('log-stream');
  if (!stream) return;
  const level = document.getElementById('log-level') ? document.getElementById('log-level').value.trim() : '';
  const q = document.getElementById('log-search-input') ? document.getElementById('log-search-input').value.trim() : '';
  try {
    const r = await ENDPOINTS.logs({ page:1, pageSize:200, level: level || undefined, q: q || undefined });
    const lines = (r.logs || []).reverse();
    stream.innerHTML = lines.map(l => {
      const clean = stripAnsi(l);
      const color = getLogLineColor(clean);
      const tweetId = getTweetId(clean);
      return '<div class="log-entry" style="color:' + color + '"' + (tweetId ? ' data-tweet-id="' + tweetId + '"' : '') + '>' + highlightLogTimestamp(esc(clean)) + '</div>';
    }).join('');
    stream.scrollTop = stream.scrollHeight;
  } catch(e) {
    stream.innerHTML = '<div class="log-entry">Error loading logs: ' + esc(e.message) + '</div>';
  }
}

async function loadLogStats() {
  try {
    const s = await ENDPOINTS.logStats();
    const el = document.getElementById('log-stats-inline');
    if (el) el.textContent = (s.total || 0) + ' lines' + (s.level ? ', level: ' + s.level : '');
  } catch(e) { /* optional stat, ignore silently */ }
}

function connectLogSSE() {
  if (logSSESource) { logSSESource.close(); logSSESource = null; }
  if (window._logSSETimer) { clearTimeout(window._logSSETimer); window._logSSETimer = null; }
  _logIntentionalDisconnect = false;
  const level = document.getElementById('log-level') ? document.getElementById('log-level').value.trim() : '';
  const q = document.getElementById('log-search-input') ? document.getElementById('log-search-input').value.trim() : '';
  const params = new URLSearchParams();
  if (level) params.append('level', level);
  if (q) params.append('q', q);
  const qs = params.toString();
  logSSESource = new EventSource(apiBase() + '/api/v1/logs/stream' + (qs ? '?' + qs : ''));

  logSSESource.addEventListener('log', (e) => {
    const stream = document.getElementById('log-stream');
    if (!stream) return;
    const el = document.createElement('div');
    el.className = 'log-entry';
    const clean = stripAnsi(e.data);
    const tweetId = getTweetId(clean);
    if (tweetId) el.dataset.tweetId = tweetId;
    const color = getLogLineColor(clean);
    el.innerHTML = highlightLogTimestamp(esc(clean));
    el.style.color = color;
    stream.appendChild(el);
    if (logAutoScroll) {
      // Auto-scroll is checked → always scroll to show new log
      stream.scrollTop = stream.scrollHeight;
    } else {
      // Only show button if user is NOT at the bottom
      const userAtBottom = stream.scrollTop + stream.clientHeight >= stream.scrollHeight - 10;
      if (!userAtBottom) {
        const btn = document.getElementById('log-new-arrived-btn');
        if (btn) btn.style.display = 'flex';
      }
    }
    // Keep last 5000 lines
    while (stream.children.length > 5000) stream.removeChild(stream.firstChild);
  });

  logSSESource.onerror = () => {
    if (logSSESource) { logSSESource.close(); logSSESource = null; }
    if (_logIntentionalDisconnect) { _logIntentionalDisconnect = false; return; }
    _logReconnectAttempts++;
    if (_logReconnectAttempts > 60) {
      _logReconnectAttempts = 0;
      return;
    }
    const delay = Math.min(2000 * Math.pow(1.5, _logReconnectAttempts - 1), 30000);
    window._logSSETimer = setTimeout(connectLogSSE, delay);
  };
}

function disconnectLogSSE() {
  _logIntentionalDisconnect = true;
  if (logSSESource) { logSSESource.close(); logSSESource = null; }
  if (window._logSSETimer) { clearTimeout(window._logSSETimer); window._logSSETimer = null; }
  _logReconnectAttempts = 0;
}

// 点击日志行任意位置复制推文 ID
document.addEventListener('click', (e) => {
  const entry = e.target.closest('.log-entry[data-tweet-id]');
  if (!entry) return;
  const id = entry.dataset.tweetId;
  if (id) {
    navigator.clipboard.writeText(id).then(() => {
      toast('已复制推文 ID: ' + id, 'success');
    }).catch(() => {
      toast('复制失败，请手动选择文本复制', 'warning');
    });
  }
});

/* ---- Sidebar ---- */
function toggleSidebar() {
  document.getElementById('sidebar').classList.toggle('open');
}

/* ---- Init ---- */
document.addEventListener('DOMContentLoaded', () => {
  // Determine initial page
  const path = location.pathname.replace(/^\//, '') || 'tasks';
  currentPage = path;

  // Highlight sidebar
  document.querySelectorAll('.nav-item').forEach(el => {
    el.addEventListener('click', () => navigateTo(el.dataset.page));
    el.classList.toggle('active', el.dataset.page === currentPage);
  });

  // Set page title
  const titles = {tasks:'Tasks', data:'Data', schedules:'Schedules', system:'System', logs:'Logs'};
  document.getElementById('page-title').textContent = titles[currentPage] || 'Tasks';

  // Connect SSE first
  connectSSE();

  // Initial health check
  checkHealth();
  setInterval(checkHealth, 30000);

  // Render initial page
  renderPage(currentPage);
});

// Export functions for inline onclick
window.ENDPOINTS = ENDPOINTS;
window.apiBase = apiBase;
