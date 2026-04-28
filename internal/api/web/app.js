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
    selectedTasks: new Set(),
    sidebarOpen: false,
    isMobile: window.innerWidth < 768,
    sseConnected: false,
    dataSubPage: 'users',
    // Database pagination state
    dbData: {
      users: { data: [], total: 0, page: 1, pageSize: 20 },
      lists: { data: [], total: 0, page: 1, pageSize: 20 },
      entities: { data: [], total: 0, page: 1, pageSize: 20 },
      listEntities: { data: [], total: 0, page: 1, pageSize: 20 },
      userLinks: { data: [], total: 0, page: 1, pageSize: 20 }
    },
    dbPagination: {
      users: { page: 1, pageSize: 20, totalPages: 1 },
      lists: { page: 1, pageSize: 20, totalPages: 1 },
      entities: { page: 1, pageSize: 20, totalPages: 1 },
      listEntities: { page: 1, pageSize: 20, totalPages: 1 },
      userLinks: { page: 1, pageSize: 20, totalPages: 1 }
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
    logs: [],
    logLevel: 'all',
    logSearch: '',
    logAutoRefresh: false,
    logPagination: { page: 1, pageSize: 100, total: 0, totalPages: 1 },
    _systemTab: 'config',
    configMode: 'form',
    configFields: [],
  },

  listeners: [],

  subscribe(fn) {
    this.listeners.push(fn);
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
  
  async request(method, path, body = null) {
    const options = {
      method,
      headers: { 'Content-Type': 'application/json' }
    };
    if (body) options.body = JSON.stringify(body);
    
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
  createListDownload(listId, opts) { 
    return this.post(`/api/v1/lists/${encodeURIComponent(listId)}/download`, opts); 
  },
  createListProfile(listId) { 
    return this.post(`/api/v1/lists/${encodeURIComponent(listId)}/profile`, {}); 
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
  
  // Config
  getConfig() { return this.get('/api/v1/config'); },
  getConfigRaw() { return this.get('/api/v1/config/raw'); },
  updateConfigRaw(content) { return this.request('PUT', '/api/v1/config/raw', { content }); },
  getConfigFields() { return this.get('/api/v1/config/fields'); },
  saveConfigFields(fields) { return this.request('PUT', '/api/v1/config/fields', { fields }); },

  // Logs
  getLogs(params = '') { return this.get(`/api/v1/logs${params}`); },

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
  reconnectDelay: 2000,
  maxReconnectDelay: 30000,
  
  connect() {
    if (this.conn) return;
    
    this.conn = new EventSource('/api/v1/sse/tasks');
    
    this.conn.onmessage = (e) => {
      try {
        const tasks = JSON.parse(e.data);
        store.setState({ tasks });
        this.reconnectDelay = 2000;
      } catch (err) {}
    };
    
    this.conn.onerror = () => {
      this.conn.close();
      this.conn = null;
      store.setState({ sseConnected: false });
      setTimeout(() => this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    };
    
    store.setState({ sseConnected: true });
  },
  
  disconnect() {
    if (this.conn) {
      this.conn.close();
      this.conn = null;
    }
    store.setState({ sseConnected: false });
  }
};

// ============================================
// Toast Notifications
// ============================================
const toast = {
  container: document.getElementById('toastContainer'),
  maxToasts: 3,
  
  show(message, type = 'success', title = '') {
    // 限制最多显示3条消息
    const existingToasts = this.container.querySelectorAll('.toast');
    if (existingToasts.length >= this.maxToasts) {
      // 移除最旧的消息（第一个）
      existingToasts[0].remove();
    }
    
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    
    const icons = { success: '✓', error: '✕', warning: '⚠' };
    const titles = { success: '成功', error: '错误', warning: '警告' };
    
    el.innerHTML = `
      <span class="toast-icon">${icons[type]}</span>
      <div class="toast-content">
        <div class="toast-title">${title || titles[type]}</div>
        <div class="toast-message">${message}</div>
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
            <div class="stat-value">${taskStats.running}</div>
            <div class="stat-label">运行中任务</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon" style="color: var(--success);">✓</div>
          <div class="stat-content">
            <div class="stat-value">${taskStats.completed}</div>
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
            <button class="btn btn-primary" onclick="handleQuickDownload()">创建任务</button>
          </div>
          <div class="text-sm text-tertiary mt-4">
            支持格式: twitter.com/username | x.com/username | @username
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
                <div class="tab" data-task-tab="batch">批量</div>
                <div class="tab" data-task-tab="jsonfile">JSON文件</div>
                <div class="tab" data-task-tab="jsonfolder">LoongTweet</div>
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
                <div class="card-subtitle">共 ${tasks.length} 个任务</div>
              </div>
            </div>
            <div class="toolbar">
              <div class="toolbar-left">
                <select class="form-select" style="width: 100px;" id="taskFilter">
                  <option value="all">全部状态</option>
                  <option value="running">运行中</option>
                  <option value="queued">排队中</option>
                  <option value="completed">已完成</option>
                  <option value="failed">失败</option>
                </select>
                <input type="text" class="form-input search-input" id="taskSearch" placeholder="搜索任务...">
              </div>
              <div class="toolbar-right">
                <button class="btn btn-ghost btn-sm" onclick="refreshTasks()">🔄 刷新</button>
              </div>
            </div>
            <div class="card-body" style="padding: 0;">
              ${tasks.length === 0 ? `
                <div class="empty-state">
                  <div class="empty-icon">🚀</div>
                  <div class="empty-title">暂无任务</div>
                  <div class="empty-desc">在左侧创建一个新任务开始下载</div>
                </div>
              ` : `
                <div class="task-list" id="taskList">
                  ${tasks.map(t => renderTaskItem(t, true)).join('')}
                </div>
              `}
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
    const pagination = dbPagination[dataSubPage] || { page: 1, pageSize: 20, totalPages: 1 };
    const sort = dbSort[dataSubPage] || { sortBy: 'id', sortOrder: 'desc' };
    const search = dbSearch[dataSubPage] || '';
    
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
              placeholder="搜索..." value="${search}" onkeypress="if(event.key==='Enter')searchDB()">
            <button class="btn btn-ghost btn-icon" onclick="searchDB()">🔍</button>
            <button class="btn btn-ghost btn-icon" onclick="refreshDBData()">🔄</button>
          </div>
        </div>
        
        <div class="card-body" style="padding: 0;">
          ${renderDBTable(dataSubPage, current.data, sort)}
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
  
  // System Page
  system() {
    const { config, configRaw, logs, logLevel, logSearch, logPagination } = store.state;

    return `
      <div class="system-tabs">
        <div class="tab ${store.state._systemTab === 'config' ? 'active' : ''}" onclick="setSystemTab('config')">⚙️ 配置编辑</div>
        <div class="tab ${store.state._systemTab === 'logs' ? 'active' : ''}" onclick="setSystemTab('logs')">📋 系统日志</div>
      </div>

      <div id="systemConfigPanel" class="system-panel" style="${store.state._systemTab === 'config' ? '' : 'display:none'}">
        ${renderConfigEditor()}
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
    'marking': ' · 标记中',
    'completed': ''
  };
  return stageMap[stage] || (stage ? ` · ${stage}` : '');
}

function renderTaskItem(task, showCheckbox = false) {
  const statusMap = {
    queued: { tag: 'tag-queued', text: '排队' },
    running: { tag: 'tag-running', text: '运行' },
    completed: { tag: 'tag-completed', text: '完成' },
    failed: { tag: 'tag-failed', text: '失败' },
    cancelled: { tag: 'tag-cancelled', text: '取消' }
  };
  
  const status = statusMap[task.status] || statusMap.queued;
  const pct = task.progress && task.progress.total ?
    Math.round((task.progress.completed || 0) / task.progress.total * 100) : 0;

  const stageText = task.progress?.stage ? getStageText(task.progress.stage) : '';
  const currentText = task.progress?.current ? ` · ${task.progress.current}` : '';

  const target = task.data?.screen_name || task.data?.list_id || 'Unknown';

  return `
    <div class="task-item" onclick="showTaskDetail('${task.task_id}')">
      ${showCheckbox ? `<div class="task-checkbox"><input type="checkbox" class="form-checkbox" data-task-id="${task.task_id}"></div>` : ''}
      <div class="task-info">
        <div class="task-title">${task.type} - ${target}</div>
        <div class="task-meta">
          <span class="tag ${status.tag}">${status.text}</span>
          <span>ID: ${task.task_id}</span>
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
          `<button class="btn btn-danger btn-sm" onclick="cancelTask('${task.task_id}')">取消</button>` :
          `<button class="btn btn-ghost btn-sm" onclick="showTaskDetail('${task.task_id}')">详情</button>`
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
          <input type="checkbox" id="userAutoFollow"> AutoFollow
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
          <input type="checkbox" id="listAutoFollow"> AutoFollow
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
    batch: `
      <div class="form-group">
        <label class="form-label">用户列表（每行一个）</label>
        <textarea class="form-textarea" id="batchUsers" placeholder="user1\nuser2\nuser3" rows="4"></textarea>
      </div>
      <div class="form-group">
        <label class="form-label">List IDs（每行一个）</label>
        <textarea class="form-textarea" id="batchLists" placeholder="123\n456\n789" rows="3"></textarea>
      </div>
      <div class="form-group">
        <label class="form-checkbox">
          <input type="checkbox" id="batchAutoFollow"> AutoFollow
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
    <label class="form-label">第三方工具导出的JSON文件路径（每行一个）</label>
    <textarea class="form-textarea" id="jsonFilePaths" placeholder="/path/to/twitter-followers-123.json\n/path/to/more.json" rows="4"></textarea>
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
    <label class="form-label">TMD .loongtweet 文件夹路径（每行一个）</label>
    <textarea class="form-textarea" id="jsonFolderPath" placeholder="/path/to/.loongtweet\n/path/to/another/.loongtweet" rows="4"></textarea>
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

// Database Table Renderer with sorting, actions and mobile support
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
    if (sort.sortBy !== field) return '↕️';
    return sort.sortOrder === 'asc' ? '↑' : '↓';
  };

  const sortableHeader = (field, label) => `
    <th onclick="sortDB('${field}')" style="cursor: pointer; user-select: none;">
      ${label} ${sortIcon(field)}
    </th>
  `;

  const columns = {
    users: ['ID', 'Screen Name', 'Name', 'Protected', 'Accessible', 'Friends', 'Actions'],
    lists: ['ID', 'Name', 'Owner ID', 'Actions'],
    entities: ['ID', 'User ID', 'Name', 'Latest Release', 'Media Count', 'Actions'],
    listEntities: ['ID', 'List ID', 'Name', 'Parent Dir', 'Actions'],
    userLinks: ['ID', 'User ID', 'Name', 'Parent Entity', 'Actions']
  };

  const renderActionButtons = (type, item) => {
    // userLinks 是关联表，不支持手动编辑/删除
    if (type === 'userLinks') {
      return '<span style="color: var(--text-tertiary);">-</span>';
    }
    // 将 ID 作为字符串传递，避免 JavaScript 大整数精度丢失
    const idStr = String(item.id);
    return `
      <div class="flex gap-2">
        <button class="btn btn-ghost btn-sm" onclick="editDBItem('${type}', '${idStr}')">✏️</button>
        <button class="btn btn-danger btn-sm" onclick="deleteDBItem('${type}', '${idStr}')">🗑️</button>
      </div>
    `;
  };

  const rows = data.map(item => {
    if (type === 'users') {
      return `<tr>
        <td>${item.id}</td>
        <td>@${item.screen_name}</td>
        <td>${item.name}</td>
        <td>${item.protected ? '🔒' : '🔓'}</td>
        <td>${item.is_accessible ? '✅' : '❌'}</td>
        <td>${item.friends_count}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else if (type === 'lists') {
      return `<tr>
        <td>${item.id}</td>
        <td>${item.name}</td>
        <td>${item.owner_uid}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else if (type === 'entities') {
      return `<tr>
        <td>${item.id}</td>
        <td>${item.user_id}</td>
        <td>${item.name}</td>
        <td>${item.latest_release_time || '-'}</td>
        <td>${item.media_count || '-'}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else if (type === 'listEntities') {
      return `<tr>
        <td>${item.id}</td>
        <td>${item.lst_id}</td>
        <td>${item.name}</td>
        <td>${item.parent_dir}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    } else {
      return `<tr>
        <td>${item.id}</td>
        <td>${item.user_id}</td>
        <td>${item.name}</td>
        <td>${item.parent_lst_entity_id}</td>
        <td>${renderActionButtons(type, item)}</td>
      </tr>`;
    }
  }).join('');

  // Mobile card view
  const mobileCards = data.map(item => {
    if (type === 'users') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">@${item.screen_name}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">${item.name}</div>
          <div style="display: flex; gap: var(--space-4); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <span>${item.protected ? '🔒 Protected' : '🔓 Public'}</span>
            <span>${item.is_accessible ? '✅ Accessible' : '❌ Not Accessible'}</span>
          </div>
          <div style="font-size: var(--text-sm); margin-bottom: var(--space-2);">Friends: ${item.friends_count}</div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else if (type === 'lists') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${item.name}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${item.id}</div>
            <div>Owner: ${item.owner_uid}</div>
          </div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else if (type === 'entities') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${item.name}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${item.id}</div>
            <div>User ID: ${item.user_id}</div>
            <div>Media: ${item.media_count || 0}</div>
          </div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else if (type === 'listEntities') {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${item.name}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${item.id}</div>
            <div>List ID: ${item.lst_id}</div>
            <div>Dir: ${item.parent_dir}</div>
          </div>
          <div>${renderActionButtons(type, item)}</div>
        </div>
      `;
    } else {
      return `
        <div class="mobile-card">
          <div style="font-weight: var(--font-semibold); margin-bottom: var(--space-2);">${item.name}</div>
          <div style="color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: var(--space-2);">
            <div>ID: ${item.id}</div>
            <div>User ID: ${item.user_id}</div>
            <div>Entity: ${item.parent_lst_entity_id}</div>
          </div>
        </div>
      `;
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
    <div class="mobile-card-list">
      ${mobileCards}
    </div>
  `;
}

function renderPageNumbers(currentPage, totalPages) {
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
    return `<button class="page-btn ${p === currentPage ? 'active' : ''}" onclick="goToDBPage(${p})">${p}</button>`;
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
      // API 返回的是分页数据对象（因为 api.request 返回 data.data）
      const data = response || {};
      store.setState({
        dbData: {
          ...store.state.dbData,
          [dataSubPage]: {
            data: data.data || [],
            total: data.total || 0,
            page: data.page || 1,
            pageSize: data.pageSize || 20
          }
        },
        dbPagination: {
          ...store.state.dbPagination,
          [dataSubPage]: {
            page: data.page || 1,
            pageSize: data.pageSize || 20,
            totalPages: data.totalPages || 1
          }
        }
      });
      render();
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
  const input = document.getElementById('dbSearchInput');
  const { dataSubPage, dbSearch } = store.state;

  store.setState({
    dbSearch: {
      ...dbSearch,
      [dataSubPage]: input.value.trim()
    },
    dbPagination: {
      ...store.state.dbPagination,
      [dataSubPage]: { ...store.state.dbPagination[dataSubPage], page: 1 }
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
        <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${item.id}</div>
      </div>
    `;

    switch (type) {
      case 'users':
        content += `
          <div class="form-group">
            <label class="form-label">Screen Name</label>
            <input type="text" class="form-input" id="editScreenName" value="${item.screen_name || ''}">
          </div>
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editName" value="${item.name || ''}">
          </div>
          <div class="form-group">
            <label class="form-label">Friends Count</label>
            <input type="number" class="form-input" id="editFriendsCount" value="${item.friends_count || 0}">
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
            <input type="text" class="form-input" id="editListName" value="${item.name || ''}">
          </div>
          <div class="form-group">
            <label class="form-label">Owner ID</label>
            <input type="text" class="form-input" id="editListOwnerId" value="${item.owner_uid || ''}">
          </div>
        `;
        break;
      case 'entities':
        content += `
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editEntityName" value="${item.name || ''}">
          </div>
          <div class="form-group">
            <label class="form-label">User ID</label>
            <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${item.user_id}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Parent Dir</label>
            <input type="text" class="form-input" id="editEntityParentDir" value="${item.parent_dir || ''}">
          </div>
          <div class="form-group">
            <label class="form-label">Media Count</label>
            <input type="number" class="form-input" id="editEntityMediaCount" value="${item.media_count || 0}">
          </div>
        `;
        break;
      case 'listEntities':
        content += `
          <div class="form-group">
            <label class="form-label">Name</label>
            <input type="text" class="form-input" id="editListEntityName" value="${item.name || ''}">
          </div>
          <div class="form-group">
            <label class="form-label">List ID</label>
            <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${item.lst_id}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Parent Dir</label>
            <input type="text" class="form-input" id="editListEntityParentDir" value="${item.parent_dir || ''}">
          </div>
        `;
        break;
    }

    const footer = `
      <button class="btn btn-secondary" onclick="drawer.close()">取消</button>
      <button class="btn btn-primary" onclick="saveDBItem('${type}', '${id}')">保存</button>
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
      data.owner_uid = document.getElementById('editListOwnerId').value.trim();
      if (!data.name) return toast.show('Name is required', 'error');
      break;
    case 'entities':
      data.name = document.getElementById('editEntityName').value.trim();
      data.parent_dir = document.getElementById('editEntityParentDir').value.trim();
      data.media_count = parseInt(document.getElementById('editEntityMediaCount').value) || 0;
      if (!data.name) return toast.show('Name is required', 'error');
      break;
    case 'listEntities':
      data.name = document.getElementById('editListEntityName').value.trim();
      data.parent_dir = document.getElementById('editListEntityParentDir').value.trim();
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
  const value = input.value.trim();
  if (!value) return toast.show('请输入用户名或链接', 'error');

  // Extract username from various formats
  let username = value;
  const match = value.match(/(?:twitter\.com|x\.com)\/([^/\s?]+)/);
  if (match) username = match[1];
  if (username.startsWith('@')) username = username.slice(1);

  try {
    await api.createUserDownload(username, { auto_follow: true });
    toast.show(`已创建用户下载任务: @${username}`);
    input.value = '';
    refreshTasks();
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
      skip_profile: document.getElementById('userSkipProfile').checked,
      no_retry: document.getElementById('userNoRetry').checked
    });
    toast.show('用户下载任务已创建');
    document.getElementById('userScreenName').value = '';
    refreshTasks();
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
    refreshTasks();
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
      skip_profile: document.getElementById('listSkipProfile').checked,
      no_retry: document.getElementById('listNoRetry').checked
    });
    toast.show('列表下载任务已创建');
    document.getElementById('listId').value = '';
    refreshTasks();
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
    refreshTasks();
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createBatchTask() {
  const users = document.getElementById('batchUsers').value.split('\n').map(s => s.trim()).filter(Boolean);
  const lists = document.getElementById('batchLists').value.split('\n').map(s => parseInt(s.trim())).filter(id => !isNaN(id));
  
  if (!users.length && !lists.length) return toast.show('请输入至少一个用户或列表', 'error');
  
  try {
    await api.createBatchDownload({
      users,
      lists,
      auto_follow: document.getElementById('batchAutoFollow').checked,
      skip_profile: document.getElementById('batchSkipProfile').checked,
      no_retry: document.getElementById('batchNoRetry').checked
    });
    toast.show(`批量任务已创建 (${users.length} 用户, ${lists.length} 列表)`);
    refreshTasks();
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createJsonFileTask() {
  const paths = document.getElementById('jsonFilePaths').value.split('\n').map(s => s.trim()).filter(Boolean);
  if (!paths.length) return toast.show('请输入至少一个 JSON 文件路径', 'error');

  try {
    await api.createJsonFileDownload({
      paths,
      no_retry: document.getElementById('jsonFileNoRetry').checked
    });
    toast.show('JSON 文件任务已创建');
    refreshTasks();
  } catch (err) {
    toast.show(err.message, 'error');
  }
}

async function createJsonFolderTask() {
  const paths = document.getElementById('jsonFolderPath').value.split('\n').map(s => s.trim()).filter(Boolean);
  if (!paths.length) return toast.show('请输入至少一个 LoongTweet 文件夹路径', 'error');

  try {
    await api.createJsonFolderDownload({
      paths,
      no_retry: document.getElementById('jsonFolderNoRetry').checked
    });
    toast.show('LoongTweet 任务已创建');
    refreshTasks();
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
  
  const pct = task.progress && task.progress.total ?
    Math.round((task.progress.completed || 0) / task.progress.total * 100) : 0;

  const stageText = task.progress?.stage ? getStageText(task.progress.stage) : '';
  const currentText = task.progress?.current ? ` · ${task.progress.current}` : '';

  const content = `
    <div class="form-group">
      <label class="form-label">任务 ID</label>
      <div class="font-mono text-sm" style="background: var(--bg-primary); padding: var(--space-3); border-radius: var(--radius-md);">${task.task_id}</div>
    </div>
    <div class="form-group">
      <label class="form-label">类型</label>
      <div>${task.type}</div>
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
      ${task.progress?.failed ? `<div class="text-sm" style="color: var(--danger); margin-top: 4px;">失败: ${task.progress.failed}</div>` : ''}
    </div>
    <div class="form-group">
      <label class="form-label">创建时间</label>
      <div>${new Date(task.created_at).toLocaleString()}</div>
    </div>
    ${task.error ? `
      <div class="form-group">
        <label class="form-label" style="color: var(--danger);">错误信息</label>
        <div style="color: var(--danger); background: var(--danger-bg); padding: var(--space-3); border-radius: var(--radius-md);">${task.error}</div>
      </div>
    ` : ''}
  `;
  
  const footer = task.status === 'running' || task.status === 'queued' ? 
    `<button class="btn btn-danger" onclick="cancelTask('${task.task_id}'); drawer.close();">取消任务</button>` : 
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

function refreshLogs() { loadLogs(); }

function clearLogs() { toast.show('日志清空功能暂未开放', 'warning'); }

function escapeHtml(str) {
  const d = document.createElement('div');
  d.textContent = str;
  return d.innerHTML;
}

function stripAnsi(str) { return str.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, ''); }

function renderConfigEditor() {
  const { configMode, configFields, configSaving, configExists, configRaw } = store.state;

  const modeTabs = `
    <div class="config-mode-tabs">
      <button class="mode-tab ${configMode === 'form' ? 'active' : ''}" onclick="setConfigMode('form')">📝 简易模式</button>
      <button class="mode-tab ${configMode === 'raw' ? 'active' : ''}" onclick="setConfigMode('raw')">🔧 高级 (YAML)</button>
    </div>
  `;

  if (configMode === 'raw') return modeTabs + renderConfigRawEditor(configRaw, configSaving, configExists);
  return modeTabs + renderConfigForm(configFields, configSaving, configExists);
}

function renderConfigForm(fields, saving, exists) {
  if (!fields || fields.length === 0) {
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
        <label class="config-label">${f.label}</label>
        ${f.type === 'password' ? `<div class="config-mask-hint">当前值: ${escapeHtml(f.value)}</div>` : ''}
        <input type="${inputType}" class="form-input config-input" id="cf_${f.name}"
          name="${f.name}" value="${escapeHtml(f.type === 'password' ? '' : f.value)}"
          placeholder="${escapeHtml(f.placeholder || f.prompt)}"
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

    <div class="card mt-4">
      <div class="card-header"><div class="card-title">当前生效配置</div></div>
      <div class="card-body">
        <div class="stats-grid">
          <div class="stat-card"><div class="stat-icon">📁</div><div class="stat-content"><div class="stat-value" style="font-size:var(--text-lg)">${escapeHtml(store.state.config?.root_path || '-')}</div><div class="stat-label">存储路径</div></div></div>
          <div class="stat-card"><div class="stat-icon">⚡</div><div class="stat-content"><div class="stat-value">${store.state.config?.max_download_routine || '-'}</div><div class="stat-label">并发下载数</div></div></div>
          <div class="stat-card"><div class="stat-icon">📝</div><div class="stat-content"><div class="stat-value">${store.state.config?.max_file_name_len || '-'}</div><div class="stat-label">文件名长度</div></div></div>
          <div class="stat-card"><div class="stat-icon">🌐</div><div class="stat-content"><div class="stat-value" style="font-size:var(--text-lg)">${escapeHtml(store.state.config?.proxy_url || '未设置')}</div><div class="stat-label">代理地址</div></div></div>
        </div>
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
          <button class="btn btn-ghost btn-sm" onclick="resetConfig()">🔄 重置</button>
          <button class="btn btn-primary btn-sm" onclick="saveConfig()" ${saving ? 'disabled' : ''}>
            ${saving ? '<span class="loading-spinner"></span> 保存中...' : '💾 保存配置'}
          </button>
        </div>
      </div>
      <div class="card-body" style="padding:0;">
        <textarea id="configEditor" class="form-textarea config-editor"
          placeholder="# TMD Configuration\nroot_path: ./downloads\ncookie:\n  auth_token: your_auth_token\n  ct0: your_ct0\nmax_download_routine: 35\nmax_file_name_len: 158\nproxy_url:"
          spellcheck="false">${escapeHtml(raw)}</textarea>
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
    for (const [key, val] of [['debug','DEBUG'],['info','INFO'],['warn','WARN'],['warning','WARNING'],['error','ERROR']]) {
      if (line.includes('level=' + key)) return val === 'ERROR' ? 'var(--danger)' : val === 'WARNING' || val === 'WARN' ? 'var(--warning)' : val === 'INFO' ? 'var(--info)' : 'var(--text-tertiary)';
    }
    return 'var(--text-secondary)';
  }
  function renderLine(line) { return `<div style="color:${getLineColor(line)};font-family:var(--font-mono);font-size:12px;padding:1px 8px;white-space:pre-wrap;word-break:break-all">${escapeHtml(stripAnsi(line))}</div>`; }

  return `
    <div class="card">
      <div class="card-header">
        <div><div class="card-title">系统日志</div><div class="card-subtitle">共 ${logPagination.total} 条记录</div></div>
        <div class="flex gap-2 items-center flex-wrap">
          <input type="text" class="form-input search-input" id="logSearchInput"
            placeholder="🔍 搜索..." value="${escapeHtml(logSearch)}"
            onkeypress="if(event.key==='Enter'){store.setState({logSearch:this.value});refreshLogs();}">
          <div class="log-level-filters">
            ${['all','debug','info','warn','error'].map(l => `<button class="btn btn-sm ${logLevel===l?'btn-primary':'btn-ghost'}" onclick="setLogLevel('${l}')">${l.toUpperCase()}</button>`).join('')}
          </div>
          <button class="btn btn-ghost btn-sm ${logAutoRefresh?'active':''}" onclick="toggleLogAutoRefresh()">${logAutoRefresh?'⏸️':'▶️'} 自动刷新</button>
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
          ${renderPageNumbers(logPagination.page, logPagination.totalPages)}
          <button class="page-btn" onclick="changeLogPage(1)" ${logPagination.page >= logPagination.totalPages ? 'disabled' : ''}>→</button>
        </div>
      </div>
    </div>
  `;
}

async function loadConfigFields() {
  try {
    const d = await api.getConfigFields();
    store.setState({ configFields: d.fields || [], configExists: d.exists || false });
  } catch (e) { toast.show('加载配置失败: ' + e.message, 'error'); }
}

async function loadConfigRaw() {
  try {
    const d = await api.getConfigRaw();
    store.setState({ configRaw: d.content || '', configExists: d.exists || false });
  } catch (e) { toast.show('加载配置失败: ' + e.message, 'error'); }
}

// 保存配置后刷新状态的通用函数
async function refreshConfigAfterSave(mode) {
  const cfg = await api.getConfig();
  store.setState({ config: cfg });
  if (mode === 'form') {
    await loadConfigFields();
  } else {
    await loadConfigRaw();
  }
}

async function saveConfigForm() {
  const inputs = document.querySelectorAll('.config-input[name]');
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
    await api.saveConfigFields(fields);
    toast.show('✅ 配置保存成功', 'success');
    await refreshConfigAfterSave('form');
  } catch (e) { toast.show('❌ 保存失败: ' + e.message, 'error'); }
  finally { store.setState({ configSaving: false }); }
}

async function saveConfig() {
  const el = document.getElementById('configEditor');
  const content = el?.value || '';
  if (!content.trim()) return toast.show('配置不能为空', 'error');
  store.setState({ configSaving: true });
  try {
    await api.updateConfigRaw(content);
    toast.show('✅ 配置保存成功', 'success');
    await refreshConfigAfterSave('raw');
  } catch (e) { toast.show('❌ 保存失败: ' + e.message, 'error'); }
  finally { store.setState({ configSaving: false }); }
}

function resetConfig() {
  if (store.state.configMode === 'form') { loadConfigFields(); }
  else { loadConfigRaw(); }
  toast.show('配置已重置', 'info');
}

function setConfigMode(mode) {
  store.setState({ configMode: mode });
  if (mode === 'form' && (!store.state.configFields || store.state.configFields.length === 0)) { loadConfigFields(); }
  if (mode === 'raw' && !store.state.configRaw) { loadConfigRaw(); }
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
}

function changeLogPage(delta) {
  const p = store.state.logPagination;
  const np = p.page + delta;
  if (np >= 1 && np <= p.totalPages) { store.setState({ logPagination: { ...p, page: np } }); loadLogs(); }
}

let logAutoRefreshTimer = null;

function toggleLogAutoRefresh() {
  const ns = !store.state.logAutoRefresh;
  store.setState({ logAutoRefresh: ns });
  if (ns) { logAutoRefreshTimer = setInterval(loadLogs, 5000); }
  else { clearInterval(logAutoRefreshTimer); logAutoRefreshTimer = null; }
}

function cleanupSystemTimers() {
  if (logAutoRefreshTimer) {
    clearInterval(logAutoRefreshTimer);
    logAutoRefreshTimer = null;
  }
  if (store.state.logAutoRefresh) {
    store.setState({ logAutoRefresh: false });
  }
}

function setSystemTab(tab) {
  store.setState({ _systemTab: tab });
  if (tab === 'config') {
    if (store.state.configMode === 'form' && (!store.state.configFields || store.state.configFields.length === 0)) { loadConfigFields(); }
    if (store.state.configMode === 'raw' && !store.state.configRaw) { loadConfigRaw(); }
  }
  if (tab === 'logs' && store.state.logs.length === 0) { loadLogs(); if (!logAutoRefreshTimer && store.state.logAutoRefresh) logAutoRefreshTimer = setInterval(loadLogs, 5000); }
  render();
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
  if (lastPage === 'system' && page !== 'system') cleanupSystemTimers();
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
  const titles = { overview: '概览', tasks: '任务中心', data: '数据管理', system: '系统' };
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
  const titles = { overview: '概览', tasks: '任务中心', data: '数据管理', system: '系统' };
  document.getElementById('pageTitle').textContent = titles[page];
  
  render();
};

function setDataSubPage(subPage) {
  store.setState({
    dataSubPage: subPage,
    dbPagination: {
      ...store.state.dbPagination,
      [subPage]: { page: 1, pageSize: 20, totalPages: 1 }
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
    
    // Re-attach tab listeners for tasks page
    if (page === 'tasks') {
      document.querySelectorAll('[data-task-tab]').forEach(tab => {
        tab.onclick = () => {
          document.querySelectorAll('[data-task-tab]').forEach(t => t.classList.remove('active'));
          tab.classList.add('active');
          document.getElementById('taskFormContainer').innerHTML = renderTaskForm(tab.dataset.taskTab);
        };
      });
      
      // Attach filter and search listeners
      const filterSelect = document.getElementById('taskFilter');
      const searchInput = document.getElementById('taskSearch');
      if (filterSelect) {
        filterSelect.onchange = () => filterTasks();
      }
      if (searchInput) {
        searchInput.oninput = () => filterTasks();
      }
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
// Utility: Format date
function formatDate(dateStr) {
  if (!dateStr) return '-';
  const date = new Date(dateStr);
  return date.toLocaleString('zh-CN', { 
    year: 'numeric', 
    month: '2-digit', 
    day: '2-digit', 
    hour: '2-digit', 
    minute: '2-digit' 
  });
}

// Utility: Show loading state on button
function setButtonLoading(btn, loading) {
  if (loading) {
    btn.dataset.originalText = btn.innerHTML;
    btn.innerHTML = '<span class="loading-spinner"></span> 处理中...';
    btn.disabled = true;
  } else {
    btn.innerHTML = btn.dataset.originalText || btn.innerHTML;
    btn.disabled = false;
  }
}

async function init() {
  // Parse URL route first
  const { page, dataSubPage } = parseRoute();
  
  // Set initial state based on URL
  store.setState({
    currentPage: page,
    dataSubPage: dataSubPage
  });
  
  // Update sidebar active state
  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });
  document.querySelectorAll('.mobile-nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === page);
  });
  
  // Update title
  const titles = { overview: '概览', tasks: '任务中心', data: '数据管理', system: '系统' };
  document.getElementById('pageTitle').textContent = titles[page] || '概览';
  
  // Show loading state
  document.getElementById('contentContainer').innerHTML = `
    <div class="empty-state">
      <div class="skeleton" style="width: 64px; height: 64px; border-radius: var(--radius-xl); margin-bottom: var(--space-4);"></div>
      <div class="empty-title">加载中...</div>
      <div class="empty-desc">正在初始化应用数据</div>
    </div>
  `;

  // Load initial data
  try {
    const [health, tasks, config] = await Promise.all([
      api.getHealth(),
      api.getTasks(),
      api.getConfig()
    ]);

    store.setState({
      health,
      tasks: tasks.tasks || [],
      config
    });

    // Load database data for current subpage
    await refreshDBData();
  } catch (err) {
    toast.show('加载数据失败: ' + err.message, 'error');
  }
  
  // Connect SSE
  sseManager.connect();
  
  // Initial render
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
let lastConfigMode = store.state.configMode;
let lastLogsLength = store.state.logs.length;
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

    if (state.currentPage === 'data') { render(); }

    if (state.currentPage === 'system') {
      const tabChanged = state._systemTab !== lastSystemTab;
      const logPagChanged = JSON.stringify(state.logPagination) !== lastLogPaginationJson;
      const configRawChanged = state.configRaw !== lastConfigRaw;
      const configSavingChanged = state.configSaving !== lastConfigSaving;
      const configFieldsChanged = JSON.stringify(state.configFields) !== lastConfigFieldsJson;
      const configModeChanged = state.configMode !== lastConfigMode;
      const logsChanged = state.logs.length !== lastLogsLength;

      if (tabChanged || logPagChanged || configRawChanged || configSavingChanged || logsChanged || configFieldsChanged || configModeChanged) {
        lastSystemTab = state._systemTab;
        lastLogPaginationJson = JSON.stringify(state.logPagination);
        lastConfigRaw = state.configRaw;
        lastConfigSaving = state.configSaving;
        lastConfigFieldsJson = JSON.stringify(state.configFields);
        lastConfigMode = state.configMode;
        lastLogsLength = state.logs.length;
        render();
      }
    }
  }
});

// Update overview page recent tasks without full re-render
function updateOverviewTasksUI(tasks) {
  const recentTasks = tasks.slice(0, 5);
  const taskList = document.querySelector('.overview-tasks-list');
  if (!taskList) return;
  
  if (recentTasks.length === 0) {
    taskList.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">📋</div>
        <div class="empty-title">暂无任务</div>
        <div class="empty-desc">创建一个新任务开始下载 Twitter 媒体文件</div>
      </div>
    `;
  } else {
    taskList.innerHTML = recentTasks.map(t => renderTaskItem(t)).join('');
  }
}

// Update only the task list part of the UI without full re-render
function updateTaskListUI(tasks) {
  const taskList = document.getElementById('taskList');
  if (!taskList) return;
  
  const filter = document.getElementById('taskFilter')?.value || 'all';
  const search = document.getElementById('taskSearch')?.value?.toLowerCase() || '';
  
  let filtered = tasks;
  
  if (filter !== 'all') {
    filtered = filtered.filter(t => t.status === filter);
  }
  
  if (search) {
    filtered = filtered.filter(t => {
      const target = (t.data?.screen_name || t.data?.list_id || '').toString().toLowerCase();
      return target.includes(search) || t.task_id.toLowerCase().includes(search);
    });
  }
  
  if (filtered.length === 0) {
    taskList.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">🔍</div>
        <div class="empty-title">没有找到匹配的任务</div>
        <div class="empty-desc">尝试调整筛选条件或搜索关键词</div>
      </div>
    `;
  } else {
    taskList.innerHTML = filtered.map(t => renderTaskItem(t, true)).join('');
  }
  
  // Update task count subtitle
  const subtitle = document.querySelector('.card-subtitle');
  if (subtitle) {
    subtitle.textContent = `共 ${tasks.length} 个任务`;
  }
}

// Start
init();