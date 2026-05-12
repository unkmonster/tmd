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
  return document.getElementById(inputId).value
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
const store = {
  state: {
    currentPage: 'overview',
    health: null,
    tasks: [],
    users: [],
    lists: [],
    entities: [],
    config: null,
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
      userLinks: { data: [], total: 0, page: 1, pageSize: 200 }
    },
    dbPagination: {
      users: { page: 1, pageSize: 200, totalPages: 1 },
      lists: { page: 1, pageSize: 200, totalPages: 1 },
      entities: { page: 1, pageSize: 200, totalPages: 1 },
      listEntities: { page: 1, pageSize: 200, totalPages: 1 },
      userLinks: { page: 1, pageSize: 200, totalPages: 1 }
    },
    dbSort: {
      users: { sortBy: 'id', sortOrder: 'desc' },
      lists: { sortBy: 'id', sortOrder: 'desc' },
      entities: { sortBy: 'id', sortOrder: 'desc' },
      listEntities: { sortBy: 'id', sortOrder: 'desc' },
      userLinks: { sortBy: 'id', sortOrder: 'desc' }
    },
    dbSearch: {
      users: '',
      lists: '',
      entities: '',
      listEntities: '',
      userLinks: ''
    },
    configRaw: '',
    configExists: false,
    configSaving: false,
    configFieldsLoading: false,
    logs: [],
    logLevel: 'all',
    logSearch: '',
    logAutoRefresh: true,
    logPagination: { page: 1, pageSize: 100, total: 0, totalPages: 1 },
    _systemTab: 'config',
    configMode: 'form',
    configFields: [],
    cookiesRaw: '',
    cookiesExists: false,
    cookiesSaving: false,
    cookieItems: [],
    cookiesMode: 'form',
    _scheduleTab: 'form',
    _schedules: [],
    _scheduleRaw: '',
    _scheduleExists: false,
    _scheduleSaving: false,
    _scheduleFormItems: [],
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
    this.state = { ...this.state, ...newState };
    this.listeners.forEach(fn => fn(this.state));
  }
};

// ============================================
// API Client
// ============================================
const api = {
  base: '',
  _abortController: null,

  abortAll() {
    if (this._abortController) {
      this._abortController.abort();
      this._abortController = null;
    }
  },

  _getAbortSignal() {
    if (!this._abortController) this._abortController = new AbortController();
    return this._abortController.signal;
  },
  
  async request(method, path, body = null, extra = {}) {
    const options = {
      method,
      signal: this._getAbortSignal()
    };
    if (extra.isFormData) {
      if (body !== null && body !== undefined) options.body = body;
    } else {
      options.headers = { 'Content-Type': 'application/json' };
      if (body !== null && body !== undefined) options.body = JSON.stringify(body);
    }
    
    const res = await fetch(this.base + path, options);
    const data = await res.json().catch(() => ({ success: false, error: 'Invalid response' }));
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

  // Schedules
  getSchedules() { return this.get('/api/v1/schedules'); },
  replaceSchedules(entries) { return this.request('PUT', '/api/v1/schedules', { entries }); },
  setScheduleEnabled(id, enabled) { return this.request('PATCH', `/api/v1/schedules/${encodeURIComponent(id)}/enabled`, { enabled }); },
  getSchedulesRaw() { return this.get('/api/v1/schedules/raw'); },
  updateSchedulesRaw(content) { return this.request('PUT', '/api/v1/schedules/raw', { content }); },
  triggerSchedule(id) { return this.request('POST', `/api/v1/schedules/${encodeURIComponent(id)}/trigger`, {}); },
  validateSchedule(body) { return this.post('/api/v1/schedules/validate', body); },

  // Database CRUD with pagination
  getDBUsers(params = '') { return this.get(`/api/v1/db/users${params ? '?' + params : ''}`); },
  getDBUser(id) { return this.get(`/api/v1/db/users/${id}`); },
  updateDBUser(id, data) { return this.request('PUT', `/api/v1/db/users/${id}`, data); },
  deleteDBUser(id) { return this.request('DELETE', `/api/v1/db/users/${id}`); },
  getDBUserPreviousNames(id, params = '') { return this.get(`/api/v1/db/users/${id}/previous-names${params ? '?' + params : ''}`); },
  
  getDBLists(params = '') { return this.get(`/api/v1/db/lists${params ? '?' + params : ''}`); },
  getDBList(id) { return this.get(`/api/v1/db/lists/${id}`); },
  updateDBList(id, data) { return this.request('PUT', `/api/v1/db/lists/${id}`, data); },
  deleteDBList(id) { return this.request('DELETE', `/api/v1/db/lists/${id}`); },
  
  getDBUserEntities(params = '') { return this.get(`/api/v1/db/user-entities${params ? '?' + params : ''}`); },
  getDBUserEntity(id) { return this.get(`/api/v1/db/user-entities/${id}`); },
  updateDBUserEntity(id, data) { return this.request('PUT', `/api/v1/db/user-entities/${id}`, data); },
  deleteDBUserEntity(id) { return this.request('DELETE', `/api/v1/db/user-entities/${id}`); },
  
  getDBListEntities(params = '') { return this.get(`/api/v1/db/list-entities${params ? '?' + params : ''}`); },
  getDBListEntity(id) { return this.get(`/api/v1/db/list-entities/${id}`); },
  updateDBListEntity(id, data) { return this.request('PUT', `/api/v1/db/list-entities/${id}`, data); },
  deleteDBListEntity(id) { return this.request('DELETE', `/api/v1/db/list-entities/${id}`); },
  
  getDBUserLinks(params = '') { return this.get(`/api/v1/db/user-links${params ? '?' + params : ''}`); }
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
      };
      if (!isScheduleFormEditing()) {
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
        const type = notif.type === 'task_completed' ? 'success' :
                     notif.type === 'task_failed' ? 'error' :
                     notif.type === 'task_cancelled' ? 'warning' :
                     notif.type === 'schedule_warning' ? 'warning' : 'success';
        toast.show(notif.message, type);
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
          handleServerShutdown('服务器连接丢失');
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
    el.title = connected ? '实时连接正常' : '实时连接已断开';
  },

  refreshCurrentPage() {
    const page = store.state.currentPage;
    if (page === 'schedules') {
      loadSchedules();
      return;
    }
    if (page !== 'system') return;

    if (store.state._systemTab === 'schedules') {
      loadSchedules({ updateFormItems: !isScheduleFormEditing() });
    } else if (store.state._systemTab === 'config') {
      refreshConfigAfterReconnect();
    } else if (store.state._systemTab === 'cookies') {
      refreshCookiesAfterReconnect();
    } else if (store.state._systemTab === 'logs') {
      loadLogs();
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
    const { health, tasks, config } = store.state;
    
    const taskStats = { queued: 0, running: 0, completed: 0, failed: 0, cancelled: 0 };
    tasks.forEach(t => { if (taskStats[t.status] !== undefined) taskStats[t.status]++; });
    
    const recentTasks = tasks.slice(0, 5);
    
    return `
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon" style="color: var(--success);">●</div>
          <div class="stat-content">
            <div class="stat-value">${health ? (health.status === 'ok' ? '健康' : '异常') : '检查中'}</div>
            <div class="stat-label">系统状态 ${health ? 'v' + health.version : ''}</div>
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
      
      <div class="card" style="margin-bottom: var(--space-6);">
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
      
      <div class="card">
        <div class="card-header">
          <div class="card-title">最近任务</div>
          <button class="btn btn-ghost btn-sm" onclick="navigateTo('tasks')">查看全部 →</button>
        </div>
        <div class="card-body" style="padding: 0;">
          ${recentTasks.length === 0 ? `
            <div class="empty-state overview-tasks-list">
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
    `;
  },
  
  // Tasks Page
  tasks() {
    const { tasks } = store.state;
    
    return `
      <div class="tasks-layout">
        <div>
          <div class="card" style="position: sticky; top: calc(var(--header-height) + var(--space-6));">
            <div class="card-header">
              <div class="card-title">创建新任务</div>
            </div>
            <div class="card-body">
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
          <div class="card">
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
                <button class="btn btn-ghost btn-sm" onclick="refreshTasks()">🔄 刷新</button>
              </div>
            </div>
            <div class="card-body" style="padding: 0;">
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
      userLinks: { title: 'User Links', data: dbData.userLinks?.data || [], count: dbData.userLinks?.total || 0 }
    };
    
    const current = dataMap[dataSubPage];
    const pagination = dbPagination[dataSubPage] || { page: 1, pageSize: 200, totalPages: 1 };
    const sort = dbSort[dataSubPage] || { sortBy: 'id', sortOrder: 'desc' };
    
    return `
      <div class="card">
        <div class="card-header">
          <div>
            <div class="tabs" style="margin: 0; border: none;">
              <div class="tab ${dataSubPage === 'users' ? 'active' : ''}" onclick="setDataSubPage('users')">Users</div>
              <div class="tab ${dataSubPage === 'lists' ? 'active' : ''}" onclick="setDataSubPage('lists')">Lists</div>
              <div class="tab ${dataSubPage === 'entities' ? 'active' : ''}" onclick="setDataSubPage('entities')">User Entities</div>
              <div class="tab ${dataSubPage === 'listEntities' ? 'active' : ''}" onclick="setDataSubPage('listEntities')">List Entities</div>
              <div class="tab ${dataSubPage === 'userLinks' ? 'active' : ''}" onclick="setDataSubPage('userLinks')">User Links</div>
            </div>
          </div>
          <div class="flex gap-2 items-center">
            <input type="text" class="form-input search-input" id="dbSearchInput" 
              placeholder="搜索..." oninput="updateSearchState('dbSearch',store.state.dataSubPage,this.value)" onkeypress="if(event.key==='Enter')searchDB()">
            <button class="btn btn-ghost btn-icon" onclick="searchDB()">🔍</button>
            <button class="btn btn-ghost btn-icon" onclick="refreshDBData()">🔄</button>
          </div>
        </div>
        
        <div class="card-body" style="padding: 0;">
          <div class="table-scroll-container">
            ${renderDBTable(dataSubPage, current.data, sort)}
          </div>
          ${renderDBMobileCards(dataSubPage, current.data)}
        </div>
        
        <div class="pagination">
          <div class="pagination-info">
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

    const schedulerBanner = !_schedulerRunning
      ? `<div class="alert alert-warning" style="margin-bottom:var(--space-3)">⚠️ 调度器未启动，定时任务不会自动执行。请在「定时任务」页面中添加并启用规则后重载配置。</div>`
      : '';

    return schedulerBanner + renderScheduleTable(_schedules, _scheduleExists);
  },

  // System Page
  system() {
    const { config, configRaw, logs, logLevel, logSearch, logPagination } = store.state;

    return `
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-4)">
        <div class="system-tabs" style="margin:0">
          <div class="tab ${store.state._systemTab === 'config' ? 'active' : ''}" onclick="setSystemTab('config')">⚙️ 配置编辑</div>
          <div class="tab ${store.state._systemTab === 'cookies' ? 'active' : ''}" onclick="setSystemTab('cookies')">🍪 额外账户</div>
          <div class="tab ${store.state._systemTab === 'schedules' ? 'active' : ''}" onclick="setSystemTab('schedules')">⏰ 任务配置</div>
          <div class="tab ${store.state._systemTab === 'logs' ? 'active' : ''}" onclick="setSystemTab('logs')">📋 系统日志</div>
        </div>
        <button class="btn btn-danger btn-sm" onclick="shutdownServer()">⏻ 关闭服务器</button>
      </div>

      <div id="systemConfigPanel" class="system-panel" style="${store.state._systemTab === 'config' ? '' : 'display:none'}">
        ${renderConfigEditor()}
      </div>

      <div id="systemCookiesPanel" class="system-panel" style="${store.state._systemTab === 'cookies' ? '' : 'display:none'}">
        ${renderCookiesEditor()}
      </div>

      <div id="systemSchedulesPanel" class="system-panel" style="${store.state._systemTab === 'schedules' ? '' : 'display:none'}">
        ${renderScheduleViewer()}
      </div>

      <div id="systemLogsPanel" class="system-panel" style="${store.state._systemTab === 'logs' ? '' : 'display:none'}">
        ${renderLogViewer()}
      </div>
    `;
  }
};

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

function renderTaskResult(task) {
  const result = task.result;
  if (!result) return '';

  const sections = [];
  if (result.main) {
    sections.push(`
      <div class="text-sm">
        <strong>主下载</strong>
        <span class="text-secondary">下载: ${escapeHtml(result.main.downloaded || 0)} · 失败: ${escapeHtml(result.main.failed || 0)}</span>
      </div>
    `);
  }
  if (result.profile) {
    sections.push(`
      <div class="text-sm">
        <strong>Profile</strong>
        <span class="text-secondary">下载: ${escapeHtml(result.profile.downloaded || 0)} · 失败: ${escapeHtml(result.profile.failed || 0)} · Versionedfile: ${escapeHtml(result.profile.versioned || 0)}</span>
      </div>
    `);
  }
  if (result.message) {
    sections.push(`<div class="text-sm text-secondary">${escapeHtml(result.message)}</div>`);
  }
  if (sections.length === 0) return '';

  return `
    <div class="form-group">
      <label class="form-label">结果</label>
      <div style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md); display: flex; flex-direction: column; gap: var(--space-2);">
        ${sections.join('')}
      </div>
    </div>
  `;
}

function getOptionalTimestamp(inputId) {
  const input = document.getElementById(inputId);
  const value = input?.value.trim() || '';
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
    <div class="task-item" data-task-id="${escapeAttr(task.task_id)}" onclick="showTaskDetail(this.dataset.taskId)">
      <div class="task-info">
        <div class="task-title">${escapeHtml(task.type)} - ${target}</div>
        <div class="task-meta">
          <span class="tag ${status.tag}">${status.text}</span>
          <span>ID: ${escapeHtml(task.task_id)}</span>
          <span>${new Date(task.created_at).toLocaleString()}</span>
        </div>
      </div>
      <div class="task-progress">
        <div class="progress-bar">
          <div class="progress-fill" style="width: ${pct}%"></div>
        </div>
        <div class="task-progress-text">${pct}%${stageText}${currentText}</div>
      </div>
      <div class="task-actions" onclick="event.stopPropagation()">
        ${task.status === 'running' || task.status === 'queued' ?
          `<button class="btn btn-danger btn-sm" data-task-id="${escapeAttr(task.task_id)}" onclick="cancelTask(this.dataset.taskId)">取消</button>` :
          `<button class="btn btn-ghost btn-sm" data-task-id="${escapeAttr(task.task_id)}" onclick="showTaskDetail(this.dataset.taskId)">详情</button>`
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
        <input type="number" class="form-input" id="listId" placeholder="例如: 123456789">
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
        <input type="datetime-local" class="form-input" id="markTimestamp">
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

  const sortIcon = (field) => {
    if (sort.sortBy !== field) return '<span class="sort-icon">↕</span>';
    return sort.sortOrder === 'asc'
      ? '<span class="sort-icon sort-active">↑</span>'
      : '<span class="sort-icon sort-active">↓</span>';
  };

  const sortableHeader = (field, label) => `
    <th data-sort-field="${escapeAttr(field)}" class="${sort.sortBy === field ? 'sort-active' : ''}" onclick="sortDB(this.dataset.sortField)">
      ${label} ${sortIcon(field)}
    </th>
  `;

  const renderActionButtons = (type, item) => {
    if (type === 'userLinks') {
      return '<span style="color: var(--text-tertiary);">-</span>';
    }
    const idStr = String(item.id);
    return `
      <div class="flex gap-2">
        <button class="btn btn-ghost btn-sm" data-db-type="${escapeAttr(type)}" data-db-id="${escapeAttr(idStr)}" onclick="editDBItem(this.dataset.dbType, this.dataset.dbId)">✏️</button>
        <button class="btn btn-danger btn-sm" data-db-type="${escapeAttr(type)}" data-db-id="${escapeAttr(idStr)}" onclick="deleteDBItem(this.dataset.dbType, this.dataset.dbId)">🗑️</button>
      </div>
    `;
  };

  const rows = data.map(item => {
    if (type === 'users') {
      return `<tr>
        <td>${escapeHtml(item.id)}</td>
        <td>@${escapeHtml(item.screen_name)}</td>
        <td>${escapeHtml(item.name)}</td>
        <td>${item.protected ? '🔒' : '🔓'}</td>
        <td>${item.is_accessible ? '✅' : '❌'}</td>
        <td>${escapeHtml(item.friends_count)}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else if (type === 'lists') {
      return `<tr>
        <td>${escapeHtml(item.id)}</td>
        <td>${escapeHtml(item.name)}</td>
        <td>${escapeHtml(item.owner_user_id)}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else if (type === 'entities') {
      return `<tr>
        <td>${escapeHtml(item.id)}</td>
        <td>${escapeHtml(item.user_id)}</td>
        <td>${escapeHtml(item.name)}</td>
        <td>${escapeHtml(item.latest_release_time || '-')}</td>
        <td>${escapeHtml(item.media_count || '-')}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else if (type === 'listEntities') {
      return `<tr>
        <td>${escapeHtml(item.id)}</td>
        <td>${escapeHtml(item.lst_id)}</td>
        <td>${escapeHtml(item.name)}</td>
        <td>${escapeHtml(item.parent_dir)}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else {
      return `<tr>
        <td>${escapeHtml(item.id)}</td>
        <td>${escapeHtml(item.user_id)}</td>
        <td>${escapeHtml(item.name)}</td>
        <td>${escapeHtml(item.parent_lst_entity_id)}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    }
  }).join('');

  return `
    <table class="data-table">
      <thead>
        <tr>
          ${type === 'users' ? `
            ${sortableHeader('id', 'ID')}
            ${sortableHeader('screen_name', 'Screen Name')}
            ${sortableHeader('name', 'Name')}
            <th>Protected</th>
            <th>Accessible</th>
            ${sortableHeader('friends_count', 'Friends')}
            <th>Actions</th>
          ` : type === 'lists' ? `
            ${sortableHeader('id', 'ID')}
            ${sortableHeader('name', 'Name')}
            ${sortableHeader('owner_id', 'Owner ID')}
            <th>Actions</th>
          ` : type === 'entities' ? `
            ${sortableHeader('id', 'ID')}
            ${sortableHeader('user_id', 'User ID')}
            ${sortableHeader('name', 'Name')}
            ${sortableHeader('latest_release_time', 'Latest Release')}
            ${sortableHeader('media_count', 'Media Count')}
            <th>Actions</th>
          ` : type === 'listEntities' ? `
            ${sortableHeader('id', 'ID')}
            ${sortableHeader('lst_id', 'List ID')}
            ${sortableHeader('name', 'Name')}
            <th>Parent Dir</th>
            <th>Actions</th>
          ` : `
            ${sortableHeader('id', 'ID')}
            ${sortableHeader('user_id', 'User ID')}
            ${sortableHeader('name', 'Name')}
            <th>Parent Entity</th>
            <th>Actions</th>
          `}
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderDBMobileCards(type, data) {
  if (!data || data.length === 0) return '';

  const renderActionButtons = (type, item) => {
    if (type === 'userLinks') return '';
    const idStr = String(item.id);
    return `
      <div class="flex gap-2">
        <button class="btn btn-ghost btn-sm" data-db-type="${escapeAttr(type)}" data-db-id="${escapeAttr(idStr)}" onclick="editDBItem(this.dataset.dbType, this.dataset.dbId)">✏️</button>
        <button class="btn btn-danger btn-sm" data-db-type="${escapeAttr(type)}" data-db-id="${escapeAttr(idStr)}" onclick="deleteDBItem(this.dataset.dbType, this.dataset.dbId)">🗑️</button>
      </div>
    `;
  };

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
      toast.show('数据已刷新');
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
  store.setState({
    dbPagination: {
      ...dbPagination,
      [dataSubPage]: { ...dbPagination[dataSubPage], page }
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
    }
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
            <input type="number" class="form-input" id="editFriendsCount" value="${escapeAttr(item.friends_count || 0)}">
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
      data.friends_count = parseInt(document.getElementById('editFriendsCount').value) || 0;
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
      data.media_count = parseInt(document.getElementById('editEntityMediaCount').value) || 0;
      if (!data.name) return toast.show('Name is required', 'error');
      break;
    case 'listEntities':
      data.name = document.getElementById('editListEntityName').value.trim();
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
      default:
        throw new Error('Unknown type: ' + type);
    }
    toast.show('删除成功');
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
    input.value = value;
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
    if (!['i', 'search', 'status', 'home', 'explore', 'notifications', 'messages', 'settings', 'compose'].includes(pathPart.toLowerCase())) {
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
    const requests = [
      ...users.map(screenName => () => api.createUserMark(screenName, timestamp)),
      ...listIDs.map(listID => () => api.createListMark(listID, timestamp)),
      ...followingNames.map(screenName => () => api.createFollowingMark(screenName, timestamp))
    ];

    const results = await Promise.allSettled(requests.map(run => run()));
    const successCount = results.filter(result => result.status === 'fulfilled').length;
    const failedResults = results.filter(result => result.status === 'rejected');

    if (successCount === 0) {
      throw failedResults[0]?.reason || new Error('创建标记任务失败');
    }

    document.getElementById('markUsers').value = '';
    document.getElementById('markLists').value = '';
    document.getElementById('markFollowingNames').value = '';
    document.getElementById('markTimestamp').value = '';

    if (failedResults.length > 0) {
      toast.show(`已创建 ${successCount} 个标记任务，${failedResults.length} 个失败`, 'warning');
      return;
    }

    toast.show(`已创建 ${successCount} 个标记任务`);
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

function showTaskDetail(id) {
  const task = store.state.tasks.find(t => t.task_id === id);
  if (!task) return;
  
  const statusMap = {
    queued: '排队中',
    running: '运行中',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消'
  };
  
  const pct = getTaskProgressPercent(task);

  const stageText = task.progress?.stage ? escapeHtml(getStageText(task.progress.stage)) : '';
  const currentText = task.progress?.current ? ` · ${escapeHtml(task.progress.current)}` : '';

  const content = `
    <div class="form-group">
      <label class="form-label">任务 ID</label>
      <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${escapeHtml(task.task_id)}</div>
    </div>
    <div class="form-group">
      <label class="form-label">类型</label>
      <div>${escapeHtml(task.type)}</div>
    </div>
    <div class="form-group">
      <label class="form-label">目标</label>
      <div>${escapeHtml(getTaskTarget(task))}</div>
      ${task.data?.users?.length ? `<div class="text-sm text-secondary" style="margin-top:4px;">用户: ${task.data.users.map(u => '@' + escapeHtml(u)).join(', ')}</div>` : ''}
      ${task.data?.lists?.length ? `<div class="text-sm text-secondary" style="margin-top:4px;">列表: ${task.data.lists.map(l => escapeHtml(String(l))).join(', ')}</div>` : ''}
      ${task.data?.following_names?.length ? `<div class="text-sm text-secondary" style="margin-top:4px;">关注: ${task.data.following_names.map(f => '@' + escapeHtml(f)).join(', ')}</div>` : ''}
    </div>
    <div class="form-group">
      <label class="form-label">状态</label>
      <div class="tag tag-${task.status}">${statusMap[task.status] || task.status}</div>
    </div>
    <div class="form-group">
      <label class="form-label">进度</label>
      <div class="progress-bar" style="margin-bottom: var(--space-2);">
        <div class="progress-fill" style="width: ${pct}%"></div>
      </div>
      <div class="text-sm text-secondary">${task.progress?.completed || 0} / ${task.progress?.total || 0} (${pct}%)${stageText}${currentText}</div>
      ${task.progress?.failed ? `<div class="text-sm" style="color: var(--danger); margin-top: 4px;">Failedtweet: ${escapeHtml(task.progress.failed)}</div>` : ''}
    </div>
    <div class="form-group">
      <label class="form-label">创建时间</label>
      <div>${new Date(task.created_at).toLocaleString()}</div>
    </div>
    ${renderTaskResult(task)}
    ${task.error ? `
      <div class="form-group">
        <label class="form-label" style="color: var(--danger);">错误信息</label>
        <div style="color: var(--danger); background: var(--danger-bg); padding: var(--space-3); border-radius: var(--radius-md);">${escapeHtml(task.error)}</div>
      </div>
    ` : ''}
  `;
  
  const footer = task.status === 'running' || task.status === 'queued' ?
    `<button class="btn btn-danger" data-task-id="${escapeAttr(task.task_id)}" onclick="cancelTask(this.dataset.taskId); drawer.close();">取消任务</button>` :
    '<button class="btn btn-secondary" onclick="drawer.close()">关闭</button>';
  
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
  return String(str).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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

  if (configMode === 'raw') return modeTabs + renderConfigRawEditor(configRaw, configSaving, configExists);
  return modeTabs + renderConfigForm(configFields, configSaving, configExists, configFieldsLoading);
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
    return `
      <div class="config-field">
        <label class="config-label">${escapeHtml(f.label)}</label>
        ${f.type === 'password' ? `<div class="config-mask-hint">当前值: ${escapeHtml(f.value)}</div>` : ''}
        <input type="${inputType}" class="form-input config-input" id="cf_${escapeAttr(f.name)}"
          name="${escapeAttr(f.name)}" value="${escapeAttr(f.type === 'password' ? '' : f.value)}"
          placeholder="${escapeAttr(f.placeholder || f.prompt)}"
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
      <div class="card-body" style="padding:0;">
        <div id="configEditorContainer"></div>
        <div class="config-hint text-sm text-tertiary p-3 mt-3">
          ⚠️ 直接编辑 YAML 需要了解语法格式。建议使用简易模式。
        </div>
      </div>
    </div>
  `;
}

function renderCookiesEditor() {
  const { cookiesMode, cookieItems, cookiesSaving, cookiesExists, cookiesRaw } = store.state;

  const modeTabs = `
    <div class="config-mode-tabs">
      <button class="mode-tab ${cookiesMode === 'form' ? 'active' : ''}" onclick="setCookiesMode('form')">📝 简易模式</button>
      <button class="mode-tab ${cookiesMode === 'raw' ? 'active' : ''}" onclick="setCookiesMode('raw')">🔧 高级 (YAML)</button>
    </div>
  `;

  if (cookiesMode === 'raw') return modeTabs + renderCookiesRawEditor(cookiesRaw, cookiesSaving, cookiesExists);
  return modeTabs + renderCookiesForm(cookieItems, cookiesSaving, cookiesExists);
}

function renderCookiesForm(items, saving, exists) {
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
            <div class="empty-desc">点击「添加账户」添加额外的 Twitter 账户 Cookie</div>
          </div>
        </div>
      </div>
    `;
  }

  const renderItem = (item, idx) => `
    <div class="config-group">
      <div class="config-group-title" style="display:flex;justify-content:space-between;align-items:center;">
        <span>🏷️ 账户 #${idx + 1}</span>
        <button class="btn btn-danger btn-sm" onclick="removeCookieAccount(${idx})">删除</button>
      </div>
      <div class="config-field">
        <label class="config-label">Auth Token</label>
        ${item.auth_token ? `<div class="config-mask-hint">当前值: ${escapeHtml(item.auth_token)}</div>` : ''}
        <input type="password" class="form-input config-input cookie-input" id="cookie_auth_${idx}"
          name="auth_token_${idx}" value="" placeholder="输入新的 Auth Token 或留空保留原值">
      </div>
      <div class="config-field">
        <label class="config-label">CT0</label>
        ${item.ct0 ? `<div class="config-mask-hint">当前值: ${escapeHtml(item.ct0)}</div>` : ''}
        <input type="password" class="form-input config-input cookie-input" id="cookie_ct0_${idx}"
          name="ct0_${idx}" value="" placeholder="输入新的 CT0 或留空保留原值">
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
        ${items.map(renderItem).join('')}
      </div>
    </div>
  `;
}

function renderCookiesRawEditor(raw, saving, exists) {
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
      <div class="card-body" style="padding:0;">
        <div id="cookiesEditorContainer"></div>
        <div class="config-hint text-sm text-tertiary p-3 mt-3">
          ⚠️ 直接编辑 YAML 需要了解语法格式。建议使用简易模式。
        </div>
      </div>
    </div>
  `;
}

function renderLogViewer() {
  const { logs, logLevel, logSearch, logPagination, logAutoRefresh } = store.state;

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
  function renderLine(line) {
    return `<div class="log-line" style="color:${getLineColor(line)}">${escapeHtml(stripAnsi(line))}</div>`;
  }

  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">系统日志</div><div class="card-subtitle">共 ${logPagination.total} 条记录</div></div>
        <div class="flex gap-2 items-center flex-wrap">
          <input type="text" class="form-input search-input" id="logSearchInput"
            placeholder="🔍 搜索..."
            oninput="updateSearchState('logSearch',null,this.value)"
            onkeypress="if(event.key==='Enter')refreshLogs()">
          <div class="log-level-filters">
            ${['all','debug','info','warn','error'].map(l => `<button class="btn btn-sm ${logLevel===l?'btn-primary':'btn-ghost'}" onclick="setLogLevel('${l}')">${l.toUpperCase()}</button>`).join('')}
          </div>
          <button class="btn btn-ghost btn-sm ${logAutoRefresh?'active':''}" onclick="toggleLogAutoRefresh()">${logAutoRefresh?'⏸️':'▶️'} 实时</button>
          <button class="btn btn-ghost btn-sm" onclick="refreshLogs()">🔄 刷新</button>
        </div>
      </div>
      <div class="card-body" style="padding:0;">
        <div class="log-container" id="logContainer">
          ${logs.length === 0 ? `<div class="empty-state"><div class="empty-icon">📋</div><div class="empty-title">暂无日志</div><div class="empty-desc">选择日志级别或调整筛选条件</div></div>` : logs.map(renderLine).join('')}
        </div>
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
  return store.state.configMode === 'raw' && configCodeMirror && getEditorValue(configCodeMirror, store.state.configRaw) !== (store.state.configRaw || '');
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

  if (_scheduleTab === 'edit') return schedulerBanner + modeTabs + renderScheduleRawEditor(_scheduleRaw, _scheduleSaving, _scheduleExists);
  return schedulerBanner + modeTabs + renderScheduleForm(_scheduleFormItems, _scheduleSaving, _scheduleExists);
}

function renderScheduleForm(items, saving, exists) {
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

  const typeOptions = (selected) => ['list', 'user', 'following', 'mixed'].map(t =>
    `<option value="${t}" ${t === selected ? 'selected' : ''}>${t === 'list' ? '📋 列表' : t === 'user' ? '👤 用户' : t === 'following' ? '👥 关注' : '🔀 混合'}</option>`
  ).join('');

  const scheduleModeOptions = (selected) => ['interval', 'daily'].map(m =>
    `<option value="${m}" ${m === selected ? 'selected' : ''}>${m === 'interval' ? '⏱️ 间隔执行' : '🕐 每日定时'}</option>`
  ).join('');

  const renderItem = (item, idx) => {
    const typeLabel = item.type === 'list' ? '📋 列表' : item.type === 'user' ? '👤 用户' : item.type === 'following' ? '👥 关注' : '🔀 混合';
    return `
    <div class="config-group">
      <div class="config-group-title" style="display:flex;justify-content:space-between;align-items:center;">
        <span>${typeLabel} #${idx + 1}${item.name ? ' · ' + escapeHtml(item.name) : ''}</span>
        <div class="flex gap-2">
          <label style="display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-secondary);cursor:pointer;">
            <input type="checkbox" id="sf_enabled_${idx}" ${item.enabled ? 'checked' : ''} style="margin:0">
            启用
          </label>
          <button class="btn btn-danger btn-sm" onclick="removeScheduleItem(${idx})">删除</button>
        </div>
      </div>
      <div class="config-field">
        <label class="config-label">类型</label>
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
        <label style="display:flex;align-items:center;gap:4px;font-size:13px;cursor:pointer;">
          <input type="checkbox" id="sf_auto_follow_${idx}" ${item.auto_follow ? 'checked' : ''} style="margin:0">
          自动申请受保护账号
        </label>
        <label style="display:flex;align-items:center;gap:4px;font-size:13px;cursor:pointer;">
          <input type="checkbox" id="sf_follow_members_${idx}" ${item.follow_members ? 'checked' : ''} style="margin:0">
          下载时关注目标/成员
        </label>
        <label style="display:flex;align-items:center;gap:4px;font-size:13px;cursor:pointer;">
          <input type="checkbox" id="sf_skip_profile_${idx}" ${item.skip_profile ? 'checked' : ''} style="margin:0">
          跳过 Profile
        </label>
        <label style="display:flex;align-items:center;gap:4px;font-size:13px;cursor:pointer;">
          <input type="checkbox" id="sf_no_retry_${idx}" ${item.no_retry ? 'checked' : ''} style="margin:0">
          不重试
        </label>
        <label style="display:flex;align-items:center;gap:4px;font-size:13px;cursor:pointer;">
          <input type="checkbox" id="sf_run_on_start_${idx}" ${item.run_on_start ? 'checked' : ''} style="margin:0">
          首次启动时立即运行
        </label>
      </div>
      <div id="sf_schedule_hint_${idx}" class="config-hint" aria-live="polite" style="font-size:12px;margin-top:8px;min-height:0"></div>
    </div>
  `;
  };

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
        ${items.map(renderItem).join('')}
      </div>
    </div>
  `;
}

function renderScheduleTable(schedules, exists) {
  const active = schedules.filter(s => readScheduleEntryField(s.entry, 'enabled', 'Enabled')).length;
  const total = schedules.length;

  if (schedules.length === 0) {
    return `
      <div class="card">
        <div class="card-header">
          <div><div class="card-title">定时下载任务</div><div class="card-subtitle">${exists ? '✅ 文件存在 · 0 条规则' : '⚠️ 配置文件不存在'}</div></div>
          <div class="flex gap-2">
            <button class="btn btn-ghost btn-sm" onclick="loadSchedules()">🔄 刷新</button>
          </div>
        </div>
        <div class="card-body">
          <div class="empty-state">
            <div class="empty-icon">⏰</div>
            <div class="empty-title">暂无定时任务</div>
            <div class="empty-desc">在「定时任务」页面中添加定时下载规则</div>
          </div>
        </div>
      </div>
    `;
  }

  const typeTag = (type) => {
    const map = { list: ['List', 'tag-info'], user: ['User', 'tag-success'], following: ['Following', 'tag-warning'], mixed: ['Mixed', 'tag-primary'] };
    const [label, cls] = map[type] || [escapeHtml(type), ''];
    return `<span class="tag ${escapeHtml(cls)}">${escapeHtml(label)}</span>`;
  };

  const failureTag = (count) => {
    if (!count || count === 0) return '';
    if (count >= 3) return `<span class="tag tag-danger">⚠ ${count}次失败</span>`;
    return `<span class="tag tag-warning">${count}次失败</span>`;
  };

  const fmtTime = (t) => t ? new Date(t).toLocaleString() : '-';

  const renderScheduleItem = (s) => {
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
          <button class="btn btn-ghost btn-sm" data-schedule-id="${escapeAttr(entry.id)}" onclick="triggerSchedule(this.dataset.scheduleId)" ${!entry.enabled ? 'disabled title="规则已禁用"' : ''}>▶️</button>
        </div>
      </div>
    `;
  };

  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">定时下载任务</div><div class="card-subtitle">共 ${total} 条规则 · ${active} 个启用</div></div>
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-sm" onclick="navigateToSystemSchedules()">📝 编辑任务</button>
          <button class="btn btn-ghost btn-sm" onclick="loadSchedules()">🔄 刷新</button>
        </div>
      </div>
      <div class="card-body" style="padding:0">
        ${schedules.map(renderScheduleItem).join('')}
      </div>
    </div>
  `;
}

function renderScheduleRawEditor(raw, saving, exists) {
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
      <div class="card-body" style="padding:0;">
        <div id="scheduleEditorContainer"></div>
        <div class="config-hint text-sm text-tertiary p-3 mt-3">
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
    }
    store.setState(update);
  } catch (e) { /* ignore */ }
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
  } catch (e) { /* ignore */ }
}

async function saveScheduleRaw() {
  const content = getEditorValue(scheduleCodeMirror, store.state._scheduleRaw);
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
    const rawData = await api.getSchedulesRaw();
    store.setState({
      _scheduleRaw: rawData.content || '',
      _scheduleExists: rawData.exists || false,
      _scheduleSaving: false,
    });
    setEditorValue(scheduleCodeMirror, store.state._scheduleRaw || '');
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

async function toggleScheduleEnabled(id, currentEnabled) {
  try {
    await api.setScheduleEnabled(id, !currentEnabled);
    toast.show(currentEnabled ? '已禁用定时任务' : '已启用定时任务');
  } catch (e) {
    toast.show('操作失败: ' + e.message, 'error');
  }
}

function navigateToSystemSchedules() {
  if (lastPage === 'system') {
    store.setState({ _systemTab: 'schedules' });
  } else {
    store.setState({ currentPage: 'system', _systemTab: 'schedules' });
    updateURL('system');
    document.querySelectorAll('.nav-item').forEach(el => {
      el.classList.toggle('active', el.dataset.page === 'system');
    });
    document.querySelectorAll('.mobile-nav-item').forEach(el => {
      el.classList.toggle('active', el.dataset.page === 'system');
    });
    document.getElementById('pageTitle').textContent = '系统';
    if (store.state.isMobile) {
      document.getElementById('sidebar').classList.remove('open');
    }
  }
}

function setScheduleTab(tab) {
  if (tab !== 'edit' && scheduleCodeMirror) {
    scheduleCodeMirror = destroyCodeMirror(scheduleCodeMirror);
    _scheduleCmInitializing = false;
  }
  store.setState({ _scheduleTab: tab });
  if (tab === 'edit' && !store.state._scheduleRaw) loadScheduleRaw();
  if (tab === 'form' && store.state._scheduleFormItems.length === 0 && store.state._schedules.length === 0) loadSchedules();
}

function addScheduleItem() {
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
  }, ...readScheduleFormItemsFromDOM()];
  store.setState({ _scheduleFormItems: items });
  glowNewFirstItem('systemSchedulesPanel');
}

function clearAllScheduleValidationTimers() {
  Object.keys(_scheduleValidateTimers).forEach(k => {
    clearTimeout(_scheduleValidateTimers[k]);
    delete _scheduleValidateTimers[k];
  });
  Object.keys(_scheduleValidateRequests).forEach(k => {
    delete _scheduleValidateRequests[k];
  });
}

function removeScheduleItem(index) {
  clearAllScheduleValidationTimers();
  const items = readScheduleFormItemsFromDOM().filter((_, i) => i !== index);
  store.setState({ _scheduleFormItems: items });
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
  clearTimeout(_scheduleValidateTimers[index]);
  delete _scheduleValidateTimers[index];
  delete _scheduleValidateRequests[index];
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
  store.setState({ _scheduleFormItems: items });
}

const _scheduleValidateTimers = {};
const _scheduleValidateRequests = {};
let _scheduleValidateRequestSeq = 0;

function scheduleFieldChanged(idx) {
  clearTimeout(_scheduleValidateTimers[idx]);
  _scheduleValidateTimers[idx] = setTimeout(() => validateScheduleField(idx), 600);
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
    return;
  }

  const entry = { type, schedule: `${mode}:${scheduleValue}` };

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

  const requestSeq = ++_scheduleValidateRequestSeq;
  _scheduleValidateRequests[idx] = requestSeq;
  try {
    const result = await api.validateSchedule({ entries: [entry] });
    if (_scheduleValidateRequests[idx] !== requestSeq) return;
    if (result.valid) {
      hint.innerHTML = '';
      setScheduleValidationAriaState(idx, false);
    } else {
      const msg = (result.errors || []).join('; ');
      hint.innerHTML = `<span style="color:var(--danger, #ef4444)">✗ ${escapeHtml(msg)}</span>`;
      setScheduleValidationAriaState(idx, true);
    }
  } catch (e) {
    if (_scheduleValidateRequests[idx] !== requestSeq) return;
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
    users: item.type === 'mixed' ? (item.users || []) : undefined,
    lists: item.type === 'mixed' ? (item.lists || []) : undefined,
    following_names: item.type === 'mixed' ? (item.following_names || []) : undefined,
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

let scheduleCodeMirror = null;
let _scheduleCmInitializing = false;

async function initScheduleCodeMirror() {
  if (_scheduleCmInitializing || scheduleCodeMirror) return;
  _scheduleCmInitializing = true;
  await waitForCodeMirror(3000);
  if (document.getElementById('scheduleEditorContainer')) {
    scheduleCodeMirror = initCodeMirror('scheduleEditorContainer', store.state._scheduleRaw, 'yaml');
  }
  _scheduleCmInitializing = false;
}

function syncScheduleTabView() {
  if (store.state._schedules.length === 0 && !store.state.sseConnected) loadSchedules();
  if (store.state._scheduleTab === 'edit' && !store.state._scheduleRaw) loadScheduleRaw();
  if (store.state._scheduleTab === 'edit' && !scheduleCodeMirror) requestAnimationFrame(() => requestAnimationFrame(initScheduleCodeMirror));
}

function renderServerClosedState() {
  document.getElementById('contentContainer').innerHTML = `
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
      const min = parseInt(el.min, 10) || 1;
      const max = parseInt(el.max, 10) || (el.name.includes('routine') ? 100 : 250);
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
  const content = getEditorValue(configCodeMirror, store.state.configRaw);
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
  try {
    const d = await api.getCookies();
    store.setState({ cookieItems: d.items || [], cookiesExists: d.exists || false });
  } catch (e) { toast.show('加载额外账户失败: ' + e.message, 'error'); }
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
  return store.state.cookiesMode === 'raw' && cookiesCodeMirror && getEditorValue(cookiesCodeMirror, store.state.cookiesRaw) !== (store.state.cookiesRaw || '');
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
    const isNewAccount = !items[i].auth_token && !items[i].ct0;

    if (isNewAccount && !authVal && !ct0Val) {
      toast.show(`账户 #${i + 1} 的 Auth Token 和 CT0 不能同时为空`, 'error');
      return;
    }

    cookies.push({
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
  const content = getEditorValue(cookiesCodeMirror, store.state.cookiesRaw);
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
  if (mode !== 'raw' && cookiesCodeMirror) {
    cookiesCodeMirror = destroyCodeMirror(cookiesCodeMirror);
  }
  store.setState({ cookiesMode: mode });
  if (mode === 'raw' && !store.state.cookiesRaw) loadCookiesRaw();
}

function addCookieAccount() {
  const items = [{ index: store.state.cookieItems.length, auth_token: '', ct0: '' }, ...store.state.cookieItems];
  store.setState({ cookieItems: items });
  glowNewFirstItem('systemCookiesPanel');
}

function removeCookieAccount(index) {
  const items = store.state.cookieItems.filter((_, i) => i !== index).map((item, i) => ({ ...item, index: i }));
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
  configCodeMirror = destroyCodeMirror(configCodeMirror);
  cookiesCodeMirror = destroyCodeMirror(cookiesCodeMirror);
  scheduleCodeMirror = destroyCodeMirror(scheduleCodeMirror);
  _scheduleCmInitializing = false;
  renderServerClosedState();
}

function setConfigMode(mode) {
  if (mode !== 'raw' && configCodeMirror) {
    configCodeMirror = destroyCodeMirror(configCodeMirror);
  }
  store.setState({ configMode: mode });
  if (mode === 'raw' && !store.state.configRaw) loadConfigRaw();
}

async function loadLogs() {
  const { logLevel, logSearch, logPagination } = store.state;
  const p = new URLSearchParams();
  if (logLevel !== 'all') p.append('level', logLevel);
  if (logSearch) p.append('q', logSearch);
  p.append('page', logPagination.page);
  p.append('pageSize', logPagination.pageSize);
  try {
    const d = await api.getLogs('?' + p.toString());
    store.setState({ logs: d.logs || [], logPagination: { page: d.page, pageSize: d.pageSize, total: d.total, totalPages: d.totalPages } });
  } catch (e) { toast.show('加载日志失败: ' + e.message, 'error'); }
}

function setLogLevel(level) {
  store.setState({ logLevel: level, logPagination: { ...store.state.logPagination, page: 1 } });
  loadLogs();
  restartLogStreamIfNeeded();
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

let logAutoRefreshTimer = null;
let logStreamConn = null;
let configCodeMirror = null;
let cookiesCodeMirror = null;

let _cmWaitCancelled = false;

function waitForCodeMirror(maxWait) {
  _cmWaitCancelled = false;
  if (typeof CodeMirror !== 'undefined') return Promise.resolve(true);
  return new Promise(resolve => {
    const start = Date.now();
    const check = () => {
      if (_cmWaitCancelled || typeof CodeMirror !== 'undefined' || Date.now() - start > maxWait) {
        resolve(!_cmWaitCancelled && typeof CodeMirror !== 'undefined');
      } else {
        setTimeout(check, 100);
      }
    };
    check();
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

  cm.setSize('100%', 400);
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
  await waitForCodeMirror(3000);
  configCodeMirror = initCodeMirror('configEditorContainer', store.state.configRaw, 'yaml');
}

async function initCookiesCodeMirror() {
  await waitForCodeMirror(3000);
  cookiesCodeMirror = initCodeMirror('cookiesEditorContainer', store.state.cookiesRaw, 'yaml');
}

function toggleLogAutoRefresh() {
  const ns = !store.state.logAutoRefresh;
  store.setState({ logAutoRefresh: ns });
  if (ns) startLogStream();
  else stopLogStream();
}

function cleanupSystemTimers() {
  _cmWaitCancelled = true;
  if (logAutoRefreshTimer) {
    clearTimeout(logAutoRefreshTimer);
    logAutoRefreshTimer = null;
  }
  clearAllScheduleValidationTimers();
  stopLogStream();
}

function buildLogStreamURL() {
  const { logLevel, logSearch } = store.state;
  const p = new URLSearchParams();
  if (logLevel !== 'all') p.append('level', logLevel);
  if (logSearch) p.append('q', logSearch);
  const qs = p.toString();
  return `/api/v1/logs/stream${qs ? '?' + qs : ''}`;
}

let _logStreamConnecting = false;
let _pendingLogStreamConn = null;

function startLogStream() {
  if (logStreamConn || _logStreamConnecting || store.state.currentPage !== 'system' || store.state._systemTab !== 'logs') return;
  _logStreamConnecting = true;
  const conn = new EventSource(buildLogStreamURL());
  _pendingLogStreamConn = conn;
  conn.onopen = () => {
    _pendingLogStreamConn = null;
    logStreamConn = conn;
    _logStreamConnecting = false;
  };
  conn.onmessage = (e) => {
    const line = e.data || '';
    if (!line) return;
    const logs = [line, ...store.state.logs].slice(0, 1000);
    const total = Math.max(store.state.logPagination.total + 1, logs.length);
    store.setState({ logs, logPagination: { ...store.state.logPagination, total, totalPages: Math.max(1, Math.ceil(total / store.state.logPagination.pageSize)) } });
    setTimeout(() => {
      const el = document.getElementById('logContainer');
      if (el) el.scrollTop = 0;
    }, 0);
  };
  conn.onerror = () => {
    _pendingLogStreamConn = null;
    _logStreamConnecting = false;
    if (conn === logStreamConn) {
      logStreamConn.close();
      logStreamConn = null;
    } else {
      conn.close();
    }
    if (store.state.logAutoRefresh && store.state.currentPage === 'system' && store.state._systemTab === 'logs') {
      logAutoRefreshTimer = setTimeout(() => { logAutoRefreshTimer = null; startLogStream(); }, 2000);
    }
  };
}

function stopLogStream() {
  _logStreamConnecting = false;
  if (_pendingLogStreamConn) {
    _pendingLogStreamConn.close();
    _pendingLogStreamConn = null;
  }
  if (logAutoRefreshTimer) {
    clearTimeout(logAutoRefreshTimer);
    logAutoRefreshTimer = null;
  }
  if (logStreamConn) {
    logStreamConn.close();
    logStreamConn = null;
  }
}

function restartLogStreamIfNeeded() {
  if (!store.state.logAutoRefresh) return;
  stopLogStream();
  startLogStream();
}

function syncConfigTabView() {
  if (store.state.configMode === 'form' && (!store.state.configFields || store.state.configFields.length === 0)) {
    loadConfigFields();
  }
  if (store.state.configMode === 'raw' && !store.state.configRaw) {
    loadConfigRaw();
  }
  if (store.state.configMode === 'raw' && !configCodeMirror) {
    requestAnimationFrame(() => requestAnimationFrame(initConfigCodeMirror));
  }
}

function syncCookiesTabView() {
  if (store.state.cookiesMode === 'form' && (!store.state.cookieItems || store.state.cookieItems.length === 0)) {
    loadCookiesItems();
  }
  if (store.state.cookiesMode === 'raw' && !store.state.cookiesRaw) {
    loadCookiesRaw();
  }
  if (store.state.cookiesMode === 'raw' && !cookiesCodeMirror) {
    requestAnimationFrame(() => requestAnimationFrame(initCookiesCodeMirror));
  }
}

function syncLogsTabView() {
  if (store.state.logs.length === 0) {
    loadLogs();
  }
  if (store.state.logAutoRefresh) startLogStream();
}

function syncSystemTabView() {
  if (store.state.currentPage !== 'system') return;

  if (store.state._systemTab === 'config') syncConfigTabView();
  if (store.state._systemTab === 'cookies') syncCookiesTabView();
  if (store.state._systemTab === 'logs') syncLogsTabView();
  if (store.state._systemTab === 'schedules') syncScheduleTabView();
}

function rerenderSystemPanel(panelId, renderFn, resetEditor = null, initEditor = null) {
  if (resetEditor) resetEditor();
  const panel = document.getElementById(panelId);
  if (panel) panel.innerHTML = renderFn();
  if (initEditor) requestAnimationFrame(() => requestAnimationFrame(initEditor));
}

function setSystemTab(tab) {
  if (store.state._systemTab === 'logs' && tab !== 'logs') {
    stopLogStream();
  }
  store.setState({ _systemTab: tab });
  setTimeout(syncSystemTabView, 0);
}

// ============================================
// Navigation & Routing
// ============================================

// Parse URL to determine current page
function parseRoute() {
  const path = window.location.pathname;
  const hash = window.location.hash.slice(1); // Remove #
  
  // Map paths to pages
  const pathMap = {
    '/': 'overview',
    '/tasks': 'tasks',
    '/data': 'data',
    '/schedules': 'schedules',
    '/system': 'system'
  };
  
  // Map hash to data sub-pages
  const hashMap = {
    'users': 'users',
    'lists': 'lists',
    'entities': 'entities',
    'list-entities': 'listEntities',
    'user-links': 'userLinks',
    'previous-names': 'previousNames'
  };
  
  const page = pathMap[path] || 'overview';
  const dataSubPage = hashMap[hash] || 'users';
  
  return { page, dataSubPage };
}

// Update URL based on current page
function updateURL(page, dataSubPage = null) {
  const pathMap = {
    'overview': '/',
    'tasks': '/tasks',
    'data': '/data',
    'schedules': '/schedules',
    'system': '/system'
  };
  
  const hashMap = {
    'users': '',
    'lists': '#lists',
    'entities': '#entities',
    'listEntities': '#list-entities',
    'userLinks': '#user-links',
    'previousNames': '#previous-names'
  };
  
  const path = pathMap[page] || '/';
  const hash = (page === 'data' && dataSubPage) ? hashMap[dataSubPage] : '';
  
  // Use history API to update URL without reloading
  const newUrl = path + hash;
  if (window.location.pathname + window.location.hash !== newUrl) {
    window.history.pushState({ page, dataSubPage }, '', newUrl);
  }
}

function navigateTo(page) {
  if (lastPage === 'system' && page !== 'system') {
    cleanupSystemTimers();
    configCodeMirror = destroyCodeMirror(configCodeMirror);
    cookiesCodeMirror = destroyCodeMirror(cookiesCodeMirror);
    scheduleCodeMirror = destroyCodeMirror(scheduleCodeMirror);
    _scheduleCmInitializing = false;
  }
  store.setState({ currentPage: page });
  
  // Update URL
  updateURL(page, store.state.dataSubPage);
  
  // Update sidebar
  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });
  
  // Update mobile nav
  document.querySelectorAll('.mobile-nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });
  
  // Update title
  const titles = { overview: '概览', tasks: '任务中心', data: '数据管理', schedules: '定时任务', system: '系统' };
  document.getElementById('pageTitle').textContent = titles[page];
  
  // Close sidebar on mobile
  if (store.state.isMobile) {
    document.getElementById('sidebar').classList.remove('open');
  }
  
  // Note: render() is called by subscribe callback when page changes
}

// Handle browser back/forward buttons
window.onpopstate = (event) => {
  const { page, dataSubPage } = parseRoute();
  if (lastPage === 'system' && page !== 'system') {
    cleanupSystemTimers();
    configCodeMirror = destroyCodeMirror(configCodeMirror);
    cookiesCodeMirror = destroyCodeMirror(cookiesCodeMirror);
    scheduleCodeMirror = destroyCodeMirror(scheduleCodeMirror);
    _scheduleCmInitializing = false;
  }
  
  if (page === 'data' && dataSubPage !== store.state.dataSubPage) {
    store.setState({ 
      currentPage: page,
      dataSubPage: dataSubPage 
    });
  } else {
    store.setState({ currentPage: page });
  }
  
  // Update sidebar active state
  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });
  document.querySelectorAll('.mobile-nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });
  
  // Update title
  const titles = { overview: '概览', tasks: '任务中心', data: '数据管理', schedules: '定时任务', system: '系统' };
  document.getElementById('pageTitle').textContent = titles[page];
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
    }
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
    container.innerHTML = pages[page]();
    if (page === 'system') {
      syncSystemTabView();
    }
    
    // Re-attach tab listeners for tasks page
    if (page === 'tasks') {
      document.querySelectorAll('[data-task-tab]').forEach(tab => {
        tab.onclick = () => {
          document.querySelectorAll('[data-task-tab]').forEach(t => t.classList.remove('active'));
          tab.classList.add('active');
          document.getElementById('taskFormContainer').innerHTML = renderTaskForm(tab.dataset.taskTab);
        };
      });
      
      // Restore filter and search values
      restoreSearchValue('taskFilter', 'taskFilter');
      restoreSearchValue('taskSearch', 'taskSearch');
    }
    
    // Attach quick download enter key listener
    if (page === 'overview') {
      const input = document.getElementById('quickDownloadInput');
      if (input) {
        input.onkeypress = (e) => { 
          if (e.key === 'Enter') handleQuickDownload(); 
        };
      }
    }
    
    // Restore search value for data page
    if (page === 'data') {
      restoreSearchValue('dbSearchInput', 'dbSearch', store.state.dataSubPage);
    }

    if (page === 'schedules') {
      if (store.state._schedules.length === 0) loadSchedules();
    }
    
    // Restore search value for logs
    if (page === 'system' && store.state._systemTab === 'logs') {
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

  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });
  document.querySelectorAll('.mobile-nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });

  const titles = { overview: '概览', tasks: '任务中心', data: '数据管理', schedules: '定时任务', system: '系统' };
  document.getElementById('pageTitle').textContent = titles[page] || '概览';

  document.getElementById('contentContainer').innerHTML = `
    <div class="empty-state">
      <div class="skeleton" style="width: 64px; height: 64px; border-radius: var(--radius-xl); margin-bottom: var(--space-4);"></div>
      <div class="empty-title">加载中...</div>
      <div class="empty-desc">正在初始化应用数据</div>
    </div>
  `;

  sseManager.connect();

  try {
    const [health, tasks, config] = await Promise.all([
      api.getHealth(),
      api.getTasks(),
      api.getConfig()
    ]);

    store.setState({
      currentPage: page,
      dataSubPage: dataSubPage,
      health,
      tasks: store.state.tasks.length > 0 ? store.state.tasks : (tasks.tasks || []),
      config
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
  document.getElementById('sidebar').classList.toggle('open');
};

document.querySelectorAll('.nav-item').forEach(el => {
  el.onclick = () => navigateTo(el.dataset.page);
});

document.querySelectorAll('.mobile-nav-item').forEach(el => {
  el.onclick = () => navigateTo(el.dataset.page);
});

document.getElementById('refreshBtn').onclick = () => {
  const page = store.state.currentPage;
  if (page === 'tasks') refreshTasks();
  else if (page === 'data') refreshDBData();
  else if (page === 'schedules') loadSchedules();
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
let lastPage = store.state.currentPage;
let lastTasksJson = JSON.stringify(store.state.tasks);
let lastSystemTab = store.state._systemTab;
let lastLogPaginationJson = JSON.stringify(store.state.logPagination);
let lastConfigRaw = store.state.configRaw;
let lastConfigSaving = store.state.configSaving;
let lastConfigFieldsJson = JSON.stringify(store.state.configFields);
let lastConfigFieldsLoading = store.state.configFieldsLoading;
let lastConfigMode = store.state.configMode;
let lastCookiesRaw = store.state.cookiesRaw;
let lastCookiesSaving = store.state.cookiesSaving;
let lastCookieItemsJson = JSON.stringify(store.state.cookieItems);
let lastCookiesMode = store.state.cookiesMode;
let lastLogsLength = store.state.logs.length;
let lastLogLevel = store.state.logLevel;
let lastDataSubPage = store.state.dataSubPage;
let lastDbDataJson = JSON.stringify(store.state.dbData);
let lastDbPaginationJson = JSON.stringify(store.state.dbPagination);
let lastDbSortJson = JSON.stringify(store.state.dbSort);
let lastSchedulesJson = JSON.stringify(store.state._schedules);
let lastScheduleRaw = store.state._scheduleRaw;
let lastScheduleExists = store.state._scheduleExists;
let lastScheduleSaving = store.state._scheduleSaving;
let lastScheduleTab = store.state._scheduleTab;
let lastScheduleFormItemsJson = JSON.stringify(store.state._scheduleFormItems);
let lastSchedulerRunning = store.state._schedulerRunning;
store.subscribe((state) => {
  if (state.currentPage !== lastPage) {
    lastPage = state.currentPage;
    render();
  } else {
    const tasksJson = JSON.stringify(state.tasks);
    const tasksChanged = tasksJson !== lastTasksJson;

    if (tasksChanged) {
      lastTasksJson = tasksJson;
      if (state.currentPage === 'tasks') { updateTaskListUI(state.tasks); }
      if (state.currentPage === 'overview') { updateOverviewTasksUI(state.tasks); }
    }

    if (state.currentPage === 'data') {
      const dataSubPageChanged = state.dataSubPage !== lastDataSubPage;
      const dbDataChanged = JSON.stringify(state.dbData) !== lastDbDataJson;
      const dbPaginationChanged = JSON.stringify(state.dbPagination) !== lastDbPaginationJson;
      const dbSortChanged = JSON.stringify(state.dbSort) !== lastDbSortJson;

      if (dataSubPageChanged || dbDataChanged || dbPaginationChanged || dbSortChanged) {
        lastDataSubPage = state.dataSubPage;
        lastDbDataJson = JSON.stringify(state.dbData);
        lastDbPaginationJson = JSON.stringify(state.dbPagination);
        lastDbSortJson = JSON.stringify(state.dbSort);
        render();
      }
    }

    if (state.currentPage === 'schedules') {
      const schedulesChanged = JSON.stringify(state._schedules) !== lastSchedulesJson;
      const scheduleExistsChanged = state._scheduleExists !== lastScheduleExists;
      const schedulerRunningChanged = state._schedulerRunning !== lastSchedulerRunning;

      if (schedulesChanged || scheduleExistsChanged || schedulerRunningChanged) {
        lastSchedulesJson = JSON.stringify(state._schedules);
        lastScheduleExists = state._scheduleExists;
        lastSchedulerRunning = state._schedulerRunning;
        render();
      }
    }

    if (state.currentPage === 'system') {
      const tabChanged = state._systemTab !== lastSystemTab;
      const logPagChanged = JSON.stringify(state.logPagination) !== lastLogPaginationJson;
      const configRawChanged = state.configRaw !== lastConfigRaw;
      const configSavingChanged = state.configSaving !== lastConfigSaving;
      const configFieldsChanged = JSON.stringify(state.configFields) !== lastConfigFieldsJson;
      const configFieldsLoadingChanged = state.configFieldsLoading !== lastConfigFieldsLoading;
      const configModeChanged = state.configMode !== lastConfigMode;
      const cookiesChanged = JSON.stringify(state.cookieItems) !== lastCookieItemsJson;
      const cookiesModeChanged = state.cookiesMode !== lastCookiesMode;
      const cookiesRawChanged = state.cookiesRaw !== lastCookiesRaw;
      const cookiesSavingChanged = state.cookiesSaving !== lastCookiesSaving;
      const logsChanged = state.logs.length !== lastLogsLength;
      const logLevelChanged = state.logLevel !== lastLogLevel;
      const schedulesChanged = JSON.stringify(state._schedules) !== lastSchedulesJson;
      const scheduleRawChanged = state._scheduleRaw !== lastScheduleRaw;
      const scheduleExistsChanged = state._scheduleExists !== lastScheduleExists;
      const scheduleSavingChanged = state._scheduleSaving !== lastScheduleSaving;
      const scheduleTabChanged = state._scheduleTab !== lastScheduleTab;
      const scheduleFormItemsChanged = JSON.stringify(state._scheduleFormItems) !== lastScheduleFormItemsJson;

      if (tabChanged) {
        lastSystemTab = state._systemTab;
        document.querySelectorAll('.system-tabs .tab').forEach(t => {
          t.classList.toggle('active', t.textContent.includes(
            state._systemTab === 'config' ? '配置' : state._systemTab === 'cookies' ? '账户' : state._systemTab === 'logs' ? '日志' : '任务配置'
          ));
        });
        document.getElementById('systemConfigPanel').style.display = state._systemTab === 'config' ? '' : 'none';
        document.getElementById('systemCookiesPanel').style.display = state._systemTab === 'cookies' ? '' : 'none';
        document.getElementById('systemLogsPanel').style.display = state._systemTab === 'logs' ? '' : 'none';
        document.getElementById('systemSchedulesPanel').style.display = state._systemTab === 'schedules' ? '' : 'none';
      }

      if (configRawChanged || configSavingChanged || configFieldsChanged || configFieldsLoadingChanged || configModeChanged) {
        lastConfigRaw = state.configRaw;
        lastConfigSaving = state.configSaving;
        lastConfigFieldsJson = JSON.stringify(state.configFields);
        lastConfigFieldsLoading = state.configFieldsLoading;
        lastConfigMode = state.configMode;
        rerenderSystemPanel(
          'systemConfigPanel',
          renderConfigEditor,
          () => { configCodeMirror = destroyCodeMirror(configCodeMirror); },
          state.configMode === 'raw' ? initConfigCodeMirror : null
        );
      }

      if (cookiesChanged || cookiesModeChanged || cookiesRawChanged || cookiesSavingChanged) {
        lastCookieItemsJson = JSON.stringify(state.cookieItems);
        lastCookiesMode = state.cookiesMode;
        lastCookiesRaw = state.cookiesRaw;
        lastCookiesSaving = state.cookiesSaving;
        rerenderSystemPanel(
          'systemCookiesPanel',
          renderCookiesEditor,
          () => { cookiesCodeMirror = destroyCodeMirror(cookiesCodeMirror); },
          state.cookiesMode === 'raw' ? initCookiesCodeMirror : null
        );
      }

      if (logsChanged || logLevelChanged || logPagChanged) {
        lastLogPaginationJson = JSON.stringify(state.logPagination);
        lastLogsLength = state.logs.length;
        lastLogLevel = state.logLevel;
        const panel = document.getElementById('systemLogsPanel');
        if (panel) {
          panel.innerHTML = renderLogViewer();
          restoreSearchValue('logSearchInput', 'logSearch');
        }
      }

      const schedulePanelSchedulesChanged = state._scheduleTab !== 'form' && schedulesChanged;
      if (schedulesChanged && !schedulePanelSchedulesChanged) {
        lastSchedulesJson = JSON.stringify(state._schedules);
      }
      if (schedulePanelSchedulesChanged || scheduleRawChanged || scheduleExistsChanged || scheduleSavingChanged || scheduleTabChanged || scheduleFormItemsChanged) {
        lastSchedulesJson = JSON.stringify(state._schedules);
        lastScheduleRaw = state._scheduleRaw;
        lastScheduleExists = state._scheduleExists;
        lastScheduleSaving = state._scheduleSaving;
        lastScheduleTab = state._scheduleTab;
        lastScheduleFormItemsJson = JSON.stringify(state._scheduleFormItems);
        rerenderSystemPanel(
          'systemSchedulesPanel',
          renderScheduleViewer,
          () => { scheduleCodeMirror = destroyCodeMirror(scheduleCodeMirror); _scheduleCmInitializing = false; },
          state._scheduleTab === 'edit' ? initScheduleCodeMirror : null
        );
      }
    }
  }
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

// Start
init();
