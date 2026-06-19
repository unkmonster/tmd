// ============================================
// Utility Functions
// ============================================
function debounce(fn, delay) {
  let timer = null;
  return function(...args) {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => { timer = null; fn.apply(this, args); }, delay);
  };
}

function glowNewFirstItem(panelId) {
  requestAnimationFrame(() => requestAnimationFrame(() => {
    const panel = document.getElementById(panelId);
    if (!panel) return;
    const first = panel.querySelector('.config-group');
    if (!first) return;
    first.classList.add('glow-new-item');
    first.addEventListener('animationend', () => first.classList.remove('glow-new-item'), { once: true });
  }));
}

function readListIDsFromTextarea(inputId) {
  const validListID = /^[1-9]\d{0,19}$/;
  const input = document.getElementById(inputId);
  if (!input) return [];
  const lines = input.value.split('\n').map(s => s.trim());
  const validIDs = [];
  const invalidCount = lines.filter(s => s && !validListID.test(s)).length;
  lines.forEach(s => { if (validListID.test(s)) validIDs.push(s); });
  if (invalidCount > 0) {
    toast.show(`发现 ${invalidCount} 个无效列表ID，已自动过滤`, 'warning');
  }
  return validIDs;
}

function readTextareaLines(inputId) {
  const el = document.getElementById(inputId);
  if (!el) return [];
  return el.value
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean);
}

// ============================================
// Search Input Helpers
// ============================================
function updateSearchState(stateKey, subKey, value) {
  if (subKey) {
    store.setState({
      [stateKey]: { ...store.state[stateKey], [subKey]: value }
    });
  } else {
    store.setState({ [stateKey]: value });
  }
}

function restoreSearchValue(inputId, stateKey, subKey = null) {
  const input = document.getElementById(inputId);
  if (!input) return;
  const value = subKey ? store.state[stateKey]?.[subKey] : store.state[stateKey];
  if (value !== undefined) {
    input.value = value;
  }
}

// ============================================
// State Management
// ============================================
function deepMerge(target, source) {
  const result = { ...target };
  for (const key of Object.keys(source)) {
    if (source[key] && typeof source[key] === 'object' && !Array.isArray(source[key]) &&
        target[key] && typeof target[key] === 'object' && !Array.isArray(target[key])) {
      result[key] = deepMerge(target[key], source[key]);
    } else {
      result[key] = source[key];
    }
  }
  return result;
}

const store = {
  state: {
    currentPage: 'overview',
    health: null,
    tasks: [],
    users: [],
    lists: [],
    entities: [],
    sidebarOpen: false,
    isMobile: window.innerWidth < 768,
    sseConnected: false,
    dataSubPage: 'users',
    taskFilter: 'all',
    taskSearch: '',
    // Database pagination state
    dbData: {
      users: { data: [], total: 0, page: 1, pageSize: 200 },
      lists: { data: [], total: 0, page: 1, pageSize: 200 },
      entities: { data: [], total: 0, page: 1, pageSize: 200 },
      listEntities: { data: [], total: 0, page: 1, pageSize: 200 },
      userLinks: { data: [], total: 0, page: 1, pageSize: 200 },
      previousNames: { data: [], total: 0, page: 1, pageSize: 200 }
    },
    dbPagination: {
      users: { page: 1, pageSize: 200, totalPages: 1 },
      lists: { page: 1, pageSize: 200, totalPages: 1 },
      entities: { page: 1, pageSize: 200, totalPages: 1 },
      listEntities: { page: 1, pageSize: 200, totalPages: 1 },
      userLinks: { page: 1, pageSize: 200, totalPages: 1 },
      previousNames: { page: 1, pageSize: 200, totalPages: 1 }
    },
    dbSort: {
      users: { sortBy: 'id', sortOrder: 'desc' },
      lists: { sortBy: 'id', sortOrder: 'desc' },
      entities: { sortBy: 'id', sortOrder: 'desc' },
      listEntities: { sortBy: 'id', sortOrder: 'desc' },
      userLinks: { sortBy: 'id', sortOrder: 'desc' },
      previousNames: { sortBy: 'record_date', sortOrder: 'desc' }
    },
    dbSearch: {
      users: '',
      lists: '',
      entities: '',
      listEntities: '',
      userLinks: '',
      previousNames: ''
    },
    _prevNameUserIdFilter: '',
    configRaw: null,
    configExists: false,
    configSaving: false,
    configFieldsLoading: false,
    logs: [],
    _logsLoading: true,
    logLevel: 'all',
    logSearch: '',
    logStats: { debug: 0, info: 0, warn: 0, error: 0, total: 0 },
    logAutoRefresh: true,
    _logAutoScrollPaused: false,
    _logNewArrived: false,
    logPagination: { page: 1, pageSize: 100, total: 0, totalPages: 1 },
    _systemTab: 'config',
    configMode: 'form',
    configFields: [],
    cookiesRaw: null,
    cookiesExists: false,
    cookiesSaving: false,
    cookieItems: [],
    _cookiesLoading: true,
    cookiesMode: 'form',
    _scheduleTab: 'form',
    _schedules: null,
    _scheduleRaw: null,
    _scheduleExists: false,
    _scheduleSaving: false,
    _scheduleFormItems: [],
    _scheduleFormDirty: false,
    _schedulerRunning: false,
  },

  listeners: [],

  subscribe(fn) {
    this.listeners.push(fn);
    return () => {
      const idx = this.listeners.indexOf(fn);
      if (idx !== -1) this.listeners.splice(idx, 1);
    };
  },

  setState(newState) {
    this.state = deepMerge(this.state, newState);
    this._scheduleNotify();
  },

  _notifyPending: false,

  _scheduleNotify() {
    if (this._notifyPending) return;
    this._notifyPending = true;
    Promise.resolve().then(() => {
      this._notifyPending = false;
      this.listeners.forEach(fn => fn(this.state));
    });
  }
};

// Update sidebar version when health changes (moved out of setState to keep it pure)
store.subscribe((state) => {
  if (state.health && state.health.version) {
    const versionEl = document.getElementById('appVersion');
    if (versionEl) versionEl.textContent = state.health.version;
  }
});

// ============================================
// API Client
// ============================================
const api = {
  base: '',
  _abortControllers: new Set(),

  abortAll() {
    const controllers = this._abortControllers;
    this._abortControllers = new Set();
    for (const ctrl of controllers) ctrl.abort();
  },

  _getAbortSignal() {
    const controller = new AbortController();
    this._abortControllers.add(controller);
    return { signal: controller.signal, controller };
  },
  
  _cleanupAbortController(controller) {
    this._abortControllers.delete(controller);
  },

  async request(method, path, body = null, extra = {}) {
    const { signal, controller } = this._getAbortSignal();
    const options = {
      method,
      signal
    };
    if (extra.isFormData) {
      if (body !== null && body !== undefined) options.body = body;
    } else {
      options.headers = { 'Content-Type': 'application/json' };
      if (body !== null && body !== undefined) options.body = JSON.stringify(body);
    }
    
    let res;
    try {
      res = await fetch(this.base + path, options);
    } catch (e) {
      this._cleanupAbortController(controller);
      throw new Error('网络请求失败，请检查服务器是否运行: ' + e.message);
    }
    const data = await res.json().catch(() => {
      this._cleanupAbortController(controller);
      return ({ success: false, error: `Invalid response (HTTP ${res.status})` });
    });
    this._cleanupAbortController(controller);
    if (!res.ok || !data.success) throw new Error(data.error || `HTTP ${res.status}`);
    return data.data;
  },
  
  get(path) { return this.request('GET', path); },
  post(path, body) { return this.request('POST', path, body); },
  
  // Health
  getHealth() { return this.get('/api/v1/health'); },
  
  // Tasks
  getTasks() { return this.get('/api/v1/tasks'); },
  getTask(id) { return this.get(`/api/v1/tasks/${id}`); },
  cancelTask(id) { return this.post(`/api/v1/tasks/${id}/cancel`, {}); },
  cancelQueuedTasks() { return this.post('/api/v1/tasks/cancel-queued', {}); },
  retryTask(id) { return this.post(`/api/v1/tasks/${id}/retry`, {}); },
  deleteTask(id) { return this.request('DELETE', `/api/v1/tasks/${id}`); },
  getTaskStats() { return this.get('/api/v1/tasks/stats'); },
  
  // Task Creation
  createUserDownload(screenName, opts) { 
    return this.post(`/api/v1/users/${encodeURIComponent(screenName)}/download`, opts); 
  },
  createProfileDownload(screenName) { 
    return this.post(`/api/v1/users/${encodeURIComponent(screenName)}/profile`, {}); 
  },
  createUserMark(screenName, timestamp) {
    return this.post(`/api/v1/users/${encodeURIComponent(screenName)}/mark`, timestamp ? { timestamp } : {});
  },
  createFollowingDownload(screenName, opts) {
    return this.post(`/api/v1/users/${encodeURIComponent(screenName)}/following/download`, opts);
  },
  createFollowingMark(screenName, timestamp) {
    return this.post(`/api/v1/users/${encodeURIComponent(screenName)}/following/mark`, timestamp ? { timestamp } : {});
  },
  createListDownload(listId, opts) { 
    return this.post(`/api/v1/lists/${encodeURIComponent(listId)}/download`, opts); 
  },
  createListProfile(listId) { 
    return this.post(`/api/v1/lists/${encodeURIComponent(listId)}/profile`, {}); 
  },
  createListMark(listId, timestamp) {
    return this.post(`/api/v1/lists/${encodeURIComponent(listId)}/mark`, timestamp ? { timestamp } : {});
  },
  createBatchDownload(data) { 
    return this.post('/api/v1/batch/download', data); 
  },
  createBatchMark(data) {
    return this.post('/api/v1/batch/mark', data);
  },
  createJsonFileDownload(data) {
    return this.post('/api/v1/json/file/download', data);
  },
  createJsonFolderDownload(data) {
    return this.post('/api/v1/json/folder/download', data);
  },
  upload(path, formData) {
    return this.request('POST', path, formData, { isFormData: true });
  },
  
  // Config
  getConfig() { return this.get('/api/v1/config'); },
  getConfigRaw() { return this.get('/api/v1/config/raw'); },
  updateConfigRaw(content) { return this.request('PUT', '/api/v1/config/raw', { content }); },
  getConfigFields() { return this.get('/api/v1/config/fields'); },
  saveConfigFields(fields) { return this.request('PUT', '/api/v1/config/fields', { fields }); },
  getCookiesRaw()           { return this.get('/api/v1/cookies/raw'); },
  updateCookiesRaw(content) { return this.request('PUT', '/api/v1/cookies/raw', { content }); },
  getCookies()              { return this.get('/api/v1/cookies'); },
  saveCookies(cookies)      { return this.request('PUT', '/api/v1/cookies', { cookies }); },
  shutdownServer() { return this.post('/api/v1/server/shutdown'); },

  // Logs
  getLogs(params = '') { return this.get(`/api/v1/logs${params}`); },
  getLogStats() { return this.get('/api/v1/logs/stats'); },
  downloadLogExport() { window.open('/api/v1/logs/export', '_blank'); },

  // Schedules
  getSchedules() { return this.get('/api/v1/schedules'); },
  replaceSchedules(entries) { return this.request('PUT', '/api/v1/schedules', { entries }); },
  setScheduleEnabled(id, enabled) { return this.request('PATCH', `/api/v1/schedules/${encodeURIComponent(id)}/enabled`, { enabled }); },
  getSchedulesRaw() { return this.get('/api/v1/schedules/raw'); },
  updateSchedulesRaw(content) { return this.request('PUT', '/api/v1/schedules/raw', { content }); },
  triggerSchedule(id) { return this.request('POST', `/api/v1/schedules/${encodeURIComponent(id)}/trigger`, {}); },
  triggerAllSchedules() { return this.post('/api/v1/schedules/trigger-all', {}); },
  getScheduleStats() { return this.get('/api/v1/schedules/stats'); },
  validateSchedule(body) { return this.post('/api/v1/schedules/validate', body); },

  // Queue
  getQueueStatus() { return this.get('/api/v1/queue/status'); },

  // Database CRUD with pagination
  getDBUsers(params = '') { return this.get(`/api/v1/db/users${params ? '?' + params : ''}`); },
  getDBUser(id) { return this.get(`/api/v1/db/users/${id}`); },
  updateDBUser(id, data) { return this.request('PATCH', `/api/v1/db/users/${id}`, data); },
  deleteDBUser(id) { return this.request('DELETE', `/api/v1/db/users/${id}`); },
  getDBUserEntities(id, params = '') { return this.get(`/api/v1/db/users/${id}/entities${params ? '?' + params : ''}`); },
  getDBUserLinks(id, params = '') { return this.get(`/api/v1/db/users/${id}/links${params ? '?' + params : ''}`); },

  getDBLists(params = '') { return this.get(`/api/v1/db/lists${params ? '?' + params : ''}`); },
  getDBList(id) { return this.get(`/api/v1/db/lists/${id}`); },
  updateDBList(id, data) { return this.request('PATCH', `/api/v1/db/lists/${id}`, data); },
  deleteDBList(id) { return this.request('DELETE', `/api/v1/db/lists/${id}`); },
  getDBListEntities(id, params = '') { return this.get(`/api/v1/db/lists/${id}/entities${params ? '?' + params : ''}`); },
  
  getDBUserEntities(params = '') { return this.get(`/api/v1/db/user-entities${params ? '?' + params : ''}`); },
  getDBUserEntity(id) { return this.get(`/api/v1/db/user-entities/${id}`); },
  updateDBUserEntity(id, data) { return this.request('PATCH', `/api/v1/db/user-entities/${id}`, data); },
  deleteDBUserEntity(id) { return this.request('DELETE', `/api/v1/db/user-entities/${id}`); },
  
  getDBListEntities(params = '') { return this.get(`/api/v1/db/list-entities${params ? '?' + params : ''}`); },
  getDBListEntity(id) { return this.get(`/api/v1/db/list-entities/${id}`); },
  updateDBListEntity(id, data) { return this.request('PATCH', `/api/v1/db/list-entities/${id}`, data); },
  deleteDBListEntity(id) { return this.request('DELETE', `/api/v1/db/list-entities/${id}`); },
  
  getDBUserLinks(params = '') { return this.get(`/api/v1/db/user-links${params ? '?' + params : ''}`); },
  getDBUserLink(id) { return this.get(`/api/v1/db/user-links/${id}`); },
  updateDBUserLink(id, data) { return this.request('PATCH', `/api/v1/db/user-links/${id}`, data); },
  deleteDBUserLink(id) { return this.request('DELETE', `/api/v1/db/user-links/${id}`); },
  getDBPreviousNames(params = '') { return this.get(`/api/v1/db/user-previous-names${params ? '?' + params : ''}`); },
  getDBStats() { return this.get('/api/v1/db/stats'); },
  getDBUserPreviousNames(id, params = '') { return this.get(`/api/v1/db/users/${id}/previous-names${params ? '?' + params : ''}`); }
};

// ============================================
// SSE Manager
// ============================================
const sseManager = {
  conn: null,
  reconnectTimer: null,
  reconnectDelay: 2000,
  maxReconnectDelay: 30000,
  baseReconnectDelay: 2000,
  reconnectAttempts: 0,
  reconnectDisabled: false,

  connect() {
    this.reconnectDisabled = false;
    if (this.conn) return;

    this.conn = new EventSource('/api/v1/sse/tasks');

    this.conn.onopen = () => {
      store.setState({ sseConnected: true });
      this._updateIndicator(true);
      if (this.reconnectAttempts > 0) {
        this.refreshCurrentPage();
      }
    };

    const debouncedTasksUpdate = debounce((tasks) => {
      store.setState({ tasks });
    }, 100);

    this.conn.addEventListener('tasks', (e) => {
      try {
        const tasks = JSON.parse(e.data);
        debouncedTasksUpdate(tasks);
        this.reconnectDelay = this.baseReconnectDelay;
        this.reconnectAttempts = 0;
      } catch (err) {
        console.warn('SSE tasks parse error:', err);
      }
    });

    const debouncedSchedulesUpdate = debounce((data) => {
      const entries = data.entries || [];
      const update = {
        _schedules: entries,
        _schedulerRunning: !!data.scheduler_running,
        _scheduleExists: data.exists !== undefined ? !!data.exists : store.state._scheduleExists,
      };
      if (!store.state._scheduleFormDirty && !isScheduleFormEditing()) {
        update._scheduleFormItems = entries.map(s => scheduleStatusToFormItem(s));
      }
      store.setState(update);
    }, 100);

    this.conn.addEventListener('schedules', (e) => {
      try {
        const data = JSON.parse(e.data);
        debouncedSchedulesUpdate(data);
      } catch (err) {
        console.warn('SSE schedules parse error:', err);
      }
    });

    this.conn.addEventListener('notification', (e) => {
      try {
        const notif = JSON.parse(e.data);
        // notification 与 tasks debounce (100ms) 对齐，避免 toast 先出现但任务列表仍显示"运行中"
        setTimeout(() => {
          const type = notif.type === 'task_completed' ? 'success' :
                       notif.type === 'task_failed' ? 'error' :
                       notif.type === 'task_cancelled' ? 'warning' :
                       notif.type === 'schedule_warning' ? 'warning' : 'success';
          toast.show(notif.message, type);
        }, 100);
      } catch (err) {
        console.warn('SSE notification parse error:', err);
      }
    });

    this.conn.addEventListener('server_shutdown', (e) => {
      try {
        const data = JSON.parse(e.data);
        handleServerShutdown(data.message);
      } catch (err) {
        handleServerShutdown('服务器正在关闭');
      }
    });

    this.conn.onerror = () => {
      this.conn.close();
      this.conn = null;
      store.setState({ sseConnected: false });
      this._updateIndicator(false);
      if (this.reconnectDisabled) return;
      if (store.state.currentPage === 'shutdown') return;
      this.reconnectAttempts++;
      if (this.reconnectAttempts >= 10 && this.reconnectAttempts % 5 === 0) {
        api.getHealth().catch(() => {
          // 忽略 — 健康检查失败可能是临时问题，继续重试
          console.warn('[SSE] 健康检查失败，继续重试...');
        });
      }
      const delay = Math.min(this.baseReconnectDelay * Math.pow(2, this.reconnectAttempts - 1), this.maxReconnectDelay);
      console.warn(`[SSE] 连接断开，${delay / 1000}s 后重试（第 ${this.reconnectAttempts} 次）`);
      this.reconnectTimer = setTimeout(() => {
        this.reconnectTimer = null;
        this.connect();
      }, delay);
    };
  },
  
  disconnect() {
    this.reconnectDisabled = true;
    this.reconnectAttempts = 0;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.conn) {
      this.conn.close();
      this.conn = null;
    }
    store.setState({ sseConnected: false });
    this._updateIndicator(false);
  },

  resume() {
    this.reconnectDisabled = false;
    this.connect();
  },

  _updateIndicator(connected) {
    const el = document.getElementById('sseIndicator');
    if (!el) return;
    el.classList.toggle('connected', connected);
    el.title = connected ? '实时连接正常 (点击刷新)' : '实时连接已断开 (点击刷新)';
  },

  refreshCurrentPage() {
    const page = store.state.currentPage;
    if (page === 'overview' || page === 'tasks') {
      this._safeRefresh(() => refreshTasks(), page);
      return;
    }
    if (page === 'schedules') {
      this._safeRefresh(() => loadSchedules(), 'schedules');
      return;
    }
    if (page === 'logs') {
      this._safeRefresh(() => loadLogs(), 'logs');
      return;
    }
    if (page !== 'system') return;

    if (store.state._systemTab === 'schedules') {
      this._safeRefresh(() => loadSchedules({ updateFormItems: !isScheduleFormEditing() }), 'system schedules');
    } else if (store.state._systemTab === 'config') {
      this._safeRefresh(() => refreshConfigAfterReconnect(), 'config');
    } else if (store.state._systemTab === 'cookies') {
      this._safeRefresh(() => refreshCookiesAfterReconnect(), 'cookies');
    }
  },

  _safeRefresh(fn, label) {
    try {
      const result = fn();
      if (result && typeof result.catch === 'function') {
        result.catch(err => console.warn(`[SSE] reconnect refresh failed (${label}):`, err));
      }
    } catch (err) {
      console.warn(`[SSE] reconnect refresh failed (${label}):`, err);
    }
  }
};

// ============================================
// Toast Notifications
// ============================================
const toast = {
  container: document.getElementById('toastContainer'),
  maxToasts: 3,
  
  show(message, type = 'success', title = '') {
    if (!this.container) return;
    const existingToasts = this.container.querySelectorAll('.toast');
    
    // Dedup: skip if same message already visible
    for (const existing of existingToasts) {
      const msgEl = existing.querySelector('.toast-message');
      if (msgEl && msgEl.textContent === message) return;
    }
    
    if (existingToasts.length >= this.maxToasts) {
      // 移除最旧的消息（第一个）
      existingToasts[0].remove();
    }
    
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    
    const icons = { success: '✓', error: '✕', warning: '⚠' };
    const titles = { success: '成功', error: '错误', warning: '警告' };
    const safeTitle = escapeHtml(title || titles[type] || '');
    const safeMessage = escapeHtml(message || '');
    
    el.innerHTML = `
      <span class="toast-icon">${icons[type]}</span>
      <div class="toast-content">
        <div class="toast-title">${safeTitle}</div>
        <div class="toast-message">${safeMessage}</div>
      </div>
      <span class="toast-close">✕</span>
    `;
    
    el.querySelector('.toast-close').onclick = () => el.remove();
    this.container.appendChild(el);
    
    setTimeout(() => el.remove(), 5000);
  }
};

// ============================================
// Drawer
// ============================================
const drawer = {
  el: document.getElementById('drawer'),
  overlay: document.getElementById('drawerOverlay'),
  title: document.getElementById('drawerTitle'),
  body: document.getElementById('drawerBody'),
  footer: document.getElementById('drawerFooter'),
  
  open(title, content, footer = '') {
    if (!this.el || !this.title || !this.body || !this.footer || !this.overlay) return;
    delete this._taskId;
    this.title.textContent = title;
    this.body.innerHTML = content;
    this.footer.innerHTML = footer;
    this.el.classList.add('open');
    this.overlay.classList.add('open');
    document.body.style.overflow = 'hidden';
  },
  
  close() {
    this.el.classList.remove('open');
    this.overlay.classList.remove('open');
    document.body.style.overflow = '';
  }
};

document.getElementById('drawerClose').onclick = () => drawer.close();
document.getElementById('drawerOverlay').onclick = () => drawer.close();

// ============================================
// Page Renderers
// ============================================
const pages = {
  // Overview Page
  overview() {
    const { health, tasks } = store.state;
    
    const taskStats = { queued: 0, running: 0, completed: 0, failed: 0, cancelled: 0 };
    tasks.forEach(t => { if (taskStats[t.status] !== undefined) taskStats[t.status]++; });
    
    const recentTasks = tasks.slice(0, 5);
    
    return `
      <div class="page-container">
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon" style="color: var(--success);">●</div>
          <div class="stat-content">
            <div class="stat-value">${health ? (health.status === 'ok' ? '健康' : '异常') : '检查中'}</div>
            <div class="stat-label">系统状态 ${health ? health.version : ''}</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon" style="color: var(--info);">🚀</div>
          <div class="stat-content">
            <div class="stat-value" data-overview-stat="running">${taskStats.running}</div>
            <div class="stat-label">运行中任务</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon" style="color: var(--success);">✓</div>
          <div class="stat-content">
            <div class="stat-value" data-overview-stat="completed">${taskStats.completed}</div>
            <div class="stat-label">已完成任务</div>
          </div>
        </div>
      </div>

      <div class="card" style="margin-bottom: var(--space-6); flex-shrink: 0">
        <div class="card-header">
          <div>
            <div class="card-title">⚡ 快速下载</div>
            <div class="card-subtitle">输入 Twitter 用户名或链接快速创建下载任务</div>
          </div>
        </div>
        <div class="card-body">
          <div class="flex gap-3" style="flex-wrap: wrap;">
            <input type="text" class="form-input" id="quickDownloadInput"
              placeholder="输入用户名，如: elonmusk 或 https://twitter.com/elonmusk"
              style="flex: 1; min-width: 280px;">
            <button class="btn btn-primary" onclick="handleQuickDownload()">粘贴并创建任务</button>
          </div>
          <div class="text-sm text-tertiary mt-4">
            支持格式: twitter.com/username | x.com/username | twitter.com/i/lists/123 | @username
          </div>
        </div>
      </div>

      <div class="card card-fill">
        <div class="card-header">
          <div class="card-title">最近任务</div>
          <button class="btn btn-ghost btn-sm" onclick="navigateTo('tasks')">查看全部 →</button>
        </div>
        <div class="card-body card-body-scroll">
          ${recentTasks.length === 0 ? `
            <div class="empty-state">
              <div class="empty-icon">📋</div>
              <div class="empty-title">暂无任务</div>
              <div class="empty-desc">创建一个新任务开始下载 Twitter 媒体文件</div>
            </div>
          ` : `
            <div class="task-list overview-tasks-list">
              ${recentTasks.map(t => renderTaskItem(t)).join('')}
            </div>
          `}
        </div>
      </div>
      </div>
    `;
  },
  
  // Tasks Page
  tasks() {
    const { tasks } = store.state;
    
    return `
      <div class="tasks-layout">
        <div>
          <div class="card card-fill">
            <div class="card-header">
              <div class="card-title">创建新任务</div>
            </div>
            <div class="card-body" style="flex:1;overflow-y:auto">
              <div class="tabs">
                <div class="tab active" data-task-tab="user">用户</div>
                <div class="tab" data-task-tab="list">列表</div>
                <div class="tab" data-task-tab="following">关注</div>
                <div class="tab" data-task-tab="batch">批量</div>
                <div class="tab" data-task-tab="jsonfile"><span>JSON</span><span>文件</span></div>
                <div class="tab" data-task-tab="jsonfolder"><span>JSON</span><span>文件夹</span></div>
                <div class="tab" data-task-tab="mark">标记</div>
              </div>

              <div id="taskFormContainer">
                ${renderTaskForm('user')}
              </div>
            </div>
          </div>
        </div>

        <div>
          <div class="card card-fill">
            <div class="card-header">
              <div>
                <div class="card-title">任务列表</div>
                <div class="card-subtitle" data-task-count-subtitle>共 ${tasks.length} 个任务</div>
              </div>
            </div>
            <div class="toolbar">
              <div class="toolbar-left">
                <select class="form-select" style="width: 100px;" id="taskFilter" onchange="updateSearchState('taskFilter',null,this.value);filterTasks()">
                  <option value="all">全部状态</option>
                  <option value="running">运行中</option>
                  <option value="queued">排队中</option>
                  <option value="completed">已完成</option>
                  <option value="failed">失败</option>
                </select>
                <input type="text" class="form-input search-input" id="taskSearch" placeholder="搜索任务..." oninput="updateSearchState('taskSearch',null,this.value);filterTasks()">
              </div>
              <div class="toolbar-right">
                <button class="btn btn-secondary btn-sm" onclick="cancelQueuedTasks()">取消排队中任务</button>
              </div>
            </div>
            <div class="card-body card-body-scroll">
              <div class="${tasks.length === 0 ? 'empty-state' : 'task-list'}" id="taskListContainer">
                ${tasks.length === 0 ? `
                  <div class="empty-icon">🚀</div>
                  <div class="empty-title">暂无任务</div>
                  <div class="empty-desc">在左侧创建一个新任务开始下载</div>
                ` : `
                  ${tasks.map(t => renderTaskItem(t)).join('')}
                `}
              </div>
            </div>
          </div>
        </div>
      </div>
    `;
  },
  
  // Data Page
  data() {
    const { dataSubPage, dbData, dbPagination, dbSort, dbSearch } = store.state;
    
    const dataMap = {
      users: { title: 'Users', data: dbData.users?.data || [], count: dbData.users?.total || 0 },
      lists: { title: 'Lists', data: dbData.lists?.data || [], count: dbData.lists?.total || 0 },
      entities: { title: 'User Entities', data: dbData.entities?.data || [], count: dbData.entities?.total || 0 },
      listEntities: { title: 'List Entities', data: dbData.listEntities?.data || [], count: dbData.listEntities?.total || 0 },
      userLinks: { title: 'User Links', data: dbData.userLinks?.data || [], count: dbData.userLinks?.total || 0 },
      previousNames: { title: 'Previous Names', data: dbData.previousNames?.data || [], count: dbData.previousNames?.total || 0 }
    };
    
    const current = dataMap[dataSubPage];
    const pagination = dbPagination[dataSubPage] || { page: 1, pageSize: 200, totalPages: 1 };
    const sort = dbSort[dataSubPage] || { sortBy: 'id', sortOrder: 'desc' };
    
    return `
      <div class="card card-page">
        <div class="card-header">
          <div>
            <div class="tabs" style="margin: 0; border: none;">
              <div class="tab ${dataSubPage === 'users' ? 'active' : ''}" onclick="setDataSubPage('users')">Users</div>
              <div class="tab ${dataSubPage === 'lists' ? 'active' : ''}" onclick="setDataSubPage('lists')">Lists</div>
              <div class="tab ${dataSubPage === 'entities' ? 'active' : ''}" onclick="setDataSubPage('entities')">User Entities</div>
              <div class="tab ${dataSubPage === 'listEntities' ? 'active' : ''}" onclick="setDataSubPage('listEntities')">List Entities</div>
              <div class="tab ${dataSubPage === 'userLinks' ? 'active' : ''}" onclick="setDataSubPage('userLinks')">User Links</div>
              <div class="tab ${dataSubPage === 'previousNames' ? 'active' : ''}" onclick="setDataSubPage('previousNames')">Previous Names</div>
            </div>
          </div>
          <div class="flex gap-2 items-center">
            <input type="text" class="form-input search-input" id="dbSearchInput"
              placeholder="搜索..." oninput="updateSearchState('dbSearch',store.state.dataSubPage,this.value)">
            <button class="btn btn-ghost btn-icon" onclick="searchDB()">🔍</button>
          </div>
        </div>

        <div class="card-body card-body-scroll">
          <div class="table-scroll-container" id="dataTableContainer">
            ${renderDBTable(dataSubPage, current.data, sort)}
          </div>
          <div id="dataMobileCards">${renderDBMobileCards(dataSubPage, current.data)}</div>
        </div>

        <div class="pagination" id="dataPagination">
          <div class="pagination-info" id="dataPaginationInfo">
            显示 ${current.data.length} / ${current.count} 条记录 
            (第 ${pagination.page} / ${pagination.totalPages} 页)
          </div>
          <div class="pagination-controls">
            <button class="page-btn" onclick="changeDBPage(-1)" ${pagination.page <= 1 ? 'disabled' : ''}>←</button>
            ${renderPageNumbers(pagination.page, pagination.totalPages)}
            <button class="page-btn" onclick="changeDBPage(1)" ${pagination.page >= pagination.totalPages ? 'disabled' : ''}>→</button>
          </div>
        </div>
      </div>
    `;
  },

  schedules() {
    const { _schedules, _scheduleExists, _schedulerRunning } = store.state;

    if (_schedules === null) {
      return `
        <div class="page-container">
          <div class="card">
            <div class="card-header"><div><div class="card-title">定时下载任务</div></div></div>
            <div class="card-body">
              <div class="empty-state">
                <div class="skeleton" style="width:64px;height:64px;border-radius:12px;margin-bottom:16px"></div>
                <div class="empty-title">加载中...</div>
                <div class="empty-desc">正在加载定时任务配置</div>
              </div>
            </div>
          </div>
        </div>
      `;
    }

    const schedulerBanner = !_schedulerRunning
      ? `<div class="alert alert-warning" style="margin-bottom:var(--space-3)">⚠️ 调度器未启动，定时任务不会自动执行。请在「定时任务」页面中添加并启用规则后重载配置。</div>`
      : '';

    return `
      <div class="page-container">
        <div style="flex-shrink:0">${schedulerBanner}</div>
        ${renderScheduleTable(_schedules, _scheduleExists)}
      </div>
    `;
  },

  // System Page
  system() {
    return `
      <div class="page-container">
        <div class="system-tab-bar">
          <div class="system-tabs" style="margin:0">
            <div class="tab ${store.state._systemTab === 'config' ? 'active' : ''}" data-tab="config" onclick="setSystemTab('config')">⚙️ 配置编辑</div>
            <div class="tab ${store.state._systemTab === 'cookies' ? 'active' : ''}" data-tab="cookies" onclick="setSystemTab('cookies')">🍪 额外账户</div>
            <div class="tab ${store.state._systemTab === 'schedules' ? 'active' : ''}" data-tab="schedules" onclick="setSystemTab('schedules')">⏰ 任务配置</div>
          </div>
          <button class="btn btn-danger btn-sm" onclick="shutdownServer()">⏻ 关闭服务器</button>
        </div>

        <div id="systemConfigPanel" class="system-panel system-panel-scroll" style="${store.state._systemTab === 'config' ? '' : 'display:none'}">
          ${renderConfigEditor()}
        </div>

        <div id="systemCookiesPanel" class="system-panel system-panel-scroll" style="${store.state._systemTab === 'cookies' ? '' : 'display:none'}">
          ${renderCookiesEditor()}
        </div>

        <div id="systemSchedulesPanel" class="system-panel system-panel-scroll" style="${store.state._systemTab === 'schedules' ? '' : 'display:none'}">
          ${renderScheduleViewer()}
        </div>
      </div>
    `;
  },

  logs() {
    return renderLogViewer();
  }
};

// ============================================
// Module-level state bag (replaces top-level let/const)
// ============================================
const _state = {};

// ============================================
// Helper Functions
// ============================================

function getStageText(stage) {
  const stageMap = {
    'preparing': ' · 准备中',
    'syncing': ' · 同步列表',
    'downloading': ' · 下载中',
    'retrying': ' · 重试中',
    'profile': ' · 下载资料',
    'profile_warning': ' · 资料下载异常',
    'marking': ' · 标记中',
    'completed': ''
  };
  return stageMap[stage] || (stage ? ` · ${stage}` : '');
}

function getTaskProgressPercent(task) {
  if (task.status === 'completed') return 100;

  const progress = task.progress || {};
  const total = progress.total || 0;
  const completed = progress.completed || 0;
  const ratio = total > 0 ? Math.min(completed / total, 1) : 0;

  if (task.status === 'failed' || task.status === 'cancelled') {
    return total > 0 ? Math.round(ratio * 100) : 0;
  }

  switch (progress.stage) {
    case 'syncing':
      return 5;
    case 'preparing':
      return 10;
    case 'downloading':
      return Math.round(10 + ratio * 70);
    case 'retrying':
      return Math.round(80 + ratio * 10);
    case 'profile':
      return total > 0 ? Math.round(90 + ratio * 9) : 90;
    case 'profile_warning':
      return 99;
    case 'marking':
      return total > 0 ? Math.round(10 + ratio * 85) : 10;
    default:
      return 0;
  }
}

function getTaskTarget(task) {
  const data = task.data || {};

  if (data.screen_name) {
    return `@${data.screen_name}`;
  }
  if (data.list_id) {
    return `List ${data.list_id}`;
  }

  const parts = [];
  if (Array.isArray(data.users) && data.users.length) {
    parts.push(`${data.users.length} 用户`);
  }
  if (Array.isArray(data.lists) && data.lists.length) {
    parts.push(`${data.lists.length} 列表`);
  }
  if (Array.isArray(data.following_names) && data.following_names.length) {
    parts.push(`${data.following_names.length} 关注源`);
  }

  return parts.length ? parts.join(' · ') : 'Unknown';
}

function getOptionalTimestamp(inputId) {
  const input = document.getElementById(inputId);
  const value = input?.value?.trim() || '';
  if (!value) return null;

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    throw new Error('请输入有效的标记时间');
  }
  return date.toISOString();
}

function renderTaskItem(task) {
  const statusMap = {
    queued: { tag: 'tag-queued', text: '排队' },
    running: { tag: 'tag-running', text: '运行' },
    completed: { tag: 'tag-completed', text: '完成' },
    failed: { tag: 'tag-failed', text: '失败' },
    cancelled: { tag: 'tag-cancelled', text: '取消' }
  };
  
  const status = statusMap[task.status] || statusMap.queued;
  const pct = getTaskProgressPercent(task);

  const stageText = task.progress?.stage ? escapeHtml(getStageText(task.progress.stage)) : '';
  const currentText = task.progress?.current ? ` · ${escapeHtml(task.progress.current)}` : '';

  const target = escapeHtml(getTaskTarget(task));

  return `
    <div class="task-item" data-task-id="${escapeAttr(task.task_id)}">
      <div class="task-info">
        <div class="task-title">${escapeHtml(task.type)} - ${target}</div>
        <div class="task-meta">
          <span class="tag ${status.tag}">${status.text}</span>
          <span>ID: ${escapeHtml(task.task_id)}</span>
          <span>${task.created_at ? new Date(task.created_at).toLocaleString() : '-'}</span>
        </div>
      </div>
      <div class="task-progress">
        <div class="progress-bar">
          <div class="progress-fill" style="width: ${pct}%"></div>
        </div>
        <div class="task-progress-text">${pct}%${stageText}${currentText}</div>
      </div>
      <div class="task-actions">
        ${task.status === 'running' || task.status === 'queued' ?
          `<button class="btn btn-danger btn-sm" data-action="cancel">取消</button>` :
          `<button class="btn btn-ghost btn-sm" data-action="detail">详情</button>`
        }
      </div>
    </div>
  `;
}

function renderTaskForm(type) {
  const forms = {
    user: `
      <div class="form-group">
        <label class="form-label">Screen Name</label>
        <input type="text" class="form-input" id="userScreenName" placeholder="例如: elonmusk">
      </div>
      <div class="form-group">
        <label class="form-checkbox">
          <input type="checkbox" id="userAutoFollow"> 自动申请受保护账号
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="userFollowMembers"> 下载时关注目标/成员
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="userSkipProfile"> SkipProfile
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="userNoRetry"> NoRetry
        </label>
      </div>
      <div class="flex gap-3">
        <button class="btn btn-primary" onclick="createUserTask()">创建下载任务</button>
        <button class="btn btn-secondary" onclick="createProfileTask()">仅下载 Profile</button>
      </div>
    `,
    list: `
      <div class="form-group">
        <label class="form-label">List ID</label>
        <input type="text" inputmode="numeric" pattern="[0-9]*" class="form-input" id="listId" placeholder="例如: 123456789">
      </div>
      <div class="form-group">
        <label class="form-checkbox">
          <input type="checkbox" id="listAutoFollow"> 自动申请受保护账号
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="listFollowMembers"> 下载时关注目标/成员
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="listSkipProfile"> SkipProfile
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="listNoRetry"> NoRetry
        </label>
      </div>
      <div class="flex gap-3">
        <button class="btn btn-primary" onclick="createListTask()">创建下载任务</button>
        <button class="btn btn-secondary" onclick="createListProfileTask()">仅下载 Profile</button>
      </div>
    `,
    following: `
      <div class="form-group">
        <label class="form-label">Screen Name</label>
        <input type="text" class="form-input" id="followingScreenName" placeholder="例如: elonmusk">
      </div>
      <div class="form-group">
        <label class="form-checkbox">
          <input type="checkbox" id="followingAutoFollow"> 自动申请受保护账号
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="followingFollowMembers"> 下载时关注目标/成员
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="followingSkipProfile"> SkipProfile
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="followingNoRetry"> NoRetry
        </label>
      </div>
      <div class="flex gap-3">
        <button class="btn btn-primary" onclick="createFollowingTask()">创建关注下载任务</button>
      </div>
    `,
    mark: `
      <div class="form-group">
        <label class="form-label">用户 Screen Name（每行一个）</label>
        <textarea class="form-textarea" id="markUsers" placeholder="elonmusk\njack" rows="3"></textarea>
      </div>
      <div class="form-group">
        <label class="form-label">List IDs（每行一个）</label>
        <textarea class="form-textarea" id="markLists" placeholder="123456789\n987654321" rows="3"></textarea>
      </div>
      <div class="form-group">
        <label class="form-label">Following 用户（每行一个）</label>
        <textarea class="form-textarea" id="markFollowingNames" placeholder="user_a\nuser_b" rows="3"></textarea>
      </div>
      <div class="form-group">
        <label class="form-label">标记时间（可选）</label>
        <input type="datetime-local" class="form-input" id="markTimestamp" placeholder="选择日期和时间">
        <div class="text-sm text-tertiary mt-2">留空则使用服务器当前时间。每个输入目标会创建独立标记任务。</div>
      </div>
      <button class="btn btn-primary" onclick="createMarkTask()">创建标记任务</button>
    `,
    batch: `
      <div class="form-group">
        <label class="form-label">用户列表（每行一个）</label>
        <textarea class="form-textarea" id="batchUsers" placeholder="user1\nuser2\nuser3" rows="3"></textarea>
      </div>
      <div class="form-group">
        <label class="form-label">List IDs（每行一个）</label>
        <textarea class="form-textarea" id="batchLists" placeholder="123\n456\n789" rows="3"></textarea>
      </div>
      <div class="form-group">
        <label class="form-label">Following 用户（每行一个）</label>
        <textarea class="form-textarea" id="batchFollowingNames" placeholder="user_a\nuser_b" rows="3"></textarea>
        <div class="text-sm text-tertiary mt-2">将这些用户的 Following 加入批量下载目标</div>
      </div>
      <div class="form-group">
        <label class="form-checkbox">
          <input type="checkbox" id="batchAutoFollow"> 自动申请受保护账号
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="batchFollowMembers"> 下载时关注目标/成员
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="batchSkipProfile"> SkipProfile
        </label>
        <label class="form-checkbox">
          <input type="checkbox" id="batchNoRetry"> NoRetry
        </label>
      </div>
      <button class="btn btn-primary" onclick="createBatchTask()">创建批量任务</button>
    `,
    jsonfile: `
  <div class="form-group">
    <label class="form-label">上传第三方工具导出的 JSON 文件</label>
    <input type="file" class="form-input" id="jsonFileUpload" accept=".json,application/json" multiple>
  </div>
  <div class="text-sm text-tertiary mt-2">
    支持多选 .json 文件。未选择文件时，可改用下面的服务端路径模式。
  </div>
  <div class="form-group mt-3">
    <label class="form-label">高级：服务端 JSON 文件路径（每行一个）</label>
    <textarea class="form-textarea" id="jsonFilePaths" placeholder="/path/to/twitter-followers-123.json\n/path/to/more.json" rows="3"></textarea>
  </div>
  <div class="text-sm text-tertiary mt-2">
    支持格式: 第三方工具导出的Twitter推文搜索结果JSON（含推文列表、media数组、metadata字段）
  </div>
  <div class="form-group mt-3">
    <label class="form-checkbox">
      <input type="checkbox" id="jsonFileNoRetry"> NoRetry
    </label>
  </div>
  <button class="btn btn-primary" onclick="createJsonFileTask()">创建 JSON 文件任务</button>
`,
jsonfolder: `
  <div class="form-group">
    <label class="form-label">上传 LoongTweet JSON 文件</label>
    <input type="file" class="form-input" id="jsonFolderUpload" accept=".json,application/json" multiple>
  </div>
  <div class="text-sm text-tertiary mt-2">
    直接选择一个或多个 .loongtweet 生成的 JSON 文件。未选择文件时，可改用下面的服务端路径模式。
  </div>
  <div class="form-group mt-3">
    <label class="form-label">高级：服务端 .loongtweet 文件夹路径（每行一个）</label>
    <textarea class="form-textarea" id="jsonFolderPath" placeholder="/path/to/.loongtweet\n/path/to/another/.loongtweet" rows="3"></textarea>
  </div>
  <div class="text-sm text-tertiary mt-2">
    从 TMD 生成的 .loongtweet 目录下载推文媒体文件（仅下载媒体，不保存元数据）
  </div>
  <div class="form-group mt-3">
    <label class="form-checkbox">
      <input type="checkbox" id="jsonFolderNoRetry"> NoRetry
    </label>
  </div>
  <button class="btn btn-primary" onclick="createJsonFolderTask()">创建 LoongTweet 任务</button>
`
  };
  return forms[type] || forms.user;
}

// Shared helpers for database table rendering
function sortIcon(sort, field) {
  if (sort.sortBy !== field) return '<span class="sort-icon">↕</span>';
  return sort.sortOrder === 'asc'
    ? '<span class="sort-icon sort-active">↑</span>'
    : '<span class="sort-icon sort-active">↓</span>';
}

function sortableHeader(sort, field, label) {
  return `
    <th data-sort-field="${escapeAttr(field)}" class="${sort.sortBy === field ? 'sort-active' : ''}" onclick="sortDB(this.dataset.sortField)">
      ${label} ${sortIcon(sort, field)}
    </th>
  `;
}

function renderActionButtons(type, item) {
  const idStr = String(item.id);
  return `
    <div class="flex gap-2">
      <button class="btn btn-ghost btn-sm" data-db-type="${escapeAttr(type)}" data-db-id="${escapeAttr(idStr)}" onclick="editDBItem(this.dataset.dbType, this.dataset.dbId)">✏️</button>
      <button class="btn btn-danger btn-sm" data-db-type="${escapeAttr(type)}" data-db-id="${escapeAttr(idStr)}" onclick="deleteDBItem(this.dataset.dbType, this.dataset.dbId)">🗑️</button>
    </div>
  `;
}

function renderDBUsersTable(type, data, sort) {
  const rows = data.map(item => `<tr>
    <td>${escapeHtml(item.id)}</td>
    <td>@${escapeHtml(item.screen_name)}</td>
    <td>${escapeHtml(item.name)}</td>
    <td>${item.protected ? '🔒' : '🔓'}</td>
    <td>${item.is_accessible ? '✅' : '❌'}</td>
    <td>${escapeHtml(item.friends_count)}</td>
    <td>${renderActionButtons(type, item)}</td>
  </tr>`).join('');
  return `
    <table class="data-table">
      <thead>
        <tr>
          ${sortableHeader(sort, 'id', 'ID')}
          ${sortableHeader(sort, 'screen_name', 'Screen Name')}
          ${sortableHeader(sort, 'name', 'Name')}
          <th>Protected</th>
          <th>Accessible</th>
          ${sortableHeader(sort, 'friends_count', 'Friends')}
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderDBListsTable(type, data, sort) {
  const rows = data.map(item => `<tr>
    <td>${escapeHtml(item.id)}</td>
    <td>${escapeHtml(item.name)}</td>
    <td>${escapeHtml(item.owner_user_id)}</td>
    <td>${renderActionButtons(type, item)}</td>
  </tr>`).join('');
  return `
    <table class="data-table">
      <thead>
        <tr>
          ${sortableHeader(sort, 'id', 'ID')}
          ${sortableHeader(sort, 'name', 'Name')}
          ${sortableHeader(sort, 'owner_id', 'Owner ID')}
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderDBEntitiesTable(type, data, sort) {
  const rows = data.map(item => `<tr>
    <td>${escapeHtml(item.id)}</td>
    <td>${escapeHtml(item.user_id)}</td>
    <td>${escapeHtml(item.name)}</td>
    <td>${escapeHtml(item.latest_release_time || '-')}</td>
    <td>${escapeHtml(item.media_count || '-')}</td>
    <td>${renderActionButtons(type, item)}</td>
  </tr>`).join('');
  return `
    <table class="data-table">
      <thead>
        <tr>
          ${sortableHeader(sort, 'id', 'ID')}
          ${sortableHeader(sort, 'user_id', 'User ID')}
          ${sortableHeader(sort, 'name', 'Name')}
          ${sortableHeader(sort, 'latest_release_time', 'Latest Release')}
          ${sortableHeader(sort, 'media_count', 'Media Count')}
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderDBListEntitiesTable(type, data, sort) {
  const rows = data.map(item => `<tr>
    <td>${escapeHtml(item.id)}</td>
    <td>${escapeHtml(item.lst_id)}</td>
    <td>${escapeHtml(item.name)}</td>
    <td>${escapeHtml(item.parent_dir)}</td>
    <td>${renderActionButtons(type, item)}</td>
  </tr>`).join('');
  return `
    <table class="data-table">
      <thead>
        <tr>
          ${sortableHeader(sort, 'id', 'ID')}
          ${sortableHeader(sort, 'lst_id', 'List ID')}
          ${sortableHeader(sort, 'name', 'Name')}
          <th>Parent Dir</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderDBPreviousNamesTable(type, data, sort) {
  const rows = data.map(item => {
    const currentLabel = item.current_screen_name ? `@${escapeHtml(item.current_screen_name)}` : escapeHtml(item.user_id || '');
    return `<tr>
      <td><a href="javascript:void(0)" onclick="filterPreviousNamesByUser('${escapeAttr(item.user_id || '')}')">${currentLabel}</a></td>
      <td>@${escapeHtml(item.screen_name)}</td>
      <td>${escapeHtml(item.name)}</td>
      <td>${escapeHtml(item.record_date || '-')}</td>
    </tr>`;
  }).join('');
  return `
    <table class="data-table">
      <thead>
        <tr>
          ${sortableHeader(sort, 'current_screen_name', 'Current User')}
          ${sortableHeader(sort, 'screen_name', 'Previous @Handle')}
          ${sortableHeader(sort, 'name', 'Previous Name')}
          ${sortableHeader(sort, 'record_date', 'Date')}
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderDBDefaultTable(type, data, sort) {
  const rows = data.map(item => `<tr>
    <td>${escapeHtml(item.id)}</td>
    <td>${escapeHtml(item.user_id)}</td>
    <td>${escapeHtml(item.name)}</td>
    <td>${escapeHtml(item.parent_lst_entity_id)}</td>
    <td>${renderActionButtons(type, item)}</td>
  </tr>`).join('');
  return `
    <table class="data-table">
      <thead>
        <tr>
          ${sortableHeader(sort, 'id', 'ID')}
          ${sortableHeader(sort, 'user_id', 'User ID')}
          ${sortableHeader(sort, 'name', 'Name')}
          <th>Parent Entity</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

// Database Table Renderer with sorting and actions
function renderDBTable(type, data, sort) {
  if (!data || data.length === 0) {
    return `
      <div class="empty-state">
        <div class="empty-icon">📊</div>
        <div class="empty-title">暂无数据</div>
        <div class="empty-desc">数据库中还没有记录</div>
      </div>
    `;
  }

  switch (type) {
    case 'users': return renderDBUsersTable(type, data, sort);
    case 'lists': return renderDBListsTable(type, data, sort);
    case 'entities': return renderDBEntitiesTable(type, data, sort);
    case 'listEntities': return renderDBListEntitiesTable(type, data, sort);
    case 'previousNames': return renderDBPreviousNamesTable(type, data, sort);
    default: return renderDBDefaultTable(type, data, sort);
  }
}

function renderDBMobileCards(type, data) {
  if (!data || data.length === 0) return '';

  const cards = data.map(item => {
    if (type === 'users') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">@${escapeHtml(item.screen_name)}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">${escapeHtml(item.name)}</div>
          <div style="display: flex; gap: var(--space-4); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <span>${item.protected ? '🔒 Protected' : '🔓 Public'}</span>
            <span>${item.is_accessible ? '✅ Accessible' : '❌ Not Accessible'}</span>
          </div>
          <div style="font-size: var(--text-sm); margin-bottom: var(--space-2);">Friends: ${escapeHtml(item.friends_count)}</div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else if (type === 'lists') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${escapeHtml(item.name)}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${escapeHtml(item.id)}</div>
            <div>Owner: ${escapeHtml(item.owner_user_id)}</div>
          </div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else if (type === 'entities') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${escapeHtml(item.name)}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${escapeHtml(item.id)}</div>
            <div>User ID: ${escapeHtml(item.user_id)}</div>
            <div>Media: ${escapeHtml(item.media_count || 0)}</div>
          </div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else if (type === 'listEntities') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${escapeHtml(item.name)}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${escapeHtml(item.id)}</div>
            <div>List ID: ${escapeHtml(item.lst_id)}</div>
            <div>Dir: ${escapeHtml(item.parent_dir)}</div>
          </div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else if (type === 'previousNames') {
      const currentLabel = item.current_screen_name ? `@${item.current_screen_name}` : (item.user_id || '');
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${escapeHtml(currentLabel)}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>Previous: @${escapeHtml(item.screen_name || '')} (${escapeHtml(item.name || '')})</div>
            <div>Date: ${escapeHtml(item.record_date || '-')}</div>
          </div>
        </div>
      `;
    } else {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${escapeHtml(item.name)}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${escapeHtml(item.id)}</div>
            <div>User ID: ${escapeHtml(item.user_id)}</div>
            <div>Entity: ${escapeHtml(item.parent_lst_entity_id)}</div>
          </div>
        </div>
      `;
    }
  }).join('');

  return `<div class="mobile-card-list">${cards}</div>`;
}

function renderPageNumbers(currentPage, totalPages, onClickHandler = 'goToDBPage') {
  if (totalPages <= 1) return `<button class="page-btn active">1</button>`;

  let pages = [];
  const maxVisible = 5;

  if (totalPages <= maxVisible) {
    for (let i = 1; i <= totalPages; i++) {
      pages.push(i);
    }
  } else {
    if (currentPage <= 3) {
      pages = [1, 2, 3, 4, '...', totalPages];
    } else if (currentPage >= totalPages - 2) {
      pages = [1, '...', totalPages - 3, totalPages - 2, totalPages - 1, totalPages];
    } else {
      pages = [1, '...', currentPage - 1, currentPage, currentPage + 1, '...', totalPages];
    }
  }

  return pages.map(p => {
    if (p === '...') return `<span class="page-btn" style="cursor: default;">...</span>`;
    return `<button class="page-btn ${p === currentPage ? 'active' : ''}" onclick="${onClickHandler}(${p})">${p}</button>`;
  }).join('');
}

// ============================================
// Database Actions
// ============================================
async function refreshDBData() {
  const { dataSubPage, dbPagination, dbSort, dbSearch } = store.state;
  const pagination = dbPagination[dataSubPage];
  const sort = dbSort[dataSubPage];
  const search = dbSearch[dataSubPage];

  const params = new URLSearchParams();
  params.append('page', pagination.page);
  params.append('pageSize', pagination.pageSize);
  params.append('sortBy', sort.sortBy);
  params.append('sortOrder', sort.sortOrder);
  if (search) params.append('q', search);

  try {
    let response;
    switch (dataSubPage) {
      case 'users':
        response = await api.getDBUsers(params.toString());
        break;
      case 'lists':
        response = await api.getDBLists(params.toString());
        break;
      case 'entities':
        response = await api.getDBUserEntities(params.toString());
        break;
      case 'listEntities':
        response = await api.getDBListEntities(params.toString());
        break;
      case 'userLinks':
        response = await api.getDBUserLinks(params.toString());
        break;
      case 'previousNames':
        if (store.state._prevNameUserIdFilter) {
          params.append('userId', store.state._prevNameUserIdFilter);
        }
        response = await api.getDBPreviousNames(params.toString());
        break;
    }

    if (response) {
      const data = response || {};
      store.setState({
        dbData: {
          ...store.state.dbData,
          [dataSubPage]: {
            data: data.data || [],
            total: data.total || 0,
            page: data.page || 1,
            pageSize: data.pageSize || 200
          }
        },
        dbPagination: {
          ...store.state.dbPagination,
          [dataSubPage]: {
            page: data.page || 1,
            pageSize: data.pageSize || 200,
            totalPages: data.totalPages || 1
          }
        }
      });
    } else {
      toast.show('获取数据失败', 'error');
    }
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

function changeDBPage(delta) {
  const { dataSubPage, dbPagination } = store.state;
  const current = dbPagination[dataSubPage];
  const newPage = current.page + delta;

  if (newPage >= 1 && newPage <= current.totalPages) {
    store.setState({
      dbPagination: {
        ...dbPagination,
        [dataSubPage]: { ...current, page: newPage }
      }
    });
    refreshDBData();
  }
}

function goToDBPage(page) {
  const { dataSubPage, dbPagination } = store.state;
  const pag = dbPagination[dataSubPage];
  if (!pag) return;
  if (page < 1) page = 1;
  if (pag.totalPages && page > pag.totalPages) page = pag.totalPages;
  store.setState({
    dbPagination: {
      ...dbPagination,
      [dataSubPage]: { ...pag, page }
    }
  });
  refreshDBData();
}

function sortDB(field) {
  const { dataSubPage, dbSort } = store.state;
  const current = dbSort[dataSubPage];

  let newOrder = 'asc';
  if (current.sortBy === field && current.sortOrder === 'asc') {
    newOrder = 'desc';
  }

  store.setState({
    dbSort: {
      ...dbSort,
      [dataSubPage]: { sortBy: field, sortOrder: newOrder }
    }
  });
  refreshDBData();
}

function searchDB() {
  store.setState({
    dbPagination: {
      ...store.state.dbPagination,
      [store.state.dataSubPage]: { ...store.state.dbPagination[store.state.dataSubPage], page: 1 }
    },
    _prevNameUserIdFilter: ''
  });
  refreshDBData();
}

function filterPreviousNamesByUser(userId) {
  if (!userId) return;
  store.setState({
    dataSubPage: 'previousNames',
    dbPagination: {
      ...store.state.dbPagination,
      previousNames: { ...store.state.dbPagination.previousNames, page: 1 }
    },
    _prevNameUserIdFilter: userId
  });
  refreshDBData();
}

async function editDBItem(type, id) {
  try {
    let item;
    switch (type) {
      case 'users':
        item = await api.getDBUser(id);
        break;
      case 'lists':
        item = await api.getDBList(id);
        break;
      case 'entities':
        item = await api.getDBUserEntity(id);
        break;
      case 'listEntities':
        item = await api.getDBListEntity(id);
        break;
      case 'userLinks':
        item = await api.getDBUserLink(id);
        break;
      default:
        throw new Error('Unknown type: ' + type);
    }

    if (!item) {
      throw new Error('Failed to load item data');
    }

    // 根据类型构建表单内容
    let content = `
      <div class="form-group">
        <label class="form-label">ID</label>
        <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${escapeHtml(item.id)}</div>
      </div>
    `;

    switch (type) {
      case 'users':
        content += `
          <div class="form-group">
            <label class="form-label">Screen Name</label>
            <input type="text" class="form-input" id="editScreenName" value="${escapeAttr(item.screen_name || '')}">
          </div>
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editName" value="${escapeAttr(item.name || '')}">
          </div>
          <div class="form-group">
            <label class="form-label">Friends Count</label>
            <input type="number" class="form-input" id="editFriendsCount" value="${escapeAttr(item.friends_count || 0)}" min="0" max="999999999">
          </div>
          <div class="form-group">
            <label class="form-checkbox">
              <input type="checkbox" id="editProtected" ${item.protected ? 'checked' : ''}> Protected
            </label>
          </div>
          <div class="form-group">
            <label class="form-checkbox">
              <input type="checkbox" id="editAccessible" ${item.is_accessible ? 'checked' : ''}> Is Accessible
            </label>
          </div>
        `;
        break;
      case 'lists':
        content += `
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editListName" value="${escapeAttr(item.name || '')}">
          </div>
          <div class="form-group">
            <label class="form-label">Owner ID</label>
            <input type="text" class="form-input" id="editListOwnerId" value="${escapeAttr(item.owner_user_id || '')}">
          </div>
        `;
        break;
      case 'entities':
        content += `
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editEntityName" value="${escapeAttr(item.name || '')}">
          </div>
          <div class="form-group">
            <label class="form-label">User ID</label>
            <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${escapeHtml(item.user_id)}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Media Count</label>
            <input type="number" class="form-input" id="editEntityMediaCount" value="${escapeAttr(item.media_count || 0)}">
          </div>
        `;
        break;
      case 'listEntities':
        content += `
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editListEntityName" value="${escapeAttr(item.name || '')}">
          </div>
          <div class="form-group">
            <label class="form-label">List ID</label>
            <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${escapeHtml(item.lst_id)}</div>
          </div>
        `;
        break;
      case 'userLinks':
        content += `
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editUserLinkName" value="${escapeAttr(item.name || '')}">
          </div>
          <div class="form-group">
            <label class="form-label">User ID</label>
            <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${escapeHtml(item.user_id)}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Parent Entity ID</label>
            <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${escapeHtml(item.parent_lst_entity_id)}</div>
          </div>
        `;
        break;
    }

    const footer = `
      <button class="btn btn-secondary" onclick="drawer.close()">取消</button>
      <button class="btn btn-primary" data-db-type="${escapeAttr(type)}" data-db-id="${escapeAttr(id)}" onclick="saveDBItem(this.dataset.dbType, this.dataset.dbId)">保存</button>
    `;

    drawer.open('编辑 ' + type, content, footer);
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function saveDBItem(type, id) {
  const data = {};

  // 根据类型收集数据
  switch (type) {
    case 'users':
      data.screen_name = document.getElementById('editScreenName').value.trim();
      data.name = document.getElementById('editName').value.trim();
      const fcVal = document.getElementById('editFriendsCount').value;
      if (fcVal !== '') {
        data.friends_count = parseInt(fcVal, 10) || 0;
      }
      data.protected = document.getElementById('editProtected').checked;
      data.is_accessible = document.getElementById('editAccessible').checked;
      if (!data.name) return toast.show('Name is required', 'error');
      break;
    case 'lists':
      data.name = document.getElementById('editListName').value.trim();
      data.owner_user_id = document.getElementById('editListOwnerId').value.trim();
      if (!data.name) return toast.show('Name is required', 'error');
      break;
    case 'entities':
      data.name = document.getElementById('editEntityName').value.trim();
      const mcVal = document.getElementById('editEntityMediaCount').value;
      if (mcVal !== '') {
        data.media_count = parseInt(mcVal, 10) || 0;
      }
      if (!data.name) return toast.show('Name is required', 'error');
      break;
    case 'listEntities':
      data.name = document.getElementById('editListEntityName').value.trim();
      if (!data.name) return toast.show('Name is required', 'error');
      break;
    case 'userLinks':
      data.name = document.getElementById('editUserLinkName').value.trim();
      if (!data.name) return toast.show('Name is required', 'error');
      break;
  }

  try {
    switch (type) {
      case 'users':
        await api.updateDBUser(id, data);
        break;
      case 'lists':
        await api.updateDBList(id, data);
        break;
      case 'entities':
        await api.updateDBUserEntity(id, data);
        break;
      case 'listEntities':
        await api.updateDBListEntity(id, data);
        break;
      case 'userLinks':
        await api.updateDBUserLink(id, data);
        break;
      default:
        throw new Error('Unknown type: ' + type);
    }
    drawer.close();
    toast.show('保存成功');
    refreshDBData();
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function deleteDBItem(type, id) {
  if (!confirm(`确定要删除这个${type}记录吗？此操作不可恢复。`)) return;

  try {
    switch (type) {
      case 'users':
        await api.deleteDBUser(id);
        break;
      case 'lists':
        await api.deleteDBList(id);
        break;
      case 'entities':
        await api.deleteDBUserEntity(id);
        break;
      case 'listEntities':
        await api.deleteDBListEntity(id);
        break;
      case 'userLinks':
        await api.deleteDBUserLink(id);
        break;
      default:
        throw new Error('Unknown type: ' + type);
    }
    toast.show('删除成功');
    // 删除操作可能使当前页越界（删除最后一页的最后一条），
    // 先请求一次获取最新数据再刷新
    const { dataSubPage, dbPagination } = store.state;
    const current = dbPagination[dataSubPage];
    const checkParams = new URLSearchParams();
    checkParams.append('page', '1');
    checkParams.append('pageSize', current.pageSize);
    const dataSubPageMap = {
      users: api.getDBUsers,
      lists: api.getDBLists,
      entities: api.getDBUserEntities,
      listEntities: api.getDBListEntities,
      userLinks: api.getDBUserLinks,
      previousNames: api.getDBPreviousNames,
    };
    const fetcher = dataSubPageMap[dataSubPage];
    if (fetcher) {
      const resp = await fetcher(checkParams.toString());
      const total = (resp || {}).total || 0;
      const totalPages = Math.max(1, Math.ceil(total / (current.pageSize || 200)));
      // 当前页超出总页数时回到最后一页
      if (current.page > totalPages) {
        store.setState({
          dbPagination: {
            ...dbPagination,
            [dataSubPage]: { ...current, page: totalPages, totalPages }
          }
        });
      } else {
        store.setState({
          dbPagination: {
            ...dbPagination,
            [dataSubPage]: { ...current, totalPages }
          }
        });
      }
    }
    refreshDBData();
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

// ============================================
// Actions
// ============================================
async function handleQuickDownload() {
  const input = document.getElementById('quickDownloadInput');
  let value = input.value.trim();
  
  if (!value) {
    if (!navigator.clipboard?.readText) {
      return toast.show('当前环境不支持读取剪切板，请手动输入', 'error');
    }
    try {
      value = await navigator.clipboard.readText();
      value = value.trim();
    } catch (err) {
      return toast.show('请输入用户名或链接，或允许读取剪切板', 'error');
    }
    if (!value) {
      return toast.show('剪切板为空，请输入用户名或链接', 'error');
    }
    // await 期间用户可能已手动输入，此时不应覆盖
    const currentVal = input.value.trim();
    if (currentVal && currentVal !== value) {
      value = currentVal;
    } else {
      input.value = value;
    }
  }

  let username = value;
  const listMatch = value.match(/(?:twitter\.com|x\.com)\/i\/lists\/(\d+)/);
  if (listMatch) {
    try {
      await api.createListDownload(listMatch[1], { auto_follow: true });
      toast.show(`已创建列表下载任务: List ${listMatch[1]}`);
      input.value = '';
    } catch (err) {
      toast.show(err.message, 'error');
    }
    return;
  }
  const userMatch = value.match(/(?:twitter\.com|x\.com)\/([^/\s?]+)/);
  if (userMatch) {
    const pathPart = userMatch[1];
    if (!['i', 'search', 'status', 'home', 'explore', 'notifications', 'messages', 'settings', 'compose', 'bookmarks', 'lists', 'communities'].includes(pathPart.toLowerCase())) {
      username = pathPart;
    }
  }
  if (username.startsWith('@')) username = username.slice(1);

  try {
    await api.createUserDownload(username, { auto_follow: true });
    toast.show(`已创建用户下载任务: @${username}`);
    input.value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createUserTask() {
  const screenName = document.getElementById('userScreenName').value.trim();
  if (!screenName) return toast.show('请输入 Screen Name', 'error');
  
  try {
    await api.createUserDownload(screenName, {
      auto_follow: document.getElementById('userAutoFollow').checked,
      follow_members: document.getElementById('userFollowMembers').checked,
      skip_profile: document.getElementById('userSkipProfile').checked,
      no_retry: document.getElementById('userNoRetry').checked
    });
    toast.show('用户下载任务已创建');
    document.getElementById('userScreenName').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createProfileTask() {
  const screenName = document.getElementById('userScreenName').value.trim();
  if (!screenName) return toast.show('请输入 Screen Name', 'error');

  try {
    await api.createProfileDownload(screenName);
    toast.show('Profile 下载任务已创建');
    document.getElementById('userScreenName').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createListTask() {
  const listId = document.getElementById('listId').value.trim();
  if (!listId) return toast.show('请输入 List ID', 'error');

  try {
    await api.createListDownload(listId, {
      auto_follow: document.getElementById('listAutoFollow').checked,
      follow_members: document.getElementById('listFollowMembers').checked,
      skip_profile: document.getElementById('listSkipProfile').checked,
      no_retry: document.getElementById('listNoRetry').checked
    });
    toast.show('列表下载任务已创建');
    document.getElementById('listId').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createListProfileTask() {
  const listId = document.getElementById('listId').value.trim();
  if (!listId) return toast.show('请输入 List ID', 'error');

  try {
    await api.createListProfile(listId);
    toast.show('列表 Profile 任务已创建');
    document.getElementById('listId').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createFollowingTask() {
  const screenName = document.getElementById('followingScreenName').value.trim();
  if (!screenName) return toast.show('请输入 Screen Name', 'error');

  try {
    await api.createFollowingDownload(screenName, {
      auto_follow: document.getElementById('followingAutoFollow').checked,
      follow_members: document.getElementById('followingFollowMembers').checked,
      skip_profile: document.getElementById('followingSkipProfile').checked,
      no_retry: document.getElementById('followingNoRetry').checked
    });
    toast.show('关注下载任务已创建');
    document.getElementById('followingScreenName').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createMarkTask() {
  const users = document.getElementById('markUsers').value.split('\n').map(s => s.trim()).filter(Boolean);
  const listIDs = readListIDsFromTextarea('markLists');
  const followingNames = document.getElementById('markFollowingNames').value.split('\n').map(s => s.trim()).filter(Boolean);

  if (!users.length && !listIDs.length && !followingNames.length) {
    return toast.show('请输入至少一个用户、列表或 Following 用户', 'error');
  }

  try {
    const timestamp = getOptionalTimestamp('markTimestamp');
    const data = {};
    if (users.length) data.users = users;
    if (listIDs.length) data.lists = listIDs;
    if (followingNames.length) data.following_names = followingNames;
    if (timestamp) data.timestamp = timestamp;

    await api.createBatchMark(data);
    document.getElementById('markUsers').value = '';
    document.getElementById('markLists').value = '';
    document.getElementById('markFollowingNames').value = '';
    document.getElementById('markTimestamp').value = '';

    const totalCount = users.length + listIDs.length + followingNames.length;
    toast.show(`已创建批量标记任务（共 ${totalCount} 个目标）`);
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createBatchTask() {
  const users = document.getElementById('batchUsers').value.split('\n').map(s => s.trim()).filter(Boolean);
  const lists = readListIDsFromTextarea('batchLists');
  const followingNames = document.getElementById('batchFollowingNames').value.split('\n').map(s => s.trim()).filter(Boolean);
  
  if (!users.length && !lists.length && !followingNames.length) {
    return toast.show('请输入至少一个用户、列表或 Following 用户', 'error');
  }
  
  try {
    await api.createBatchDownload({
      users,
      lists,
      following_names: followingNames,
      auto_follow: document.getElementById('batchAutoFollow').checked,
      follow_members: document.getElementById('batchFollowMembers').checked,
      skip_profile: document.getElementById('batchSkipProfile').checked,
      no_retry: document.getElementById('batchNoRetry').checked
    });
    toast.show(`批量任务已创建 (${users.length} 用户, ${lists.length} 列表, ${followingNames.length} 关注源)`);
    document.getElementById('batchUsers').value = '';
    document.getElementById('batchLists').value = '';
    document.getElementById('batchFollowingNames').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createJsonFileTask() {
  const uploadInput = document.getElementById('jsonFileUpload');
  const paths = readTextareaLines('jsonFilePaths');
  const noRetry = document.getElementById('jsonFileNoRetry').checked;

  if (uploadInput.files.length > 0) {
    const formData = new FormData();
    for (const file of uploadInput.files) formData.append('files', file);
    formData.append('no_retry', String(noRetry));

    try {
      const result = await api.upload('/api/v1/json/file/download', formData);
      toast.show(result.message || 'JSON 文件上传任务已创建');
      uploadInput.value = '';
      document.getElementById('jsonFilePaths').value = '';
    } catch (err) {
      toast.show(err.message, 'error');
    }
    return;
  }

  if (!paths.length) return toast.show('请选择至少一个 JSON 文件，或填写服务端路径', 'error');

  try {
    const result = await api.createJsonFileDownload({
      paths,
      no_retry: noRetry
    });
    toast.show(result.message || 'JSON 文件任务已创建');
    document.getElementById('jsonFilePaths').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createJsonFolderTask() {
  const uploadInput = document.getElementById('jsonFolderUpload');
  const paths = readTextareaLines('jsonFolderPath');
  const noRetry = document.getElementById('jsonFolderNoRetry').checked;

  if (uploadInput.files.length > 0) {
    const formData = new FormData();
    for (const file of uploadInput.files) formData.append('files', file);
    formData.append('no_retry', String(noRetry));

    try {
      const result = await api.upload('/api/v1/json/folder/download', formData);
      toast.show(result.message || 'LoongTweet 上传任务已创建');
      uploadInput.value = '';
      document.getElementById('jsonFolderPath').value = '';
    } catch (err) {
      toast.show(err.message, 'error');
    }
    return;
  }

  if (!paths.length) return toast.show('请选择至少一个 JSON 文件，或填写 LoongTweet 文件夹路径', 'error');

  try {
    const result = await api.createJsonFolderDownload({
      paths,
      no_retry: noRetry
    });
    toast.show(result.message || 'LoongTweet 任务已创建');
    document.getElementById('jsonFolderPath').value = '';
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function cancelTask(id) {
  if (!confirm('确定要取消这个任务吗？')) return;
  
  try {
    await api.cancelTask(id);
    toast.show('任务已取消');
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function retryTask(id) {
  try {
    await api.retryTask(id);
    toast.show('任务已重新创建');
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function deleteTask(id) {
  if (!confirm('确定要删除这个任务吗？')) return;
  
  try {
    await api.deleteTask(id);
    toast.show('任务已删除');
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function cancelQueuedTasks() {
  const queuedCount = store.state.tasks.filter(t => t.status === 'queued').length;
  if (queuedCount === 0) return toast.show('没有排队中的任务', 'error');
  if (!confirm(`确定要取消 ${queuedCount} 个排队中的任务吗？`)) return;
  
  try {
    const result = await api.cancelQueuedTasks();
    toast.show(`已取消 ${result.cancelled_count} 个排队中的任务`);
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function showTaskDetail(id) {
  drawer._taskId = id;
  drawer.open('任务详情', '<div class="text-sm text-secondary" style="text-align:center;padding:var(--space-8)">加载中...</div>');

  let task;
  try {
    task = await api.getTask(id);
  } catch (err) {
    drawer.open('任务详情',
      `<div class="task-detail-error">获取任务详情失败: ${escapeHtml(err.message)}</div>`,
      `<button class="btn btn-secondary" onclick="drawer.close()">关闭</button>
       <button class="btn btn-primary" onclick="showTaskDetail('${escapeAttr(id)}')">重试</button>`
    );
    return;
  }

  if (!task) {
    drawer.open('任务详情',
      '<div class="task-detail-error">未找到该任务</div>',
      '<button class="btn btn-secondary" onclick="drawer.close()">关闭</button>'
    );
    return;
  }

  const statusMap = {
    queued: '排队中',
    running: '运行中',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消'
  };

  const statusColors = {
    queued: '#8b949e',
    running: '#58a6ff',
    completed: '#3fb950',
    failed: '#f85149',
    cancelled: '#6e7681'
  };

  const bgColors = {
    queued: 'rgba(139,148,158,0.1)',
    running: 'rgba(88,166,255,0.1)',
    completed: 'rgba(63,185,80,0.1)',
    failed: 'rgba(248,81,73,0.1)',
    cancelled: 'rgba(110,118,129,0.1)'
  };

  const statusText = statusMap[task.status] || escapeHtml(task.status);
  const statusColor = statusColors[task.status] || '#8b949e';
  const bgColor = bgColors[task.status] || 'rgba(139,148,158,0.1)';
  const pct = getTaskProgressPercent(task);
  const stageText = task.progress?.stage ? escapeHtml(getStageText(task.progress.stage)) : '';
  const currentText = task.progress?.current ? ` · ${escapeHtml(task.progress.current)}` : '';
  const target = escapeHtml(getTaskTarget(task));

  // Build target details
  let targetDetails = '';
  if (task.data?.screen_name) {
    targetDetails = `<div class="task-detail-grid"><div class="task-detail-label">用户</div><div class="task-detail-value">@${escapeHtml(task.data.screen_name)}</div></div>`;
  } else if (task.data?.list_id) {
    targetDetails = `<div class="task-detail-grid"><div class="task-detail-label">列表</div><div class="task-detail-value">${escapeHtml(String(task.data.list_id))}</div></div>`;
  } else {
    const parts = [];
    if (task.data?.users?.length) parts.push(`<div class="task-detail-label">用户</div><div class="task-detail-value">${task.data.users.map(u => '@' + escapeHtml(u)).join(', ')}</div>`);
    if (task.data?.lists?.length) parts.push(`<div class="task-detail-label">列表</div><div class="task-detail-value">${task.data.lists.map(l => escapeHtml(String(l))).join(', ')}</div>`);
    if (task.data?.following_names?.length) parts.push(`<div class="task-detail-label">关注</div><div class="task-detail-value">${task.data.following_names.map(f => '@' + escapeHtml(f)).join(', ')}</div>`);
    if (parts.length) targetDetails = `<div class="task-detail-grid">${parts.join('')}</div>`;
  }

  // Build time timeline
  const createdTime = task.created_at ? new Date(task.created_at).toLocaleString() : '-';
  const startedTime = task.started_at ? new Date(task.started_at).toLocaleString() : null;
  const endedTime = task.ended_at ? new Date(task.ended_at).toLocaleString() : null;

  let durationText = '';
  if (task.started_at && task.ended_at) {
    const dur = new Date(task.ended_at) - new Date(task.started_at);
    const mins = Math.floor(dur / 60000);
    const secs = Math.round((dur % 60000) / 1000);
    if (mins > 0) durationText = `${mins}分${secs}秒`;
    else durationText = `${secs}秒`;
  }

  let timeHtml = `
    <div class="task-detail-time-row">
      <div class="task-detail-time-dot" style="background:var(--info)"></div>
      <div class="task-detail-time-label">创建</div>
      <div class="task-detail-time-value">${createdTime}</div>
    </div>`;
  if (startedTime) {
    timeHtml += `
    <div class="task-detail-time-line"></div>
    <div class="task-detail-time-row">
      <div class="task-detail-time-dot" style="background:var(--warning)"></div>
      <div class="task-detail-time-label">开始</div>
      <div class="task-detail-time-value">${startedTime}</div>
    </div>`;
  }
  if (endedTime) {
    timeHtml += `
    <div class="task-detail-time-line"></div>
    <div class="task-detail-time-row">
      <div class="task-detail-time-dot" style="background:var(--success)"></div>
      <div class="task-detail-time-label">结束</div>
      <div class="task-detail-time-value">${endedTime}</div>
    </div>`;
  }
  if (durationText) {
    timeHtml += `
    <div class="task-detail-time-line"></div>
    <div class="task-detail-time-row">
      <div class="task-detail-time-dot" style="background:var(--text-secondary)"></div>
      <div class="task-detail-time-label">耗时</div>
      <div class="task-detail-time-value" style="color:var(--text-primary)">${durationText}</div>
    </div>`;
  }

  // Build result
  let resultHtml = '';
  const result = task.result;
  if (result) {
    let mainHtml = '';
    if (result.main) {
      const parts = [`<span class="task-detail-stat"><span class="task-detail-stat-val success">${result.main.downloaded || 0}</span><span class="task-detail-stat-lbl">已下载</span></span>`];
      if (result.main.failed) {
        parts.push(`<span class="task-detail-stat"><span class="task-detail-stat-val danger">${result.main.failed}</span><span class="task-detail-stat-lbl">失败</span></span>`);
      }
      mainHtml = `<div class="task-detail-section-title-sm">主下载</div><div class="task-detail-stats">${parts.join('')}</div>`;
    }
    let profileHtml = '';
    if (result.profile) {
      const parts = [`<span class="task-detail-stat"><span class="task-detail-stat-val success">${result.profile.downloaded || 0}</span><span class="task-detail-stat-lbl">已下载</span></span>`];
      if (result.profile.failed) {
        parts.push(`<span class="task-detail-stat"><span class="task-detail-stat-val danger">${result.profile.failed}</span><span class="task-detail-stat-lbl">失败</span></span>`);
      }
      if (result.profile.versioned) {
        parts.push(`<span class="task-detail-stat"><span class="task-detail-stat-val info">${result.profile.versioned}</span><span class="task-detail-stat-lbl">已更新</span></span>`);
      }
      profileHtml = `<div class="task-detail-section-title-sm">Profile</div><div class="task-detail-stats">${parts.join('')}</div>`;
    }
    const msgHtml = result.message ? `<div class="task-detail-msg">${escapeHtml(result.message)}</div>` : '';

    if (mainHtml || profileHtml || msgHtml) {
      resultHtml = `
        <div class="task-detail-section">
          <div class="task-detail-section-title">结果</div>
          <div class="task-detail-card">
            ${mainHtml}${mainHtml && (profileHtml || msgHtml) ? '<div style="height:1px;background:var(--border-secondary);margin:var(--space-2) 0"></div>' : ''}
            ${profileHtml}${profileHtml && msgHtml ? '<div style="height:1px;background:var(--border-secondary);margin:var(--space-2) 0"></div>' : ''}
            ${msgHtml}
          </div>
        </div>`;
    }
  }

  // Build content
  const content = `
    <div class="task-detail-header" style="background:${bgColor}">
      <div class="task-detail-header-info">
        <div class="task-detail-header-title">${target || '未知目标'}</div>
        <div class="task-detail-header-sub">${escapeHtml(task.task_id)}</div>
      </div>
      <span class="tag tag-${task.status}" style="font-size:var(--text-base)">${statusText}</span>
    </div>

    <div class="task-detail-section">
      <div class="task-detail-section-title">概览</div>
      <div class="task-detail-card">
        <div class="task-detail-grid">
          <div class="task-detail-label">类型</div>
          <div class="task-detail-value">${escapeHtml(task.type)}</div>
          <div class="task-detail-label">状态</div>
          <div class="task-detail-value" style="color:${statusColor}">${statusText}</div>
        </div>
      </div>
    </div>

    ${targetDetails ? `
    <div class="task-detail-section">
      <div class="task-detail-section-title">目标</div>
      <div class="task-detail-card">${targetDetails}</div>
    </div>` : ''}

    <div class="task-detail-section">
      <div class="task-detail-section-title">进度</div>
      <div class="task-detail-card">
        <div class="progress-bar" style="margin-bottom: var(--space-2);">
          <div class="progress-fill" style="width: ${pct}%"></div>
        </div>
        <div class="text-sm" style="display:flex;justify-content:space-between;color:var(--text-secondary);">
          <span>${task.progress?.completed || 0} / ${task.progress?.total || 0} (${pct}%)</span>
          <span>${stageText}${currentText}</span>
        </div>
        ${task.progress?.failed ? `<div class="text-sm" style="color: var(--danger); margin-top: 6px;">失败推文: ${escapeHtml(task.progress.failed)}</div>` : ''}
      </div>
    </div>

    <div class="task-detail-section">
      <div class="task-detail-section-title">时间</div>
      <div class="task-detail-card">${timeHtml}</div>
    </div>

    ${resultHtml}

    ${task.error ? `
    <div class="task-detail-section">
      <div class="task-detail-section-title" style="color:var(--danger);border-bottom-color:rgba(248,81,73,0.3);">错误</div>
      <div class="task-detail-error">${escapeHtml(task.error)}</div>
    </div>` : ''}
  `;

  const footer = task.status === 'running' || task.status === 'queued' ?
    `<button class="btn btn-danger" data-task-id="${escapeAttr(task.task_id)}" onclick="cancelTask(this.dataset.taskId); drawer.close();">取消任务</button>` :
    `<button class="btn btn-primary" data-task-id="${escapeAttr(task.task_id)}" onclick="retryTask(this.dataset.taskId); drawer.close();">重试</button>
     <button class="btn btn-danger" data-task-id="${escapeAttr(task.task_id)}" onclick="deleteTask(this.dataset.taskId); drawer.close();">删除</button>
     <button class="btn btn-secondary" onclick="drawer.close()">关闭</button>`;

  drawer.open('任务详情', content, footer);
}

async function refreshTasks() {
  try {
    const data = await api.getTasks();
    store.setState({ tasks: data.tasks || [] });
    toast.show('任务列表已刷新');
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

function refreshLogs() { loadLogs(); restartLogStreamIfNeeded(); }

function escapeHtml(str) {
  const d = document.createElement('div');
  d.textContent = str;
  return d.innerHTML;
}

function escapeAttr(str) {
  return String(str).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/`/g, '&#96;');
}

function stripAnsi(str) { return str.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, ''); }

function renderConfigEditor() {
  const { configMode, configFields, configSaving, configExists, configRaw, configFieldsLoading } = store.state;

  const modeTabs = `
    <div class="config-mode-tabs">
      <button class="mode-tab ${configMode === 'form' ? 'active' : ''}" onclick="setConfigMode('form')">📝 简易模式</button>
      <button class="mode-tab ${configMode === 'raw' ? 'active' : ''}" onclick="setConfigMode('raw')">🔧 高级 (YAML)</button>
    </div>
  `;

  if (configMode === 'raw') return `<div style="display:flex;flex-direction:column;height:100%">${modeTabs}${renderConfigRawEditor(configRaw, configSaving, configExists)}</div>`;
  return `<div style="display:flex;flex-direction:column;height:100%">${modeTabs}${renderConfigForm(configFields, configSaving, configExists, configFieldsLoading)}</div>`;
}

function renderConfigForm(fields, saving, exists, loading = false) {
  if (loading || !fields || fields.length === 0) {
    return `
      <div class="card">
        <div class="card-body"><div class="empty-state"><div class="empty-icon">⏳</div><div class="empty-title">加载中...</div></div></div>
      </div>
    `;
  }

  const groups = { basic: [], cookie: [], advanced: [] };
  fields.forEach(f => { if (groups[f.group]) groups[f.group].push(f); });
  const groupLabels = { basic: '📁 基础设置', cookie: '🍪 Cookie 认证', advanced: '⚙️ 高级选项' };

  const renderField = f => {
    const inputType = f.type === 'password' ? 'password' : (f.type === 'number' ? 'number' : 'text');
    const placeholder = f.type === 'password' && f.value
      ? `当前值: ${escapeHtml(f.value)}`
      : escapeAttr(f.placeholder || f.prompt);
    return `
      <div class="config-field">
        <label class="config-label">${escapeHtml(f.label)}</label>
        <input type="${inputType}" class="form-input config-input" id="cf_${escapeAttr(f.name)}"
          name="${escapeAttr(f.name)}" value="${escapeAttr(f.type === 'password' ? '' : f.value)}"
          placeholder="${placeholder}"
          ${f.type === 'number' ? `min="1" max="${f.name.includes('routine') ? '100' : '250'}"` : ''}>
      </div>
    `;
  };

  return `
    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">配置编辑</div>
          <div class="card-subtitle">${exists ? '✅ 配置文件存在' : '⚠️ 将创建新配置'} · 共 ${fields.length} 个可编辑项</div>
        </div>
        <button class="btn btn-primary btn-sm" onclick="saveConfigForm()" ${saving ? 'disabled' : ''}>
          ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存配置'}
        </button>
      </div>
      <div class="card-body">
        ${Object.entries(groups).map(([key, items]) => items.length ? `
          <div class="config-group">
            <div class="config-group-title">${groupLabels[key]}</div>
            ${items.map(renderField).join('')}
          </div>
        ` : '').join('')}
      </div>
    </div>
  `;
}

function renderConfigRawEditor(raw, saving, exists) {
  if (raw === null) {
    return `
      <div class="card">
        <div class="card-header"><div><div class="card-title">conf.yaml 原始编辑器</div></div></div>
        <div class="card-body">
          <div class="empty-state">
            <div class="skeleton" style="width:64px;height:64px;border-radius:12px;margin-bottom:16px"></div>
            <div class="empty-title">加载中...</div>
            <div class="empty-desc">正在加载配置文件</div>
          </div>
        </div>
      </div>`;
  }
  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">conf.yaml 原始编辑器</div><div class="card-subtitle">${exists ? '✅ 文件存在' : '⚠️ 将创建新配置'}</div></div>
        <div class="flex gap-2">
          <button class="btn btn-primary btn-sm" onclick="saveConfig()" ${saving ? 'disabled' : ''}>
            ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存配置'}
          </button>
        </div>
      </div>
      <div class="card-body" style="padding:0;display:flex;flex-direction:column;overflow:hidden">
        <div id="configEditorContainer" style="flex:1;min-height:0"></div>
        <div class="config-hint text-sm text-tertiary p-3 mt-3" style="flex-shrink:0">
          ⚠️ 直接编辑 YAML 需要了解语法格式。建议使用简易模式。
        </div>
      </div>
    </div>
  `;
}

function renderCookiesEditor() {
  const { cookiesMode, cookieItems, cookiesSaving, cookiesExists, cookiesRaw, _cookiesLoading } = store.state;

  const modeTabs = `
    <div class="config-mode-tabs">
      <button class="mode-tab ${cookiesMode === 'form' ? 'active' : ''}" onclick="setCookiesMode('form')">📝 简易模式</button>
      <button class="mode-tab ${cookiesMode === 'raw' ? 'active' : ''}" onclick="setCookiesMode('raw')">🔧 高级 (YAML)</button>
    </div>
  `;

  if (cookiesMode === 'raw') return `<div style="display:flex;flex-direction:column;height:100%">${modeTabs}${renderCookiesRawEditor(cookiesRaw, cookiesSaving, cookiesExists)}</div>`;
  return `<div style="display:flex;flex-direction:column;height:100%">${modeTabs}${renderCookiesForm(cookieItems, cookiesSaving, cookiesExists, _cookiesLoading)}</div>`;
}

function renderCookiesForm(items, saving, exists, loading = false) {
  if (loading) {
    return `
      <div class="card">
        <div class="card-header"><div><div class="card-title">额外账户管理</div></div></div>
        <div class="card-body">
          <div class="empty-state">
            <div class="skeleton" style="width:64px;height:64px;border-radius:12px;margin-bottom:16px"></div>
            <div class="empty-title">加载中...</div>
            <div class="empty-desc">正在加载额外账户配置</div>
          </div>
        </div>
      </div>
    `;
  }
  if (!items || items.length === 0) {
    return `
      <div class="card">
        <div class="card-header">
          <div><div class="card-title">额外账户管理</div><div class="card-subtitle">${exists ? '✅ 文件存在 · 0 个账户' : '⚠️ 将创建新文件'}</div></div>
          <div class="flex gap-2">
            <button class="btn btn-ghost btn-sm" onclick="addCookieAccount()">➕ 添加账户</button>
            <button class="btn btn-primary btn-sm" onclick="saveCookiesForm()" ${saving ? 'disabled' : ''}>
              ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存配置'}
            </button>
          </div>
        </div>
        <div class="card-body">
          <div class="empty-state">
            <div class="empty-icon">🍪</div>
            <div class="empty-title">暂无额外账户</div>
            <div class="empty-desc">点击「添加账户」添加额外的 Twitter 账号</div>
          </div>
        </div>
      </div>
    `;
  }

  const renderItem = (item, idx) => `
    <div class="config-group">
      <div class="config-group-title">
        <span>🏷️ 账户 #${idx + 1}</span>
        <button class="btn btn-danger btn-sm" onclick="removeCookieAccount(${idx})">删除</button>
      </div>
      <div class="config-field">
        <label class="config-label">Auth Token</label>
        <input type="password" class="form-input config-input cookie-input" id="cookie_auth_${idx}"
          name="auth_token_${idx}" value="" placeholder="${item.auth_token ? '当前值: ' + escapeHtml(item.auth_token) : '请输入 auth_token'}">
      </div>
      <div class="config-field">
        <label class="config-label">CT0</label>
        <input type="password" class="form-input config-input cookie-input" id="cookie_ct0_${idx}"
          name="ct0_${idx}" value="" placeholder="${item.ct0 ? '当前值: ' + escapeHtml(item.ct0) : '请输入 ct0'}">
      </div>
    </div>
  `;

  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">额外账户管理</div><div class="card-subtitle">${exists ? '✅ 文件存在' : '⚠️ 将创建新文件'} · 共 ${items.length} 个账户</div></div>
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-sm" onclick="addCookieAccount()">➕ 添加账户</button>
          <button class="btn btn-primary btn-sm" onclick="saveCookiesForm()" ${saving ? 'disabled' : ''}>
            ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存配置'}
          </button>
        </div>
      </div>
      <div class="card-body">
        ${items.map(renderItem).join('<div class="config-divider"></div>')}
      </div>
    </div>
  `;
}

function renderCookiesRawEditor(raw, saving, exists) {
  if (raw === null) {
    return `
      <div class="card">
        <div class="card-header"><div><div class="card-title">additional_cookies.yaml 原始编辑器</div></div></div>
        <div class="card-body">
          <div class="empty-state">
            <div class="skeleton" style="width:64px;height:64px;border-radius:12px;margin-bottom:16px"></div>
            <div class="empty-title">加载中...</div>
            <div class="empty-desc">正在加载额外账户配置</div>
          </div>
        </div>
      </div>`;
  }
  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">additional_cookies.yaml 原始编辑器</div><div class="card-subtitle">${exists ? '✅ 文件存在' : '⚠️ 将创建新文件'}</div></div>
        <div class="flex gap-2">
          <button class="btn btn-primary btn-sm" onclick="saveCookies()" ${saving ? 'disabled' : ''}>
            ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存配置'}
          </button>
        </div>
      </div>
      <div class="card-body" style="padding:0;display:flex;flex-direction:column;overflow:hidden">
        <div id="cookiesEditorContainer" style="flex:1;min-height:0"></div>
        <div class="config-hint text-sm text-tertiary p-3 mt-3" style="flex-shrink:0">
          ⚠️ 直接编辑 YAML 需要了解语法格式。建议使用简易模式。
        </div>
      </div>
    </div>
  `;
}

function renderLogViewer() {
  const { logs, logLevel, logSearch, logPagination, logAutoRefresh, logStats, _logsLoading } = store.state;

  function getLineColor(line) {
    if (line.startsWith('ERRO[')) return 'var(--danger)';
    if (line.startsWith('WARN[')) return 'var(--warning)';
    if (line.startsWith('INFO[')) return 'var(--info)';
    if (line.startsWith('DEBU[')) return 'var(--text-tertiary)';
    const levelColors = {
      DEBUG: 'var(--text-tertiary)',
      INFO: 'var(--info)',
      WARNING: 'var(--warning)',
      WARN: 'var(--warning)',
      ERROR: 'var(--danger)'
    };
    for (const [key, level] of [['debug', 'DEBUG'], ['info', 'INFO'], ['warn', 'WARN'], ['warning', 'WARNING'], ['error', 'ERROR']]) {
      if (line.includes('level=' + key)) return levelColors[level];
    }
    return 'var(--text-secondary)';
  }
  function highlightLogTimestamp(line) {
    // logrus 格式: time="..." → escapeHtml 后 time=&quot;...&quot;
    line = line.replace(
      /time=(&quot;)(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[-+]\d{2}:\d{2})(&quot;)/g,
      'time=<span class="log-timestamp">$2</span>'
    );
    // text 格式: LEVEL[TIMESTAMP]
    line = line.replace(
      /(ERRO|WARN|INFO|DEBU)\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})\]/g,
      '$1[<span class="log-timestamp">$2</span>]'
    );
    return line;
  }
  function renderLine(line) {
    return `<div class="log-line" style="color:${getLineColor(line)}">${highlightLogTimestamp(escapeHtml(stripAnsi(line)))}</div>`;
  }

  return `
    <div class="card card-page">
      <div class="card-header">
        <div><div class="card-title">系统日志</div><div class="card-subtitle">共 ${logPagination.total} 条记录</div></div>
        <div class="flex gap-2 items-center flex-wrap">
          <input type="text" class="form-input search-input" id="logSearchInput"
            placeholder="🔍 搜索..."
            oninput="onLogSearchInput(this.value)">
          <div class="log-level-filters">
            ${['all','debug','info','warn','error'].map(l => {
              const count = l === 'all' ? logStats.total : (logStats[l] || 0);
              return `<button class="btn btn-sm ${logLevel===l?'btn-primary':'btn-ghost'}" onclick="setLogLevel('${l}')">${l.toUpperCase()}${count > 0 ? ` (${count})` : ''}</button>`;
            }).join('')}
          </div>
          <button class="btn btn-ghost btn-sm ${logAutoRefresh?'active':''}" onclick="toggleLogAutoRefresh()">${logAutoRefresh?'⏸️':'▶️'} 实时</button>
          <button class="btn btn-ghost btn-sm" onclick="api.downloadLogExport()" title="导出完整日志文件">📥 导出</button>
        </div>
      </div>
      <div class="card-body card-body-scroll" style="position:relative">
        <div class="log-container" id="logContainer">
          ${_logsLoading && logs.length === 0
            ? `<div class="empty-state"><div class="skeleton" style="width:64px;height:64px;border-radius:12px;margin-bottom:16px"></div><div class="empty-title">加载中...</div><div class="empty-desc">正在加载系统日志</div></div>`
            : logs.length === 0
            ? `<div class="empty-state"><div class="empty-icon">📋</div><div class="empty-title">暂无日志</div><div class="empty-desc">选择日志级别或调整筛选条件</div></div>`
            : logs.map(renderLine).join('')}
        </div>
        <button class="log-scroll-to-top-btn" id="logScrollToTopBtn"
          onclick="scrollLogToTop()" aria-label="滚动到日志顶部"
          style="display:${store.state._logNewArrived ? 'flex' : 'none'}">
          📌 新日志已到达
        </button>
      </div>
      <div class="pagination">
        <div class="pagination-info">显示 ${logs.length} / ${logPagination.total} 条 (第 ${logPagination.page}/${logPagination.totalPages} 页)</div>
        <div class="pagination-controls">
          <button class="page-btn" onclick="changeLogPage(-1)" ${logPagination.page <= 1 ? 'disabled' : ''}>←</button>
          ${renderPageNumbers(logPagination.page, logPagination.totalPages, 'goToLogPage')}
          <button class="page-btn" onclick="changeLogPage(1)" ${logPagination.page >= logPagination.totalPages ? 'disabled' : ''}>→</button>
        </div>
      </div>
    </div>
  `;
}

async function loadConfigFields() {
  if (store.state.configFieldsLoading) return;
  store.setState({ configFieldsLoading: true });
  try {
    const d = await api.getConfigFields();
    store.setState({ configFields: d.fields || [], configExists: d.exists || false, configFieldsLoading: false });
  } catch (e) {
    toast.show('加载配置失败: ' + e.message, 'error');
    store.setState({ configFieldsLoading: false });
  }
}

async function loadConfigRaw() {
  try {
    const d = await api.getConfigRaw();
    store.setState({ configRaw: d.content || '', configExists: d.exists || false });
  } catch (e) { toast.show('加载配置失败: ' + e.message, 'error'); }
}

function isPanelInputFocused(panelId) {
  const panel = document.getElementById(panelId);
  if (!panel || !document.activeElement || !panel.contains(document.activeElement)) return false;
  return document.activeElement.matches('input, textarea, select');
}

function isConfigFormDirty() {
  if (store.state.configMode !== 'form') return false;
  return (store.state.configFields || []).some(field => {
    const input = document.getElementById(`cf_${field.name}`);
    if (!input) return false;
    if (field.type === 'password') return input.value.trim() !== '';
    return input.value !== String(field.value ?? '');
  });
}

function isConfigRawDirty() {
  return store.state.configMode === 'raw' && _state.configCodeMirror && getEditorValue(_state.configCodeMirror, store.state.configRaw) !== (store.state.configRaw || '');
}

function refreshConfigAfterReconnect() {
  if (isPanelInputFocused('systemConfigPanel') || isConfigFormDirty() || isConfigRawDirty()) return;
  if (store.state.configMode === 'raw') loadConfigRaw();
  else loadConfigFields();
}

function showManualRestartNotice(subject) {
  toast.show(`✅ ${subject}已保存，需要手动重启服务后生效`, 'success');
}

function renderScheduleViewer() {
  const { _scheduleTab, _schedules, _scheduleRaw, _scheduleExists, _scheduleSaving, _scheduleFormItems, _schedulerRunning } = store.state;

  const schedulerBanner = !_schedulerRunning
    ? `<div class="alert alert-warning" style="margin-bottom:var(--space-3)">⚠️ 调度器未启动，定时任务不会自动执行。请添加并启用规则后重载配置。</div>`
    : '';

  const modeTabs = `
    <div class="config-mode-tabs">
      <button class="mode-tab ${_scheduleTab === 'form' ? 'active' : ''}" onclick="setScheduleTab('form')">📝 简易模式</button>
      <button class="mode-tab ${_scheduleTab === 'edit' ? 'active' : ''}" onclick="setScheduleTab('edit')">🔧 高级 (YAML)</button>
    </div>
  `;

  if (_scheduleTab === 'edit') return `<div style="display:flex;flex-direction:column;height:100%">${schedulerBanner}${modeTabs}${renderScheduleRawEditor(_scheduleRaw, _scheduleSaving, _scheduleExists)}</div>`;
  return `<div style="display:flex;flex-direction:column;height:100%">${schedulerBanner}${modeTabs}${renderScheduleForm(_scheduleFormItems, _scheduleSaving, _scheduleExists, _schedules === null)}</div>`;
}

function renderScheduleFormField(item, idx) {
  const typeOptions = (selected) => ['list', 'user', 'following', 'mixed'].map(t =>
    `<option value="${t}" ${t === selected ? 'selected' : ''}>${t === 'list' ? '📋 列表' : t === 'user' ? '👤 用户' : t === 'following' ? '👥 关注' : '🔀 混合'}</option>`
  ).join('');

  const scheduleModeOptions = (selected) => ['interval', 'daily'].map(m =>
    `<option value="${m}" ${m === selected ? 'selected' : ''}>${m === 'interval' ? '⏱️ 间隔执行' : '🕐 每日定时'}</option>`
  ).join('');

  return `
    <div class="config-group">
      <div class="config-group-title">
        <span>📋 任务 #${idx + 1}${item.name ? ' · ' + escapeHtml(item.name) : ''}</span>
        <button class="btn btn-danger btn-sm" onclick="removeScheduleItem(${idx})">删除</button>
      </div>
      <div class="config-field">
        <label class="config-label" for="sf_type_${idx}">类型</label>
        <select class="form-input config-input" id="sf_type_${idx}" onchange="updateScheduleFormItem(${idx}, 'type', this.value)">
          ${typeOptions(item.type)}
        </select>
      </div>
      ${item.type === 'mixed' ? `
      <div class="config-field">
        <label class="config-label" for="sf_users_${idx}">用户名 <span style="font-size:11px;color:var(--text-tertiary)">每行一个</span></label>
        <textarea class="form-textarea config-input" id="sf_users_${idx}" rows="3"
          aria-describedby="sf_schedule_hint_${idx}"
          placeholder="elonmusk&#10;openai" oninput="scheduleFieldChanged(${idx})" onblur="validateScheduleField(${idx})">${escapeHtml((item.users || []).join('\n'))}</textarea>
      </div>
      <div class="config-field">
        <label class="config-label" for="sf_lists_${idx}">列表 ID <span style="font-size:11px;color:var(--text-tertiary)">每行一个</span></label>
        <textarea class="form-textarea config-input" id="sf_lists_${idx}" rows="3"
          aria-describedby="sf_schedule_hint_${idx}"
          placeholder="123456789&#10;987654321" oninput="scheduleFieldChanged(${idx})" onblur="validateScheduleField(${idx})">${escapeHtml((item.lists || []).join('\n'))}</textarea>
      </div>
      <div class="config-field">
        <label class="config-label" for="sf_following_${idx}">关注用户名 <span style="font-size:11px;color:var(--text-tertiary)">每行一个</span></label>
        <textarea class="form-textarea config-input" id="sf_following_${idx}" rows="3"
          aria-describedby="sf_schedule_hint_${idx}"
          placeholder="someuser" oninput="scheduleFieldChanged(${idx})" onblur="validateScheduleField(${idx})">${escapeHtml((item.following_names || []).join('\n'))}</textarea>
      </div>` : `
      <div class="config-field">
        <label class="config-label" for="sf_target_${idx}">${item.type === 'list' ? '列表 ID' : '用户名 (Screen Name)'}</label>
        <input type="text" class="form-input config-input" id="sf_target_${idx}"
          value="${escapeAttr(item.target || '')}"
          aria-describedby="sf_schedule_hint_${idx}"
          placeholder="${item.type === 'list' ? '例如: 123456789' : '例如: elonmusk'}"
          onblur="validateScheduleField(${idx})" oninput="scheduleFieldChanged(${idx})">
      </div>`}
      <div class="config-field">
        <label class="config-label">名称（可选）</label>
        <input type="text" class="form-input config-input" id="sf_name_${idx}"
          value="${escapeAttr(item.name || '')}"
          placeholder="给这条规则起个名字">
      </div>
      <div class="config-field">
        <label class="config-label">调度方式</label>
        <select class="form-input config-input" id="sf_schedule_mode_${idx}" onchange="updateScheduleFormItem(${idx}, 'scheduleMode', this.value)">
          ${scheduleModeOptions(item.scheduleMode || 'interval')}
        </select>
      </div>
      <div class="config-field">
        <label class="config-label" for="sf_schedule_value_${idx}">${(item.scheduleMode || 'interval') === 'interval' ? '执行间隔' : '执行时间'}</label>
        <input type="text" class="form-input config-input" id="sf_schedule_value_${idx}"
          value="${escapeAttr(item.scheduleValue || '')}"
          aria-describedby="sf_schedule_hint_${idx}"
          placeholder="${(item.scheduleMode || 'interval') === 'interval' ? '例如: 2h, 30m, 6h30m, 24h' : '例如: 07:00,21:00 或 02:30'}"
          onblur="validateScheduleField(${idx})" oninput="scheduleFieldChanged(${idx})">
      </div>
      <div class="config-field" style="display:flex;gap:16px;flex-wrap:wrap;">
        <label class="config-label" style="display:inline-flex;align-items:center;gap:4px">
          <input type="checkbox" id="sf_enabled_${idx}" ${item.enabled ? 'checked' : ''} style="margin:0">
          启用
        </label>
        <label class="config-label" style="display:inline-flex;align-items:center;gap:4px">
          <input type="checkbox" id="sf_auto_follow_${idx}" ${item.auto_follow ? 'checked' : ''} style="margin:0">
          自动申请受保护账号
        </label>
        <label class="config-label" style="display:inline-flex;align-items:center;gap:4px">
          <input type="checkbox" id="sf_follow_members_${idx}" ${item.follow_members ? 'checked' : ''} style="margin:0">
          下载时关注目标/成员
        </label>
        <label class="config-label" style="display:inline-flex;align-items:center;gap:4px">
          <input type="checkbox" id="sf_skip_profile_${idx}" ${item.skip_profile ? 'checked' : ''} style="margin:0">
          跳过 Profile
        </label>
        <label class="config-label" style="display:inline-flex;align-items:center;gap:4px">
          <input type="checkbox" id="sf_no_retry_${idx}" ${item.no_retry ? 'checked' : ''} style="margin:0">
          不重试
        </label>
        <label class="config-label" style="display:inline-flex;align-items:center;gap:4px">
          <input type="checkbox" id="sf_run_on_start_${idx}" ${item.run_on_start ? 'checked' : ''} style="margin:0">
          首次启动时立即运行
        </label>
      </div>
      <div id="sf_schedule_hint_${idx}" class="config-hint" aria-live="polite" style="font-size:12px;margin-top:8px;min-height:0"></div>
    </div>
  `;
}

function renderScheduleForm(items, saving, exists, loading = false) {
  if (loading) {
    return `
      <div class="card">
        <div class="card-header"><div><div class="card-title">定时下载任务</div></div></div>
        <div class="card-body">
          <div class="empty-state">
            <div class="skeleton" style="width:64px;height:64px;border-radius:12px;margin-bottom:16px"></div>
            <div class="empty-title">加载中...</div>
            <div class="empty-desc">正在加载定时任务配置</div>
          </div>
        </div>
      </div>
    `;
  }
  if (!items || items.length === 0) {
    return `
      <div class="card">
        <div class="card-header">
          <div><div class="card-title">定时下载任务</div><div class="card-subtitle">${exists ? '✅ 文件存在 · 0 条规则' : '⚠️ 配置文件不存在'}</div></div>
          <div class="flex gap-2">
            <button class="btn btn-ghost btn-sm" onclick="addScheduleItem()">➕ 添加规则</button>
          </div>
        </div>
        <div class="card-body">
          <div class="empty-state">
            <div class="empty-icon">⏰</div>
            <div class="empty-title">暂无定时任务</div>
            <div class="empty-desc">点击「添加规则」创建定时下载任务</div>
          </div>
        </div>
      </div>
    `;
  }

  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">定时下载任务</div><div class="card-subtitle">${exists ? '✅ 文件存在' : '⚠️ 将创建新文件'} · 共 ${items.length} 条规则</div></div>
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-sm" onclick="addScheduleItem()">➕ 添加规则</button>
          <button class="btn btn-primary btn-sm" onclick="saveScheduleForm()" ${saving ? 'disabled' : ''}>
            ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存并重载'}
          </button>
        </div>
      </div>
      <div class="card-body">
        ${items.map((item, idx) => renderScheduleFormField(item, idx)).join('<div class="config-divider"></div>')}
      </div>
    </div>
  `;
}

// Shared helpers for schedule table rendering
function typeTag(type) {
  const map = { list: ['List', 'tag-info'], user: ['User', 'tag-success'], following: ['Following', 'tag-warning'], mixed: ['Mixed', 'tag-primary'] };
  const [label, cls] = map[type] || [escapeHtml(type), ''];
  return `<span class="tag ${escapeHtml(cls)}">${escapeHtml(label)}</span>`;
}

function failureTag(count) {
  if (!count || count === 0) return '';
  if (count >= 3) return `<span class="tag tag-danger">⚠ ${count}次失败</span>`;
  return `<span class="tag tag-warning">${count}次失败</span>`;
}

function getLastTask(s) {
  const taskId = s ? s.last_task_id : null;
  if (!taskId) return null;
  return (store.state.tasks || []).find(t => t.task_id === taskId);
}

function taskStatusTag(task) {
  if (!task) return '';
  const statusMap = {
    completed: { tag: 'tag-completed', text: '完成' },
    failed: { tag: 'tag-failed', text: '失败' },
    running: { tag: 'tag-running', text: '运行中' },
    queued: { tag: 'tag-queued', text: '排队' },
    cancelled: { tag: 'tag-cancelled', text: '已取消' }
  };
  const st = statusMap[task.status];
  if (!st) return '';
  return `<span class="tag ${st.tag}" style="font-size:10px;padding:1px 6px">${st.text}</span>`;
}

function fmtTime(t) {
  return t ? new Date(t).toLocaleString() : '-';
}

function renderScheduleItem(s) {
  const entry = normalizeScheduleEntry(s.entry);
  const failures = s.consecutive_failures || 0;
  let displayName = entry.name || entry.target;
  if (entry.type === 'mixed' && !displayName) {
    const parts = [];
    if ((entry.users || []).length) parts.push(`${entry.users.length} 用户`);
    if ((entry.lists || []).length) parts.push(`${entry.lists.length} 列表`);
    if ((entry.following_names || []).length) parts.push(`${entry.following_names.length} 关注`);
    displayName = parts.join(' · ') || '混合任务';
  } else if (!displayName) {
    displayName = entry.type === 'following'
      ? '关注任务'
      : entry.type === 'user'
        ? '用户任务'
        : entry.type === 'list'
          ? '列表任务'
          : '定时任务';
  }
  const metaParts = [escapeHtml(s.schedule_display), `执行 ${s.run_count} 次`];
  if (entry.type === 'mixed') {
    const targetParts = [];
    if ((entry.users || []).length) targetParts.push(`${entry.users.length}用户`);
    if ((entry.lists || []).length) targetParts.push(`${entry.lists.length}列表`);
    if ((entry.following_names || []).length) targetParts.push(`${entry.following_names.length}关注`);
    if (targetParts.length) metaParts.unshift(targetParts.join('+'));
  }
  const fTag = failureTag(failures);
  if (fTag) metaParts.push(fTag);

  const lastTask = getLastTask(s);
  const tTag = taskStatusTag(lastTask);
  if (tTag) metaParts.push(tTag);

  const entryId = escapeAttr(entry.id);

  return `
    <div class="schedule-item${failures >= 3 ? ' has-failure' : ''}">
      <div class="schedule-type">${typeTag(entry.type)}</div>
      <div class="schedule-info">
        <div class="schedule-title">${escapeHtml(displayName)}</div>
        <div class="schedule-meta">${metaParts.join('<span style="color:var(--border-secondary)">·</span>')}</div>
      </div>
      <div class="schedule-status">
        <span class="tag ${entry.enabled ? 'tag-success' : 'tag-danger'}" style="cursor:pointer" data-schedule-id="${escapeAttr(entry.id)}" data-enabled="${entry.enabled}" onclick="toggleScheduleEnabled(this.dataset.scheduleId, this.dataset.enabled === 'true')">${entry.enabled ? '启用' : '禁用'}</span>
      </div>
      <div class="schedule-time">
        <div>上次 ${fmtTime(s.last_run_at)}</div>
        <div>下次 ${fmtTime(s.next_run_at)}</div>
      </div>
      <div class="schedule-actions">
        <button class="btn btn-primary btn-sm" data-schedule-id="${escapeAttr(entry.id)}" onclick="triggerSchedule(this.dataset.scheduleId)" ${!entry.enabled ? 'disabled title="规则已禁用"' : ''}>▶ 执行</button>
      </div>
    </div>
  `;
}

function renderScheduleTable(schedules, exists) {
  schedules = schedules || [];
  const active = schedules.filter(s => readScheduleEntryField(s.entry, 'enabled', 'Enabled')).length;
  const total = schedules.length;
  const failures = schedules.filter(s => (s.consecutive_failures || 0) > 0).length;

  if (schedules.length === 0) {
    return `
      <div class="card">
        <div class="card-header">
          <div><div class="card-title">定时下载任务</div><div class="card-subtitle">${exists ? '✅ 文件存在 · 0 条规则' : '⚠️ 配置文件不存在'}</div></div>
        </div>
        <div class="card-body">
          <div class="empty-state">
            <div class="empty-icon">⏰</div>
            <div class="empty-title">暂无定时任务</div>
            <div class="empty-desc">点击「添加规则」创建定时下载任务</div>
          </div>
        </div>
      </div>
    `;
  }

  return `
    <div class="card card-fill">
      <div class="card-header">
        <div><div class="card-title">定时下载任务</div><div class="card-subtitle">共 ${total} 条规则 · ${active} 个启用${failures > 0 ? ` · ${failures} 个异常` : ''}</div></div>
        <div class="flex gap-2">
          <button class="btn btn-primary btn-sm" id="btnTriggerAll" onclick="triggerAllSchedules()">⬇️ 下载全部</button>
          <button class="btn btn-ghost btn-sm" onclick="navigateToSystemSchedules()">📝 编辑任务</button>
        </div>
      </div>
      <div class="card-body card-body-scroll">
        ${schedules.length === 0 ? `
          <div class="empty-state">
            <div class="empty-icon">⏰</div>
            <div class="empty-title">暂无定时任务</div>
            <div class="empty-desc">点击上方「编辑任务」按钮创建定时下载规则</div>
          </div>
        ` : `
          <div class="schedule-list">
            ${schedules.map(renderScheduleItem).join('')}
          </div>
        `}
      </div>
    </div>
  `;
}

function renderScheduleRawEditor(raw, saving, exists) {
  if (raw === null) {
    return `
      <div class="card">
        <div class="card-header"><div><div class="card-title">schedules.yaml 原始编辑器</div></div></div>
        <div class="card-body">
          <div class="empty-state">
            <div class="skeleton" style="width:64px;height:64px;border-radius:12px;margin-bottom:16px"></div>
            <div class="empty-title">加载中...</div>
            <div class="empty-desc">正在加载定时任务配置</div>
          </div>
        </div>
      </div>`;
  }
  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">schedules.yaml 原始编辑器</div><div class="card-subtitle">${exists ? '✅ 文件存在' : '⚠️ 将创建新文件'}</div></div>
        <div class="flex gap-2">
          <button class="btn btn-primary btn-sm" onclick="saveScheduleRaw()" ${saving ? 'disabled' : ''}>
            ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存并重载'}
          </button>
        </div>
      </div>
      <div class="card-body" style="padding:0;display:flex;flex-direction:column;overflow:hidden">
        <div id="scheduleEditorContainer" style="flex:1;min-height:0"></div>
        <div class="config-hint text-sm text-tertiary p-3 mt-3" style="flex-shrink:0">
          ⚠️ 保存后将自动重载调度配置，无需重启服务。
        </div>
      </div>
    </div>
  `;
}

function isScheduleFormEditing() {
  if (store.state.currentPage !== 'system' || store.state._systemTab !== 'schedules' || store.state._scheduleTab !== 'form') {
    return false;
  }
  const panel = document.getElementById('systemSchedulesPanel');
  if (!panel || !document.activeElement || !panel.contains(document.activeElement)) {
    return false;
  }
  return document.activeElement.matches('input, textarea, select');
}

async function loadSchedules(options = {}) {
  try {
    const data = await api.getSchedules();
    const entries = data.entries || [];
    const update = {
      _schedules: entries,
      _schedulerRunning: !!data.scheduler_running,
    };
    if (options.updateFormItems !== false) {
      update._scheduleFormItems = entries.map(s => scheduleStatusToFormItem(s));
      update._scheduleFormDirty = false;
    }
    store.setState(update);
  } catch (e) {
    console.warn('loadSchedules failed:', e);
    toast.show('加载定时任务失败: ' + e.message, 'error');
  }
}

function scheduleStatusToFormItem(status) {
  const e = normalizeScheduleEntry(status.entry);
  const raw = e.schedule || '';
  let scheduleMode = 'interval';
  let scheduleValue = '';
  if (raw.startsWith('daily:')) {
    scheduleMode = 'daily';
    scheduleValue = raw.replace('daily:', '');
  } else if (raw.startsWith('interval:')) {
    scheduleMode = 'interval';
    scheduleValue = raw.replace('interval:', '');
  } else if (raw) {
    // 未知格式，尝试按 interval 解析，保留原值以便用户修正
    scheduleMode = 'interval';
    scheduleValue = raw;
  }
  return {
    id: e.id || '',
    type: e.type || 'list',
    target: e.target || '',
    users: e.users || [],
    lists: e.lists || [],
    following_names: e.following_names || [],
    name: e.name || '',
    scheduleMode,
    scheduleValue,
    enabled: e.enabled !== false,
    run_on_start: !!e.run_on_start,
    auto_follow: !!e.auto_follow,
    follow_members: !!e.follow_members,
    skip_profile: !!e.skip_profile,
    no_retry: !!e.no_retry,
  };
}

function normalizeScheduleEntry(entry) {
  return {
    id: readScheduleEntryField(entry, 'id', 'ID') || '',
    type: readScheduleEntryField(entry, 'type', 'Type') || '',
    target: readScheduleEntryField(entry, 'target', 'Target') || '',
    users: readScheduleEntryField(entry, 'users', 'Users') || [],
    lists: readScheduleEntryField(entry, 'lists', 'Lists') || [],
    following_names: readScheduleEntryField(entry, 'following_names', 'FollowingNames') || [],
    name: readScheduleEntryField(entry, 'name', 'Name') || '',
    schedule: readScheduleEntryField(entry, 'schedule', 'Schedule') || '',
    enabled: readScheduleEntryField(entry, 'enabled', 'Enabled') !== false,
    run_on_start: !!readScheduleEntryField(entry, 'run_on_start', 'RunOnStart'),
    auto_follow: !!readScheduleEntryField(entry, 'auto_follow', 'AutoFollow'),
    follow_members: !!readScheduleEntryField(entry, 'follow_members', 'FollowMembers'),
    skip_profile: !!readScheduleEntryField(entry, 'skip_profile', 'SkipProfile'),
    no_retry: !!readScheduleEntryField(entry, 'no_retry', 'NoRetry'),
  };
}

function readScheduleEntryField(entry, jsonName, legacyName) {
  if (!entry) return undefined;
  return entry[jsonName] !== undefined ? entry[jsonName] : entry[legacyName];
}

async function loadScheduleRaw() {
  try {
    const data = await api.getSchedulesRaw();
    store.setState({ _scheduleRaw: data.content || '', _scheduleExists: data.exists || false });
  } catch (e) {
    console.warn('loadScheduleRaw failed:', e);
    toast.show('加载调度原始配置失败: ' + e.message, 'error');
  }
}

async function saveScheduleRaw() {
  const content = getEditorValue(_state.scheduleCodeMirror, store.state._scheduleRaw);
  store.setState({ _scheduleRaw: content, _scheduleSaving: true });
  try {
    const validateResult = await api.validateSchedule({ raw: content });
    if (!validateResult.valid) {
      const msg = (validateResult.errors || []).join('; ');
      toast.show('校验失败: ' + msg, 'error');
      store.setState({ _scheduleSaving: false });
      return;
    }
    await api.updateSchedulesRaw(content);
    toast.show('调度配置已保存并重载');
    await loadSchedules({ updateFormItems: false });
    const rawData = await api.getSchedulesRaw();
    store.setState({
      _scheduleRaw: rawData.content || '',
      _scheduleExists: rawData.exists || false,
      _scheduleSaving: false,
    });
    setEditorValue(_state.scheduleCodeMirror, store.state._scheduleRaw || '');
  } catch (e) {
    toast.show('保存失败: ' + e.message, 'error');
    store.setState({ _scheduleSaving: false });
  }
}

async function triggerSchedule(id) {
  try {
    const data = await api.triggerSchedule(id);
    toast.show('已触发定时任务: ' + data.task_id);
  } catch (e) {
    toast.show('触发失败: ' + e.message, 'error');
  }
}

async function triggerAllSchedules() {
  const btn = document.getElementById('btnTriggerAll');
  const schedules = (store.state._schedules || []).filter(s => readScheduleEntryField(s.entry, 'enabled', 'Enabled'));
  if (schedules.length === 0) {
    toast.show('没有已启用的调度任务', 'error');
    return;
  }
  if (!confirm(`确定要触发全部 ${schedules.length} 个已启用的调度任务吗？`)) return;

  // 禁用按钮，显示 loading
  btn.disabled = true;
  btn.innerHTML = '<span class="loading-spinner"></span> 触发中...';

  try {
    const data = await api.triggerAllSchedules();
    if (data.failed > 0) {
      const errMsgs = (data.results || []).filter(r => r.error).map(r => `${r.entry_id}: ${r.error}`).join('; ');
      toast.show(`${data.succeeded} 成功, ${data.failed} 失败: ${errMsgs}`, 'error');
    } else {
      toast.show(`已全部触发成功 (${data.succeeded})`);
    }
  } catch (e) {
    toast.show('触发失败: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = '⬇️ 下载全部';
  }
}

async function toggleScheduleEnabled(id, currentEnabled) {
  try {
    await api.setScheduleEnabled(id, !currentEnabled);
    toast.show(currentEnabled ? '已禁用定时任务' : '已启用定时任务');
  } catch (e) {
    toast.show('操作失败: ' + e.message, 'error');
  }
}

function navigateToSystemSchedules() {
  if (_state.lastPage === 'system') {
    store.setState({ _systemTab: 'schedules' });
  } else {
    store.setState({ currentPage: 'system', _systemTab: 'schedules' });
    updateURL('system');
    updateNavigationUI('system');
    if (store.state.isMobile) {
      document.getElementById('sidebar').classList.remove('open');
      document.getElementById('sidebarOverlay').classList.remove('open');
    }
  }
}

function setScheduleTab(tab) {
  if (tab !== 'edit' && _state.scheduleCodeMirror) {
    _state.scheduleCodeMirror = destroyCodeMirror(_state.scheduleCodeMirror);
    _state._scheduleCmInitializing = false;
  }
  store.setState({ _scheduleTab: tab });
  if (tab === 'edit' && store.state._scheduleRaw === null) loadScheduleRaw();
  if (tab === 'form' && store.state._scheduleFormItems.length === 0 && (store.state._schedules || []).length === 0) loadSchedules();
}

_state._addScheduleItemPending = false;

function addScheduleItem() {
  if (_state._addScheduleItemPending) return;
  // 先保存当前 DOM 中的未保存编辑内容，再添加新条目
  const currentItems = readScheduleFormItemsFromDOM();
  const items = [{
    id: '',
    type: 'list',
    target: '',
    users: [],
    lists: [],
    following_names: [],
    name: '',
    scheduleMode: 'interval',
    scheduleValue: '8h',
    enabled: true,
    run_on_start: false,
    auto_follow: false,
    follow_members: false,
    skip_profile: false,
    no_retry: false,
  }, ...currentItems];
  store.setState({ _scheduleFormItems: items, _scheduleFormDirty: true });
  glowNewFirstItem('systemSchedulesPanel');
  _state._addScheduleItemPending = true;
  setTimeout(() => { _state._addScheduleItemPending = false; }, 0);
}

function clearAllScheduleValidationTimers() {
  Object.keys(_state._scheduleValidateTimers).forEach(k => {
    clearTimeout(_state._scheduleValidateTimers[k]);
    delete _state._scheduleValidateTimers[k];
  });
  Object.keys(_state._scheduleValidateRequests).forEach(k => {
    delete _state._scheduleValidateRequests[k];
  });
}

function removeScheduleItem(index) {
  clearAllScheduleValidationTimers();
  const items = readScheduleFormItemsFromDOM().filter((_, i) => i !== index);
  store.setState({ _scheduleFormItems: items, _scheduleFormDirty: true });
}

function readScheduleFormItemsFromDOM() {
  return store.state._scheduleFormItems.map((fallback, idx) => {
    const type = document.getElementById(`sf_type_${idx}`)?.value || fallback.type || 'list';
    const scheduleMode = document.getElementById(`sf_schedule_mode_${idx}`)?.value || fallback.scheduleMode || 'interval';
    const readLines = (id) => (document.getElementById(id)?.value || '').split('\n').map(s => s.trim()).filter(Boolean);
    return {
      id: fallback.id || '',
      type,
      target: type !== 'mixed' ? (document.getElementById(`sf_target_${idx}`)?.value || '') : '',
      name: document.getElementById(`sf_name_${idx}`)?.value || '',
      scheduleMode,
      scheduleValue: document.getElementById(`sf_schedule_value_${idx}`)?.value || '',
      enabled: document.getElementById(`sf_enabled_${idx}`)?.checked ?? fallback.enabled !== false,
      run_on_start: document.getElementById(`sf_run_on_start_${idx}`)?.checked ?? !!fallback.run_on_start,
      auto_follow: document.getElementById(`sf_auto_follow_${idx}`)?.checked ?? !!fallback.auto_follow,
      follow_members: document.getElementById(`sf_follow_members_${idx}`)?.checked ?? !!fallback.follow_members,
      skip_profile: document.getElementById(`sf_skip_profile_${idx}`)?.checked ?? !!fallback.skip_profile,
      no_retry: document.getElementById(`sf_no_retry_${idx}`)?.checked ?? !!fallback.no_retry,
      users: type === 'mixed' ? readLines(`sf_users_${idx}`) : [],
      lists: type === 'mixed' ? readLines(`sf_lists_${idx}`) : [],
      following_names: type === 'mixed' ? readLines(`sf_following_${idx}`) : [],
    };
  });
}

function clearScheduleValidationState(index) {
  clearTimeout(_state._scheduleValidateTimers[index]);
  delete _state._scheduleValidateTimers[index];
  delete _state._scheduleValidateRequests[index];
  setScheduleValidationAriaState(index, false);
  const clearHint = () => {
    const hint = document.getElementById(`sf_schedule_hint_${index}`);
    if (hint) hint.innerHTML = '';
  };
  clearHint();
  setTimeout(clearHint, 0);
}

function setScheduleValidationAriaState(index, invalid) {
  const fieldIds = [
    `sf_target_${index}`,
    `sf_users_${index}`,
    `sf_lists_${index}`,
    `sf_following_${index}`,
    `sf_schedule_value_${index}`,
  ];
  fieldIds.forEach(id => {
    const el = document.getElementById(id);
    if (el) el.setAttribute('aria-invalid', invalid ? 'true' : 'false');
  });
}

function updateScheduleFormItem(index, field, value) {
  const items = readScheduleFormItemsFromDOM();
  if (field === 'type') {
    clearScheduleValidationState(index);
    const prevType = items[index].type;
    items[index].type = value;
    if (value === 'mixed') {
      items[index].target = '';
    } else {
      items[index].users = [];
      items[index].lists = [];
      items[index].following_names = [];
      if (prevType === 'mixed') {
        items[index].target = '';
      }
    }
  }
  if (field === 'scheduleMode') {
    clearScheduleValidationState(index);
    items[index].scheduleMode = value;
    items[index].scheduleValue = '';
    const scheduleValue = document.getElementById(`sf_schedule_value_${index}`);
    if (scheduleValue) {
      scheduleValue.value = '';
      const label = scheduleValue.closest('.config-field')?.querySelector('.config-label');
      if (label) label.textContent = value === 'interval' ? '执行间隔' : '执行时间';
      scheduleValue.placeholder = value === 'interval' ? '例如: 2h, 30m, 6h30m, 24h' : '例如: 07:00,21:00 或 02:30';
    }
  }
  store.setState({ _scheduleFormItems: items, _scheduleFormDirty: true });
}

_state._scheduleValidateTimers = {};
_state._scheduleValidateRequests = {};
_state._scheduleValidateRequestSeq = 0;

function scheduleFieldChanged(idx) {
  clearTimeout(_state._scheduleValidateTimers[idx]);
  _state._scheduleValidateTimers[idx] = setTimeout(() => validateScheduleField(idx), 600);
}

async function validateScheduleField(idx) {
  const hint = document.getElementById(`sf_schedule_hint_${idx}`);
  if (!hint) return;

  const type = document.getElementById(`sf_type_${idx}`)?.value || 'list';
  const mode = document.getElementById(`sf_schedule_mode_${idx}`)?.value || 'interval';
  const scheduleValue = document.getElementById(`sf_schedule_value_${idx}`)?.value?.trim() || '';
  if (!scheduleValue) {
    hint.innerHTML = '';
    setScheduleValidationAriaState(idx, false);
    // schedule 为空时仍继续验证 target 等其他字段
  }

  const entry = { type, schedule: scheduleValue ? `${mode}:${scheduleValue}` : '' };

  if (type === 'mixed') {
    const usersRaw = document.getElementById(`sf_users_${idx}`)?.value || '';
    const listsRaw = document.getElementById(`sf_lists_${idx}`)?.value || '';
    const followingRaw = document.getElementById(`sf_following_${idx}`)?.value || '';
    entry.users = usersRaw.split('\n').map(s => s.trim()).filter(Boolean);
    entry.lists = listsRaw.split('\n').map(s => s.trim()).filter(Boolean);
    entry.following_names = followingRaw.split('\n').map(s => s.trim()).filter(Boolean);
  } else {
    entry.target = document.getElementById(`sf_target_${idx}`)?.value?.trim() || '';
  }

  // schedule 为空时也发送请求，让后端验证 target 等其他字段
  const requestSeq = ++_state._scheduleValidateRequestSeq;
  _state._scheduleValidateRequests[idx] = requestSeq;
  try {
    const result = await api.validateSchedule({ entries: [entry] });
    if (_state._scheduleValidateRequests[idx] !== requestSeq) return;
    if (result.valid) {
      hint.innerHTML = '';
      setScheduleValidationAriaState(idx, false);
    } else {
      const msg = (result.errors || []).join('; ');
      hint.innerHTML = `<span style="color:var(--danger, #ef4444)">✗ ${escapeHtml(msg)}</span>`;
      setScheduleValidationAriaState(idx, true);
    }
  } catch (e) {
    if (_state._scheduleValidateRequests[idx] !== requestSeq) return;
    hint.innerHTML = '';
    setScheduleValidationAriaState(idx, false);
  }
}

async function validateScheduleForm() {
  const items = readScheduleFormItemsFromDOM();
  const entries = items.map(item => ({
    type: item.type,
    target: item.type === 'mixed' ? '' : item.target.trim(),
    schedule: `${item.scheduleMode}:${item.scheduleValue.trim()}`,
    ...(item.type === 'mixed' ? {
      users: item.users || [],
      lists: item.lists || [],
      following_names: item.following_names || [],
    } : {}),
  }));
  try {
    const result = await api.validateSchedule({ entries });
    if (!result.valid) {
      const msg = (result.errors || []).join('; ');
      toast.show(msg, 'error');
      return false;
    }
  } catch (e) {
    toast.show('校验请求失败: ' + e.message, 'error');
    return false;
  }
  return true;
}

async function saveScheduleForm() {
  const items = readScheduleFormItemsFromDOM();

  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (item.type !== 'mixed' && !item.target.trim()) {
      toast.show(`规则 #${i + 1}: 目标不能为空`, 'error');
      return;
    }
    if (item.type === 'mixed' && !item.users.length && !item.lists.length && !item.following_names.length) {
      toast.show(`规则 #${i + 1}: 混合任务至少需要一个目标`, 'error');
      return;
    }
    if (!item.scheduleValue.trim()) {
      toast.show(`规则 #${i + 1}: 调度值不能为空`, 'error');
      return;
    }
  }

  if (!(await validateScheduleForm())) return;

  const schedules = items.map(item => ({
    id: item.id || '',
    type: item.type,
    target: item.type === 'mixed' ? '' : item.target.trim(),
    users: item.type === 'mixed' ? (item.users || []) : [],
    lists: item.type === 'mixed' ? (item.lists || []) : [],
    following_names: item.type === 'mixed' ? (item.following_names || []) : [],
    name: item.name.trim(),
    schedule: `${item.scheduleMode}:${item.scheduleValue.trim()}`,
    enabled: item.enabled,
    run_on_start: item.run_on_start,
    auto_follow: item.auto_follow,
    follow_members: item.follow_members,
    skip_profile: item.skip_profile,
    no_retry: item.no_retry,
  }));

  store.setState({ _scheduleFormItems: items, _scheduleSaving: true });
  try {
    const saved = await api.replaceSchedules(schedules);
    if (saved?.entries) {
      store.setState({
        _scheduleFormItems: saved.entries.map(entry => scheduleStatusToFormItem({ entry })),
        _scheduleFormDirty: false,
      });
    }
    await loadSchedules({ updateFormItems: false });
    toast.show('调度配置已保存并重载');
    const rawData = await api.getSchedulesRaw();
    store.setState({
      _scheduleRaw: rawData.content || '',
      _scheduleExists: rawData.exists || false,
      _scheduleSaving: false,
    });
  } catch (e) {
    toast.show('保存失败: ' + e.message, 'error');
    store.setState({ _scheduleSaving: false });
  }
}

_state.scheduleCodeMirror = null;
_state._scheduleCmInitializing = false;

async function initScheduleCodeMirror() {
  if (_state._scheduleCmInitializing || _state.scheduleCodeMirror) return;
  _state._scheduleCmInitializing = true;
  await waitForCodeMirror(3000);
  if (document.getElementById('scheduleEditorContainer')) {
    _state.scheduleCodeMirror = initCodeMirror('scheduleEditorContainer', store.state._scheduleRaw, 'yaml');
  }
  _state._scheduleCmInitializing = false;
}

function syncScheduleTabView() {
  if (store.state._schedules === null && !store.state.sseConnected) loadSchedules();
  if (store.state._scheduleTab === 'edit' && store.state._scheduleRaw === null) loadScheduleRaw();
  if (store.state._scheduleTab === 'edit' && !_state.scheduleCodeMirror) requestAnimationFrame(() => requestAnimationFrame(initScheduleCodeMirror));
}

function renderServerClosedState() {
  const el = document.getElementById('contentContainer');
  if (!el) return;
  el.innerHTML = `
    <div class="empty-state" style="padding: 80px 20px;">
      <div style="font-size: 48px; margin-bottom: 20px;">👋</div>
      <div class="empty-title">服务器已关闭</div>
      <div class="empty-desc">如需重新启动，请运行 tmd -server</div>
    </div>
  `;
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function saveConfigForm() {
  const inputs = document.querySelectorAll('.config-input:not(.cookie-input)[name]');
  const fields = {};
  for (const el of inputs) {
    if (el.type === 'password' && el.value.trim() === '') { fields[el.name] = '__KEEP_OLD__'; continue; }
    if (el.type === 'number') {
      const val = parseInt(el.value, 10);
      const min = el.min !== '' ? parseInt(el.min, 10) : 1;
      const max = el.max !== '' ? parseInt(el.max, 10) : (el.name.includes('routine') ? 100 : 250);
      if (isNaN(val) || val < min || val > max) {
        toast.show(`${el.name} 必须在 ${min}-${max} 之间`, 'error');
        return;
      }
    }
    fields[el.name] = el.value;
  }

  store.setState({ configSaving: true });
  try {
    const data = await api.saveConfigFields(fields);
    store.setState({
      configSaving: false,
      configFields: data.fields || store.state.configFields,
      configRaw: data.yaml_preview || store.state.configRaw
    });
    showManualRestartNotice('配置');
  } catch (e) {
    toast.show('❌ 保存失败: ' + e.message, 'error');
    store.setState({ configSaving: false });
  }
}

async function saveConfig() {
  const content = getEditorValue(_state.configCodeMirror, store.state.configRaw);
  if (!content.trim()) return toast.show('配置不能为空', 'error');
  store.setState({ configRaw: content, configSaving: true });
  try {
    const data = await api.updateConfigRaw(content);
    store.setState({
      configSaving: false,
      configRaw: data.yaml_preview || content
    });
    showManualRestartNotice('配置');
  } catch (e) {
    toast.show('❌ 保存失败: ' + e.message, 'error');
    store.setState({ configSaving: false });
  }
}

async function loadCookiesItems() {
  store.setState({ _cookiesLoading: true });
  try {
    const d = await api.getCookies();
    store.setState({ cookieItems: d.items || [], cookiesExists: d.exists || false, _cookiesLoading: false });
  } catch (e) {
    store.setState({ _cookiesLoading: false });
    toast.show('加载额外账户失败: ' + e.message, 'error');
  }
}

async function loadCookiesRaw() {
  try {
    const d = await api.getCookiesRaw();
    store.setState({ cookiesRaw: d.content || '', cookiesExists: d.exists || false });
  } catch (e) { toast.show('加载额外账户失败: ' + e.message, 'error'); }
}

function isCookiesFormDirty() {
  if (store.state.cookiesMode !== 'form') return false;
  const inputs = document.querySelectorAll('#systemCookiesPanel input.cookie-input');
  return Array.from(inputs).some(input => input.value.trim() !== '');
}

function isCookiesRawDirty() {
  return store.state.cookiesMode === 'raw' && _state.cookiesCodeMirror && getEditorValue(_state.cookiesCodeMirror, store.state.cookiesRaw) !== (store.state.cookiesRaw || '');
}

function refreshCookiesAfterReconnect() {
  if (isPanelInputFocused('systemCookiesPanel') || isCookiesFormDirty() || isCookiesRawDirty()) return;
  if (store.state.cookiesMode === 'raw') loadCookiesRaw();
  else loadCookiesItems();
}

async function saveCookiesForm() {
  const cookies = [];
  const items = store.state.cookieItems;

  for (let i = 0; i < items.length; i++) {
    const authInput = document.getElementById(`cookie_auth_${i}`);
    const ct0Input = document.getElementById(`cookie_ct0_${i}`);
    const authVal = authInput ? authInput.value.trim() : '';
    const ct0Val = ct0Input ? ct0Input.value.trim() : '';
    const originalIndex = Number.isInteger(items[i].index) ? items[i].index : null;
    const isNewAccount = originalIndex === null;

    if (isNewAccount && !authVal && !ct0Val) {
      toast.show(`账户 #${i + 1} 的 Auth Token 和 CT0 不能同时为空`, 'error');
      return;
    }

    cookies.push({
      index: originalIndex,
      auth_token: (isNewAccount || authVal) ? authVal : '__KEEP_OLD__',
      ct0: (isNewAccount || ct0Val) ? ct0Val : '__KEEP_OLD__',
    });
  }

  store.setState({ cookiesSaving: true });
  try {
    await api.saveCookies(cookies);
    store.setState({ cookiesSaving: false });
    showManualRestartNotice('额外账户');
  } catch (e) {
    toast.show('❌ 保存失败: ' + e.message, 'error');
    store.setState({ cookiesSaving: false });
  }
}

async function saveCookies() {
  const content = getEditorValue(_state.cookiesCodeMirror, store.state.cookiesRaw);
  if (!content.trim()) return toast.show('内容不能为空', 'error');

  store.setState({ cookiesRaw: content, cookiesSaving: true });
  try {
    await api.updateCookiesRaw(content);
    store.setState({ cookiesSaving: false, cookiesRaw: content });
    showManualRestartNotice('额外账户');
  } catch (e) {
    toast.show('❌ 保存失败: ' + e.message, 'error');
    store.setState({ cookiesSaving: false });
  }
}

function setCookiesMode(mode) {
  if (mode !== 'raw' && _state.cookiesCodeMirror) {
    _state.cookiesCodeMirror = destroyCodeMirror(_state.cookiesCodeMirror);
  }
  store.setState({ cookiesMode: mode });
  if (mode === 'raw' && store.state.cookiesRaw === null) loadCookiesRaw();
}

function addCookieAccount() {
  const items = [{ index: null, auth_token: '', ct0: '' }, ...store.state.cookieItems];
  store.setState({ cookieItems: items });
  glowNewFirstItem('systemCookiesPanel');
}

function removeCookieAccount(index) {
  const items = store.state.cookieItems.filter((_, i) => i !== index);
  store.setState({ cookieItems: items });
}

async function shutdownServer() {
  if (!confirm('确定要关闭服务器吗？\n\n关闭后需要手动重新启动 TMD 服务。')) {
    return;
  }

  toast.show('正在关闭服务器...', 'warning');
  cleanupSystemTimers();

  try {
    await api.shutdownServer();
  } catch (err) {
    sseManager.disconnect();
    renderServerClosedState();
  }
}

function handleServerShutdown(message) {
  cleanupSystemTimers();
  api.abortAll();
  sseManager.disconnect();
  destroyAllEditors();
  renderServerClosedState();
}

function setConfigMode(mode) {
  if (mode !== 'raw' && _state.configCodeMirror) {
    _state.configCodeMirror = destroyCodeMirror(_state.configCodeMirror);
  }
  store.setState({ configMode: mode });
  if (mode === 'raw' && store.state.configRaw === null) loadConfigRaw();
}

async function loadLogs() {
  const { logLevel, logSearch, logPagination } = store.state;
  const p = new URLSearchParams();
  if (logLevel !== 'all') p.append('level', logLevel);
  if (logSearch) p.append('q', logSearch);
  p.append('page', logPagination.page);
  p.append('pageSize', logPagination.pageSize);
  store.setState({ _logsLoading: true });
  try {
    const d = await api.getLogs('?' + p.toString());
    store.setState({ logs: d.logs || [], logPagination: { page: d.page, pageSize: d.pageSize, total: d.total, totalPages: d.totalPages }, _logsLoading: false });
  } catch (e) {
    store.setState({ _logsLoading: false });
    toast.show('加载日志失败: ' + e.message, 'error');
  }

  // 异步获取统计（不阻塞日志加载）
  api.getLogStats()
    .then(s => store.setState({ logStats: { debug: s.debug || 0, info: s.info || 0, warn: s.warn || 0, error: s.error || 0, total: s.total || 0 } }))
    .catch(() => {});
}

function setLogLevel(level) {
  store.setState({ logLevel: level, logPagination: { ...store.state.logPagination, page: 1 } });
  loadLogs();
  restartLogStreamIfNeeded();
}

_state._logSearchTimer = null;
function onLogSearchInput(value) {
  store.setState({ logSearch: value, logPagination: { ...store.state.logPagination, page: 1 } });
  clearTimeout(_state._logSearchTimer);
  _state._logSearchTimer = setTimeout(() => {
    loadLogs();
    restartLogStreamIfNeeded();
  }, 300);
}

function changeLogPage(delta) {
  const p = store.state.logPagination;
  const np = p.page + delta;
  if (np >= 1 && np <= p.totalPages) { store.setState({ logPagination: { ...p, page: np } }); loadLogs(); }
}

function goToLogPage(page) {
  const p = store.state.logPagination;
  if (page >= 1 && page <= p.totalPages) { store.setState({ logPagination: { ...p, page } }); loadLogs(); }
}

_state.logAutoRefreshTimer = null;
_state.logStreamConn = null;
_state.configCodeMirror = null;
_state.cookiesCodeMirror = null;
_state._configCmInitializing = false;
_state._cookiesCmInitializing = false;

_state._cmWaitCancelled = false;

function waitForCodeMirror(maxWait) {
  _state._cmWaitCancelled = false;
  if (typeof CodeMirror !== 'undefined') return Promise.resolve(true);
  // Dynamically load CodeMirror CSS and JS only when first needed
  return loadCodeMirrorAssets().then(() => {
    if (typeof CodeMirror !== 'undefined') return true;
    return new Promise(resolve => {
      const start = Date.now();
      const check = () => {
        if (_state._cmWaitCancelled || typeof CodeMirror !== 'undefined' || Date.now() - start > maxWait) {
          resolve(!_state._cmWaitCancelled && typeof CodeMirror !== 'undefined');
        } else {
          setTimeout(check, 100);
        }
      };
      check();
    });
  });
}

function loadCodeMirrorAssets() {
  return new Promise((resolve) => {
    // Check if already loaded
    if (document.querySelector('link[href*="codemirror.min.css"]')) {
      resolve();
      return;
    }
    // Load CSS files
    const cssUrls = [
      'https://cdn.jsdelivr.net/npm/codemirror@5.65.18/lib/codemirror.min.css',
      'https://cdn.jsdelivr.net/npm/codemirror@5.65.18/theme/material-darker.min.css'
    ];
    let loaded = 0;
    cssUrls.forEach(url => {
      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = url;
      link.onload = () => { loaded++; if (loaded === cssUrls.length) loadScripts(); };
      link.onerror = () => { loaded++; if (loaded === cssUrls.length) loadScripts(); };
      document.head.appendChild(link);
    });
    function loadScripts() {
      const scripts = [
        'https://cdn.jsdelivr.net/npm/codemirror@5.65.18/lib/codemirror.min.js',
        'https://cdn.jsdelivr.net/npm/codemirror@5.65.18/mode/yaml/yaml.min.js'
      ];
      let scriptLoaded = 0;
      scripts.forEach(src => {
        const script = document.createElement('script');
        script.src = src;
        script.onload = () => { scriptLoaded++; if (scriptLoaded === scripts.length) { setTimeout(resolve, 50); } };
        script.onerror = () => { scriptLoaded++; if (scriptLoaded === scripts.length) { setTimeout(resolve, 50); } };
        document.body.appendChild(script);
      });
    }
  });
}

function initCodeMirror(containerId, content, mode) {
  const container = document.getElementById(containerId);
  if (!container) return null;

  if (typeof CodeMirror === 'undefined') {
    container.innerHTML = '';
    const textarea = document.createElement('textarea');
    textarea.className = 'form-textarea config-editor';
    textarea.spellcheck = false;
    textarea.value = content;
    container.appendChild(textarea);
    return textarea;
  }

  container.innerHTML = '';

  const cm = CodeMirror(container, {
    value: content,
    mode: mode || 'yaml',
    theme: 'material-darker',
    lineNumbers: true,
    lineWrapping: true,
    tabSize: 2,
    indentWithTabs: false,
    matchBrackets: true,
    autoCloseBrackets: true,
  });

  cm.setSize('100%', '100%');
  return cm;
}

function destroyCodeMirror(editor) {
  if (!editor) return null;
  if (typeof editor.toTextArea === 'function') editor.toTextArea();
  return null;
}

function getEditorValue(editor, fallback = '') {
  if (!editor) return fallback || '';
  if (typeof editor.getValue === 'function') return editor.getValue();
  return editor.value || fallback || '';
}

function setEditorValue(editor, value) {
  if (!editor) return;
  if (typeof editor.setValue === 'function') {
    editor.setValue(value);
  } else {
    editor.value = value;
  }
}

async function initConfigCodeMirror() {
  if (_state._configCmInitializing || _state.configCodeMirror) return;
  _state._configCmInitializing = true;
  await waitForCodeMirror(3000);
  if (document.getElementById('configEditorContainer')) {
    _state.configCodeMirror = initCodeMirror('configEditorContainer', store.state.configRaw, 'yaml');
  }
  _state._configCmInitializing = false;
}

async function initCookiesCodeMirror() {
  if (_state._cookiesCmInitializing || _state.cookiesCodeMirror) return;
  _state._cookiesCmInitializing = true;
  await waitForCodeMirror(3000);
  if (document.getElementById('cookiesEditorContainer')) {
    _state.cookiesCodeMirror = initCodeMirror('cookiesEditorContainer', store.state.cookiesRaw, 'yaml');
  }
  _state._cookiesCmInitializing = false;
}

function toggleLogAutoRefresh() {
  const ns = !store.state.logAutoRefresh;
  store.setState({ logAutoRefresh: ns });
  if (ns) {
    store.setState({ _logAutoScrollPaused: false, _logNewArrived: false });
    _state._logScrollListenerAttached = false;
    startLogStream();
  }
  else stopLogStream();
}

function cleanupSystemTimers() {
  _state._cmWaitCancelled = true;
  _state._logsPageLoaded = false;
  if (_state.logAutoRefreshTimer) {
    clearTimeout(_state.logAutoRefreshTimer);
    _state.logAutoRefreshTimer = null;
  }
  clearAllScheduleValidationTimers();
  stopLogStream();
}

function destroyAllEditors() {
  _state.configCodeMirror = destroyCodeMirror(_state.configCodeMirror);
  _state._configCmInitializing = false;
  _state.cookiesCodeMirror = destroyCodeMirror(_state.cookiesCodeMirror);
  _state._cookiesCmInitializing = false;
  _state.scheduleCodeMirror = destroyCodeMirror(_state.scheduleCodeMirror);
  _state._scheduleCmInitializing = false;
}

function buildLogStreamURL() {
  const { logLevel, logSearch } = store.state;
  const p = new URLSearchParams();
  if (logLevel !== 'all') p.append('level', logLevel);
  if (logSearch) p.append('q', logSearch);
  const qs = p.toString();
  return `/api/v1/logs/stream${qs ? '?' + qs : ''}`;
}

_state._logStreamConnecting = false;
_state._pendingLogStreamConn = null;
_state._logsPageLoaded = false;
_state._logReconnectAttempts = 0;

function startLogStream() {
  if (_state.logStreamConn || _state._logStreamConnecting || store.state.currentPage !== 'logs') return;
  _state._logScrollListenerAttached = false;
  _state._logStreamConnecting = true;
  const conn = new EventSource(buildLogStreamURL());
  _state._pendingLogStreamConn = conn;
  conn.onopen = () => {
    _state._pendingLogStreamConn = null;
    _state.logStreamConn = conn;
    _state._logStreamConnecting = false;
    _state._logReconnectAttempts = 0;
    // 挂载日志容器的滚动监听
    attachLogScrollListener();
  };
  conn.addEventListener('log', (e) => {
    const line = e.data || '';
    if (!line) return;
    const logs = [line, ...store.state.logs].slice(0, 1000);
    store.setState({ logs });
    setTimeout(() => {
      const el = document.getElementById('logContainer');
      if (!el) return;
      if (store.state._logAutoScrollPaused) {
        store.setState({ _logNewArrived: true });
      } else {
        el.scrollTop = 0;
      }
    }, 0);
  });
  conn.onerror = () => {
    _state._pendingLogStreamConn = null;
    _state._logStreamConnecting = false;
    if (conn === _state.logStreamConn) {
      _state.logStreamConn.close();
      _state.logStreamConn = null;
    } else {
      conn.close();
    }
    if (store.state.logAutoRefresh && store.state.currentPage === 'logs') {
      _state._logReconnectAttempts++;
      if (_state._logReconnectAttempts > 30) {
        store.setState({ logAutoRefresh: false });
        toast.show('日志流重连失败次数过多，已停止自动重连，请手动刷新', 'warning');
        return;
      }
      const delay = Math.min(2000 * Math.pow(2, _state._logReconnectAttempts - 1), 30000);
      _state.logAutoRefreshTimer = setTimeout(() => { _state.logAutoRefreshTimer = null; startLogStream(); }, delay);
    }
  };
}

function stopLogStream() {
  _state._logStreamConnecting = false;
  if (_state._pendingLogStreamConn) {
    _state._pendingLogStreamConn.close();
    _state._pendingLogStreamConn = null;
  }
  if (_state.logAutoRefreshTimer) {
    clearTimeout(_state.logAutoRefreshTimer);
    _state.logAutoRefreshTimer = null;
  }
  if (_state.logStreamConn) {
    _state.logStreamConn.close();
    _state.logStreamConn = null;
  }
}

_state._logScrollHandler = null;

function detachLogScrollListener() {
  const el = document.getElementById('logContainer');
  if (_state._logScrollHandler) {
    el?.removeEventListener('scroll', _state._logScrollHandler);
    _state._logScrollHandler = null;
  }
}

function attachLogScrollListener() {
  // 先移除旧的监听器，防止多次调用累积
  detachLogScrollListener();
  const el = document.getElementById('logContainer');
  if (!el) return;
  _state._logScrollHandler = () => {
    if (el.scrollTop > 50) {
      if (!store.state._logAutoScrollPaused) {
        store.setState({ _logAutoScrollPaused: true });
      }
    } else {
      if (store.state._logAutoScrollPaused || store.state._logNewArrived) {
        store.setState({ _logAutoScrollPaused: false, _logNewArrived: false });
      }
    }
  };
  el.addEventListener('scroll', _state._logScrollHandler);
}

function scrollLogToTop() {
  const el = document.getElementById('logContainer');
  if (el) el.scrollTop = 0;
  store.setState({ _logAutoScrollPaused: false, _logNewArrived: false });
}

function restartLogStreamIfNeeded() {
  if (!store.state.logAutoRefresh) return;
  _state._logScrollListenerAttached = false;
  store.setState({ _logAutoScrollPaused: false, _logNewArrived: false });
  stopLogStream();
  startLogStream();
}

function syncConfigTabView() {
  if (store.state.configMode === 'form' && (!store.state.configFields || store.state.configFields.length === 0)) {
    loadConfigFields();
  }
  if (store.state.configMode === 'raw' && store.state.configRaw === null) {
    loadConfigRaw();
  }
  if (store.state.configMode === 'raw' && !_state.configCodeMirror) {
    requestAnimationFrame(() => requestAnimationFrame(initConfigCodeMirror));
  }
}

function syncCookiesTabView() {
  if (store.state.cookiesMode === 'form' && (!store.state.cookieItems || store.state.cookieItems.length === 0)) {
    loadCookiesItems();
  }
  if (store.state.cookiesMode === 'raw' && store.state.cookiesRaw === null) {
    loadCookiesRaw();
  }
  if (store.state.cookiesMode === 'raw' && !_state.cookiesCodeMirror) {
    requestAnimationFrame(() => requestAnimationFrame(initCookiesCodeMirror));
  }
}

function syncLogsPageView() {
  if (!_state._logsPageLoaded) {
    _state._logsPageLoaded = true;
    loadLogs();
  }
  if (store.state.logAutoRefresh) startLogStream();
}

function syncSystemTabView() {
  if (store.state.currentPage !== 'system') return;

  if (store.state._systemTab === 'config') syncConfigTabView();
  if (store.state._systemTab === 'cookies') syncCookiesTabView();
  if (store.state._systemTab === 'schedules') syncScheduleTabView();
}

function rerenderSystemPanel(panelId, renderFn, resetEditor = null, initEditor = null, saveFn = null, restoreFn = null) {
  const saved = saveFn ? saveFn() : null;
  if (resetEditor) resetEditor();
  const panel = document.getElementById(panelId);
  if (panel) panel.innerHTML = renderFn();
  if (initEditor) requestAnimationFrame(() => requestAnimationFrame(() => {
    initEditor();
    if (restoreFn && saved !== null) restoreFn(saved);
  }));
}

function setSystemTab(tab) {
  store.setState({ _systemTab: tab });
  setTimeout(syncSystemTabView, 0);
}

// ============================================
// Navigation & Routing
// ============================================

// Shared route mappings (single source of truth)
const ROUTE_TO_PAGE = { '/': 'overview', '/tasks': 'tasks', '/data': 'data', '/schedules': 'schedules', '/system': 'system', '/logs': 'logs' };
const PAGE_TO_ROUTE = { overview: '/', tasks: '/tasks', data: '/data', schedules: '/schedules', system: '/system', logs: '/logs' };
const HASH_TO_SUB = { 'users': 'users', 'lists': 'lists', 'entities': 'entities', 'list-entities': 'listEntities', 'user-links': 'userLinks', 'previous-names': 'previousNames' };
const SUB_TO_HASH = { 'users': '', 'lists': '#lists', 'entities': '#entities', 'listEntities': '#list-entities', 'userLinks': '#user-links', 'previousNames': '#previous-names' };
const PAGE_TITLES = { overview: '概览', tasks: '任务中心', data: '数据管理', schedules: '定时任务', system: '应用配置', logs: '系统日志' };

function updateNavigationUI(page) {
  document.querySelectorAll('.nav-item').forEach(el => el.classList.toggle('active', el.dataset.page === page));
  document.querySelectorAll('.mobile-nav-item').forEach(el => el.classList.toggle('active', el.dataset.page === page));
  document.getElementById('pageTitle').textContent = PAGE_TITLES[page] || '概览';
}

// Parse URL to determine current page
function parseRoute() {
  const path = window.location.pathname;
  const hash = window.location.hash.slice(1); // Remove #
  
  const page = ROUTE_TO_PAGE[path] || 'overview';
  const dataSubPage = HASH_TO_SUB[hash] || 'users';
  
  return { page, dataSubPage };
}

// Update URL based on current page
function updateURL(page, dataSubPage = null) {
  const path = PAGE_TO_ROUTE[page] || '/';
  const hash = (page === 'data' && dataSubPage) ? SUB_TO_HASH[dataSubPage] : '';
  
  // Use history API to update URL without reloading
  const newUrl = path + hash;
  if (window.location.pathname + window.location.hash !== newUrl) {
    window.history.pushState({ page, dataSubPage }, '', newUrl);
  }
}

function navigateTo(page) {
  drawer.close();
  if ((_state.lastPage === 'system' || _state.lastPage === 'logs') && page !== _state.lastPage) {
    cleanupSystemTimers();
    destroyAllEditors();
  }
  store.setState({ currentPage: page });
  
  // Update URL
  updateURL(page, store.state.dataSubPage);
  
  // Update sidebar, mobile nav, and title
  updateNavigationUI(page);
  
  // Close sidebar on mobile
  if (store.state.isMobile) {
    document.getElementById('sidebar').classList.remove('open');
    document.getElementById('sidebarOverlay').classList.remove('open');
  }
  
  // Note: render() is called by subscribe callback when page changes
}

// Handle browser back/forward buttons
window.onpopstate = (event) => {
  const { page, dataSubPage } = parseRoute();
  if ((_state.lastPage === 'system' || _state.lastPage === 'logs') && page !== _state.lastPage) {
    cleanupSystemTimers();
    destroyAllEditors();
  }
  
  if (page === 'data' && dataSubPage !== store.state.dataSubPage) {
    store.setState({ 
      currentPage: page,
      dataSubPage: dataSubPage 
    });
  } else {
    store.setState({ currentPage: page });
  }
  
  // Update sidebar, mobile nav, and title
  updateNavigationUI(page);
};

function setDataSubPage(subPage) {
  store.setState({
    dataSubPage: subPage,
    dbPagination: {
      ...store.state.dbPagination,
      [subPage]: { page: 1, pageSize: 200, totalPages: 1 }
    },
    dbSearch: {
      ...store.state.dbSearch,
      [subPage]: ''
    },
    _prevNameUserIdFilter: subPage === 'previousNames' ? store.state._prevNameUserIdFilter : ''
  });
  
  // Update URL when changing data sub-page
  updateURL('data', subPage);
  
  // Note: render() is called by subscribe callback for data page
  refreshDBData();
}

function render() {
  const container = document.getElementById('contentContainer');
  const page = store.state.currentPage;

  if (pages[page]) {
    let scrollPos = 0;
    let scrollEl = null;
    if (page === 'schedules') {
      scrollEl = container.querySelector('.schedule-list') || container;
      scrollPos = scrollEl.scrollTop;
    }

    container.innerHTML = pages[page]();

    if (page === 'schedules' && scrollEl) {
      requestAnimationFrame(() => {
        const target = container.querySelector('.schedule-list') || container;
        if (target) target.scrollTop = scrollPos;
      });
    }
    if (page === 'system') {
      // Defer to avoid re-entering store subscription via loadConfigFields() -> setState
      setTimeout(() => syncSystemTabView(), 0);
    } else if (page === 'logs') {
      syncLogsPageView();
    }
    
    // Restore filter and search values
      restoreSearchValue('taskFilter', 'taskFilter');
      restoreSearchValue('taskSearch', 'taskSearch');

    // Restore search value for data page
    if (page === 'data') {
      restoreSearchValue('dbSearchInput', 'dbSearch', store.state.dataSubPage);
    }

    if (page === 'schedules') {
      if (store.state._schedules === null) loadSchedules();
    }
    
    // Restore search value for logs
    if (page === 'logs') {
      restoreSearchValue('logSearchInput', 'logSearch');
    }
  }
}

// Filter tasks based on status and search
function filterTasks() {
  // Reuse updateTaskListUI to render filtered tasks
  updateTaskListUI(store.state.tasks);
}

// ============================================
// Initialization
// ============================================

async function init() {
  const { page, dataSubPage } = parseRoute();
  updateNavigationUI(page);

  document.getElementById('contentContainer').innerHTML = `
    <div class="empty-state">
      <div class="skeleton" style="width: 64px; height: 64px; border-radius: var(--radius-xl); margin-bottom: var(--space-4);"></div>
      <div class="empty-title">加载中...</div>
      <div class="empty-desc">正在初始化应用数据</div>
    </div>
  `;

  sseManager.connect();

  try {
    const [health, tasks] = await Promise.all([
      api.getHealth(),
      api.getTasks()
    ]);

    store.setState({
      currentPage: page,
      dataSubPage: dataSubPage,
      health,
      tasks: store.state.tasks.length > 0 ? store.state.tasks : (tasks.tasks || []),
    });

    await refreshDBData();

  } catch (err) {
    store.setState({
      currentPage: page,
      dataSubPage: dataSubPage
    });
    toast.show('加载数据失败: ' + err.message, 'error');
  }

  render();
}

// Event Listeners
document.getElementById('menuToggle').onclick = () => {
  const sb = document.getElementById('sidebar');
  const ov = document.getElementById('sidebarOverlay');
  sb.classList.toggle('open');
  ov.classList.toggle('open', sb.classList.contains('open'));
  document.getElementById('menuToggle').setAttribute('aria-expanded', sb.classList.contains('open'));
};

document.getElementById('sidebarOverlay').onclick = () => {
  document.getElementById('sidebar').classList.remove('open');
  document.getElementById('sidebarOverlay').classList.remove('open');
};

document.querySelectorAll('.nav-item').forEach(el => {
  el.onclick = () => navigateTo(el.dataset.page);
});

document.querySelectorAll('.mobile-nav-item').forEach(el => {
  el.onclick = () => navigateTo(el.dataset.page);
});

document.getElementById('sseIndicator').onclick = () => {
  const page = store.state.currentPage;
  if (page === 'tasks') refreshTasks();
  else if (page === 'data') refreshDBData();
  else if (page === 'schedules') loadSchedules();
  else if (page === 'logs') loadLogs();
  else init();
};

// Handle window resize
window.addEventListener('resize', () => {
  const isMobile = window.innerWidth < 768;
  if (isMobile !== store.state.isMobile) {
    store.setState({ isMobile });
  }
});

// Subscribe to state changes
_state.lastPage = store.state.currentPage;
_state.lastTasksJson = JSON.stringify(store.state.tasks);
_state.lastSystemTab = store.state._systemTab;
_state.lastLogPaginationJson = JSON.stringify(store.state.logPagination);
_state.lastConfigRaw = store.state.configRaw;
_state.lastConfigSaving = store.state.configSaving;
_state.lastConfigFieldsJson = JSON.stringify(store.state.configFields);
_state.lastConfigFieldsLoading = store.state.configFieldsLoading;
_state.lastConfigMode = store.state.configMode;
_state.lastCookiesRaw = store.state.cookiesRaw;
_state.lastCookiesSaving = store.state.cookiesSaving;
_state.lastCookieItemsJson = JSON.stringify(store.state.cookieItems);
_state.lastCookiesMode = store.state.cookiesMode;
_state.lastLogsLength = store.state.logs.length;
_state.lastLogLevel = store.state.logLevel;
_state.lastLogNewArrived = store.state._logNewArrived;
_state.lastDataSubPage = store.state.dataSubPage;
_state.lastDbDataJson = JSON.stringify(store.state.dbData);
_state.lastDbPaginationJson = JSON.stringify(store.state.dbPagination);
_state.lastDbSortJson = JSON.stringify(store.state.dbSort);
_state.lastSchedulesJson = JSON.stringify(store.state._schedules);
_state.lastScheduleRaw = store.state._scheduleRaw;
_state.lastScheduleExists = store.state._scheduleExists;
_state.lastScheduleSaving = store.state._scheduleSaving;
_state.lastScheduleTab = store.state._scheduleTab;
_state.lastScheduleFormItemsJson = JSON.stringify(store.state._scheduleFormItems);
_state.lastSchedulerRunning = store.state._schedulerRunning;

// ============================================
// Page-specific state sync functions
// ============================================

function syncDataPage(state) {
  const dataSubPageChanged = state.dataSubPage !== _state.lastDataSubPage;
  const dbDataChanged = JSON.stringify(state.dbData) !== _state.lastDbDataJson;
  const dbPaginationChanged = JSON.stringify(state.dbPagination) !== _state.lastDbPaginationJson;
  const dbSortChanged = JSON.stringify(state.dbSort) !== _state.lastDbSortJson;

  if (!dataSubPageChanged && !dbDataChanged && !dbPaginationChanged && !dbSortChanged) return;

  _state.lastDataSubPage = state.dataSubPage;
  _state.lastDbDataJson = JSON.stringify(state.dbData);
  _state.lastDbPaginationJson = JSON.stringify(state.dbPagination);
  _state.lastDbSortJson = JSON.stringify(state.dbSort);

  // 子页面切换（如 Users→Lists）：全量重建（tab 切换需要重新渲染标题）
  if (dataSubPageChanged) { render(); return; }

  // 仅数据/排序/分页变化：局部更新表格 + 分页栏，保留标签页和搜索状态
  const subPage = state.dataSubPage;
  const current = state.dbData[subPage] || { data: [], total: 0 };
  const pagination = state.dbPagination[subPage] || { page: 1, pageSize: 200, totalPages: 1 };
  const sort = state.dbSort[subPage] || { sortBy: 'id', sortOrder: 'desc' };

  const tableEl = document.getElementById('dataTableContainer');
  if (tableEl) tableEl.innerHTML = renderDBTable(subPage, current.data, sort);

  const mobileEl = document.getElementById('dataMobileCards');
  if (mobileEl) mobileEl.innerHTML = renderDBMobileCards(subPage, current.data);

  const pagEl = document.getElementById('dataPagination');
  if (pagEl) {
    const infoEl = pagEl.querySelector('#dataPaginationInfo');
    if (infoEl) {
      infoEl.innerHTML = `显示 ${current.data.length || 0} / ${current.total || 0} 条记录 (第 ${pagination.page} / ${pagination.totalPages} 页)`;
    }
    const controlsEl = pagEl.querySelector('.pagination-controls');
    if (controlsEl) {
      controlsEl.innerHTML = `
        <button class="page-btn" onclick="changeDBPage(-1)" ${pagination.page <= 1 ? 'disabled' : ''}>←</button>
        ${renderPageNumbers(pagination.page, pagination.totalPages)}
        <button class="page-btn" onclick="changeDBPage(1)" ${pagination.page >= pagination.totalPages ? 'disabled' : ''}>→</button>
      `;
    }
  }
}

function syncSystemPage(state, tasksChanged) {
  const tabChanged = state._systemTab !== _state.lastSystemTab;
  const configRawChanged = state.configRaw !== _state.lastConfigRaw;
  const configSavingChanged = state.configSaving !== _state.lastConfigSaving;
  const configFieldsChanged = JSON.stringify(state.configFields) !== _state.lastConfigFieldsJson;
  const configFieldsLoadingChanged = state.configFieldsLoading !== _state.lastConfigFieldsLoading;
  const configModeChanged = state.configMode !== _state.lastConfigMode;
  const cookiesChanged = JSON.stringify(state.cookieItems) !== _state.lastCookieItemsJson;
  const cookiesModeChanged = state.cookiesMode !== _state.lastCookiesMode;
  const cookiesRawChanged = state.cookiesRaw !== _state.lastCookiesRaw;
  const cookiesSavingChanged = state.cookiesSaving !== _state.lastCookiesSaving;
  const schedulesChanged = JSON.stringify(state._schedules) !== _state.lastSchedulesJson;
  const scheduleRawChanged = state._scheduleRaw !== _state.lastScheduleRaw;
  const scheduleExistsChanged = state._scheduleExists !== _state.lastScheduleExists;
  const scheduleSavingChanged = state._scheduleSaving !== _state.lastScheduleSaving;
  const scheduleTabChanged = state._scheduleTab !== _state.lastScheduleTab;
  const scheduleFormItemsChanged = JSON.stringify(state._scheduleFormItems) !== _state.lastScheduleFormItemsJson;

  if (tabChanged) {
    _state.lastSystemTab = state._systemTab;
    document.querySelectorAll('.system-tabs .tab').forEach(t => {
      t.classList.toggle('active', t.dataset.tab === state._systemTab);
    });
    document.getElementById('systemConfigPanel').style.display = state._systemTab === 'config' ? '' : 'none';
    document.getElementById('systemCookiesPanel').style.display = state._systemTab === 'cookies' ? '' : 'none';
    document.getElementById('systemSchedulesPanel').style.display = state._systemTab === 'schedules' ? '' : 'none';
  }

  const configRawRebuildNeeded = configRawChanged && _state.lastConfigRaw === null && state.configRaw !== null;
  const configPanelShouldRebuild = state.configMode === 'raw'
    ? (configModeChanged || configSavingChanged || configRawRebuildNeeded)
    : (configRawChanged || configFieldsChanged || configFieldsLoadingChanged || configSavingChanged || configModeChanged);
  if (configPanelShouldRebuild) {
    _state.lastConfigRaw = state.configRaw;
    _state.lastConfigSaving = state.configSaving;
    _state.lastConfigFieldsJson = JSON.stringify(state.configFields);
    _state.lastConfigFieldsLoading = state.configFieldsLoading;
    _state.lastConfigMode = state.configMode;
    rerenderSystemPanel(
      'systemConfigPanel',
      renderConfigEditor,
      () => { _state.configCodeMirror = destroyCodeMirror(_state.configCodeMirror); _state._configCmInitializing = false; },
      state.configMode === 'raw' ? initConfigCodeMirror : null,
      () => state.configMode === 'raw' ? getEditorValue(_state.configCodeMirror, null) : null,
      (val) => { if (val !== null && _state.configCodeMirror) setEditorValue(_state.configCodeMirror, val); }
    );
  } else if (configRawChanged && state.configMode === 'raw' && _state.configCodeMirror) {
    _state.lastConfigRaw = state.configRaw;
    setEditorValue(_state.configCodeMirror, state.configRaw);
  }

  const cookiesRawRebuildNeeded = cookiesRawChanged && _state.lastCookiesRaw === null && state.cookiesRaw !== null;
  const cookiesPanelShouldRebuild = state.cookiesMode === 'raw'
    ? (cookiesModeChanged || cookiesSavingChanged || cookiesRawRebuildNeeded)
    : (cookiesChanged || cookiesModeChanged || cookiesRawChanged || cookiesSavingChanged);
  if (cookiesPanelShouldRebuild) {
    _state.lastCookieItemsJson = JSON.stringify(state.cookieItems);
    _state.lastCookiesMode = state.cookiesMode;
    _state.lastCookiesRaw = state.cookiesRaw;
    _state.lastCookiesSaving = state.cookiesSaving;
    rerenderSystemPanel(
      'systemCookiesPanel',
      renderCookiesEditor,
      () => { _state.cookiesCodeMirror = destroyCodeMirror(_state.cookiesCodeMirror); _state._cookiesCmInitializing = false; },
      state.cookiesMode === 'raw' ? initCookiesCodeMirror : null,
      () => state.cookiesMode === 'raw' ? getEditorValue(_state.cookiesCodeMirror, null) : null,
      (val) => { if (val !== null && _state.cookiesCodeMirror) setEditorValue(_state.cookiesCodeMirror, val); }
    );
  } else if (cookiesRawChanged && state.cookiesMode === 'raw' && _state.cookiesCodeMirror) {
    _state.lastCookiesRaw = state.cookiesRaw;
    setEditorValue(_state.cookiesCodeMirror, state.cookiesRaw);
  }

  const schedulePanelSchedulesChanged = state._scheduleTab !== 'form' && schedulesChanged;
  if (schedulesChanged && !schedulePanelSchedulesChanged) {
    _state.lastSchedulesJson = JSON.stringify(state._schedules);
  }
  const scheduleRawRebuildNeeded = scheduleRawChanged && _state.lastScheduleRaw === null && state._scheduleRaw !== null;
  const schedulePanelShouldRebuild = state._scheduleTab === 'edit'
    ? (scheduleTabChanged || scheduleSavingChanged || scheduleExistsChanged || scheduleRawRebuildNeeded || schedulePanelSchedulesChanged || scheduleFormItemsChanged)
    : (schedulePanelSchedulesChanged || scheduleRawChanged || scheduleExistsChanged || scheduleSavingChanged || scheduleTabChanged || scheduleFormItemsChanged);
  if (schedulePanelShouldRebuild) {
    _state.lastSchedulesJson = JSON.stringify(state._schedules);
    _state.lastTasksJson = JSON.stringify(state.tasks);
    _state.lastScheduleRaw = state._scheduleRaw;
    _state.lastScheduleExists = state._scheduleExists;
    _state.lastScheduleSaving = state._scheduleSaving;
    _state.lastScheduleTab = state._scheduleTab;
    _state.lastScheduleFormItemsJson = JSON.stringify(state._scheduleFormItems);
    rerenderSystemPanel(
      'systemSchedulesPanel',
      renderScheduleViewer,
      () => { _state.scheduleCodeMirror = destroyCodeMirror(_state.scheduleCodeMirror); _state._scheduleCmInitializing = false; },
      state._scheduleTab === 'edit' ? initScheduleCodeMirror : null,
      () => state._scheduleTab === 'edit' ? getEditorValue(_state.scheduleCodeMirror, null) : null,
      (val) => { if (val !== null && _state.scheduleCodeMirror) setEditorValue(_state.scheduleCodeMirror, val); }
    );
  } else if (scheduleRawChanged && state._scheduleTab === 'edit' && _state.scheduleCodeMirror) {
    _state.lastScheduleRaw = state._scheduleRaw;
    setEditorValue(_state.scheduleCodeMirror, state._scheduleRaw);
  }
}

function syncSchedulesPage(state) {
  const schedulesChanged = JSON.stringify(state._schedules) !== _state.lastSchedulesJson;
  const scheduleExistsChanged = state._scheduleExists !== _state.lastScheduleExists;
  const schedulerRunningChanged = state._schedulerRunning !== _state.lastSchedulerRunning;

  if (schedulesChanged || scheduleExistsChanged || schedulerRunningChanged) {
    _state.lastSchedulesJson = JSON.stringify(state._schedules);
    _state.lastScheduleExists = state._scheduleExists;
    _state.lastSchedulerRunning = state._schedulerRunning;
    render();
  }
}

function syncLogsPage(state) {
  const logPagChanged = JSON.stringify(state.logPagination) !== _state.lastLogPaginationJson;
  const logsChanged = state.logs.length !== _state.lastLogsLength;
  const logLevelChanged = state.logLevel !== _state.lastLogLevel;
  const logNewArrivedChanged = state._logNewArrived !== _state.lastLogNewArrived;

  if (logNewArrivedChanged) {
    _state.lastLogNewArrived = state._logNewArrived;
    const btn = document.getElementById('logScrollToTopBtn');
    if (btn) btn.style.display = state._logNewArrived ? 'flex' : 'none';
  }

  if (logsChanged || logLevelChanged || logPagChanged) {
    _state.lastLogPaginationJson = JSON.stringify(state.logPagination);
    _state.lastLogsLength = state.logs.length;
    _state.lastLogLevel = state.logLevel;
    render();
  }
}

function syncOverviewPage(state) {
  const tasksJson = JSON.stringify(state.tasks);
  if (tasksJson !== _state.lastTasksJson) {
    _state.lastTasksJson = tasksJson;
    updateOverviewTasksUI(state.tasks);
  }
}

store.subscribe((state) => {
  if (state.currentPage !== _state.lastPage) {
    _state.lastPage = state.currentPage;
    render();
    return;
  }

  const tasksJson = JSON.stringify(state.tasks);
  const tasksChanged = tasksJson !== _state.lastTasksJson;

  if (tasksChanged) {
    _state.lastTasksJson = tasksJson;
    if (state.currentPage === 'tasks') { updateTaskListUI(state.tasks); }
  }

  if (state.currentPage === 'data') syncDataPage(state);
  else if (state.currentPage === 'system') syncSystemPage(state, tasksChanged);
  else if (state.currentPage === 'schedules') syncSchedulesPage(state);
  else if (state.currentPage === 'logs') syncLogsPage(state);
  else if (state.currentPage === 'overview') syncOverviewPage(state);
});

function getTaskStats(tasks) {
  const taskStats = { queued: 0, running: 0, completed: 0, failed: 0, cancelled: 0 };
  tasks.forEach(t => { if (taskStats[t.status] !== undefined) taskStats[t.status]++; });
  return taskStats;
}

function updateOverviewStatsUI(tasks) {
  const taskStats = getTaskStats(tasks);

  const runningStat = document.querySelector('[data-overview-stat="running"]');
  if (runningStat) runningStat.textContent = taskStats.running;

  const completedStat = document.querySelector('[data-overview-stat="completed"]');
  if (completedStat) completedStat.textContent = taskStats.completed;
}

// Update overview page recent tasks without full re-render
function updateOverviewTasksUI(tasks) {
  const recentTasks = tasks.slice(0, 5);
  const taskList = document.querySelector('.overview-tasks-list');
  if (!taskList) return;

  updateOverviewStatsUI(tasks);
  
  if (recentTasks.length === 0) {
    taskList.className = 'empty-state overview-tasks-list';
    taskList.innerHTML = `
      <div class="empty-icon">📋</div>
      <div class="empty-title">暂无任务</div>
      <div class="empty-desc">创建一个新任务开始下载 Twitter 媒体文件</div>
    `;
  } else {
    taskList.className = 'task-list overview-tasks-list';
    taskList.innerHTML = recentTasks.map(t => renderTaskItem(t)).join('');
  }
}

// Update only the task list part of the UI without full re-render
function updateTaskListUI(tasks) {
  const taskList = document.getElementById('taskListContainer');
  if (!taskList) return;
  
  const filter = store.state.taskFilter;
  const search = store.state.taskSearch.toLowerCase();
  
  let filtered = tasks;
  
  if (filter !== 'all') {
    filtered = filtered.filter(t => t.status === filter);
  }
  
  if (search) {
    filtered = filtered.filter(t => {
      const target = (t.data?.screen_name || t.data?.list_id || '').toString().toLowerCase();
      const batchTargets = [
        ...(t.data?.users || []),
        ...(t.data?.lists || []),
        ...(t.data?.following_names || [])
      ].join(' ').toLowerCase();
      return target.includes(search) || batchTargets.includes(search) || t.task_id.toLowerCase().includes(search);
    });
  }
  
  if (filtered.length === 0) {
    taskList.className = 'empty-state';
    taskList.innerHTML = `
      <div class="empty-icon">🔍</div>
      <div class="empty-title">没有找到匹配的任务</div>
      <div class="empty-desc">尝试调整筛选条件或搜索关键词</div>
    `;
  } else {
    taskList.className = 'task-list';
    taskList.innerHTML = filtered.map(t => renderTaskItem(t)).join('');
  }
  
  // Update task count subtitle
  const subtitle = document.querySelector('[data-task-count-subtitle]');
  if (subtitle) {
    subtitle.textContent = `共 ${tasks.length} 个任务`;
  }
}

// Register global event listeners once (event delegation on content container)
document.getElementById('contentContainer').addEventListener('keydown', (e) => {
  if (e.key !== 'Enter') return;
  const id = e.target.id;
  if (id === 'quickDownloadInput') handleQuickDownload();
  else if (id === 'dbSearchInput') searchDB();
  else if (id === 'logSearchInput') refreshLogs();
});

document.getElementById('contentContainer').addEventListener('click', (e) => {
  const tab = e.target.closest('[data-task-tab]');
  if (tab) {
    document.querySelectorAll('[data-task-tab]').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');
    document.getElementById('taskFormContainer').innerHTML = renderTaskForm(tab.dataset.taskTab);
    return;
  }

  const cancelBtn = e.target.closest('[data-action="cancel"]');
  if (cancelBtn) {
    const taskItem = cancelBtn.closest('[data-task-id]');
    if (taskItem) cancelTask(taskItem.dataset.taskId);
    return;
  }

  const detailBtn = e.target.closest('[data-action="detail"]');
  if (detailBtn) {
    const taskItem = detailBtn.closest('[data-task-id]');
    if (taskItem) showTaskDetail(taskItem.dataset.taskId);
    return;
  }

  const taskItem = e.target.closest('.task-item[data-task-id]');
  if (taskItem) {
    showTaskDetail(taskItem.dataset.taskId);
  }
});

// Start
init();
