# web2 前端代码规范速查

## 与 web1 的核心差异

| 特性 | web1 | web2 |
|---|---|---|
| 事件系统 | `data-action` 统一 dispatch | `onclick="fn()"` 内联 + `addEventListener` |
| 函数导出 | 模块对象方法 | `window.X = X`（顶层函数） |
| Toast | `toast.show(msg, type)` 对象方法 | `toast(msg, type)` 函数 |
| Modal | `drawer.open()` 预建组件 | `openModal(html)` 动态创建 |
| 转义 | `escapeHtml(str)` / `escapeAttr(str)` | `esc(str)` — 同时用于内容和属性 |
| 状态管理 | `store.setState/subscribe` | 模块级 let 变量 + 函数内直接赋值 |
| CSS 状态类 | `tag-queued` / `tag-running` 等 | `badge-queued` / `badge-running` 等 |
| 按钮样式 | `btn btn-secondary` | `btn btn-ghost` |
| 文本颜色 | 无内置 | `.text-muted` 类（`color: var(--text-secondary)`）|

## 文件结构

```
internal/api/web/web2/
├── index.html      # 单页 HTML，加载 /static/app.js + /static/styles.css
├── app.js          # 全部 JS（~2475 行）
└── styles.css      # 全部样式（~800 行），锌灰 + 蓝色强调 + 系统字体
```

## app.js 段落布局

| 段落 | 行号 | 内容 |
|---|---|---|
| API Client | 1-112 | `API_BASE`, `fetchWithTimeout`, `API` 对象（\_fetch/get/post/put/patch/del/\_tryRefreshJWT/\_doRefreshJWT）|
| Utility | 114-233 | `esc`, `jsEsc`, `getTweetId`, `getLogLineColor`, `relativeTime`, `formatTime`, `formatDuration`, `getTaskProgressPercent`, `getStageText`, `getTaskTarget`, `taskTypeName`, `taskTypeIcon` |
| Toast | 235-264 | `toast(msg, type)`, `dismissToast(id)` |
| Modal | 266-279 | `openModal(html)`, `closeModal()` |
| Global State | 281-294 | `pageTasks`, `sseConnected`, `pageRenderers`, `currentPage`, `debounce` |
| API Endpoints | 296-441 | `ENDPOINTS` 对象 |
| SSE | 444-545 | `sseJWT`, `tryRefreshJWT`, `debouncedTasksUpdate`, `debouncedSchedulesUpdate`, `connectSSE` |
| Navigation | 547-605 | `navigateTo`, `renderPage`, `loadPageModule` |
| Pages | 607-1200 | 页面渲染器：`renderTasksPage`, `renderDataPage`, `renderSchedulesPage`, `renderSystemPage`, `renderLogsPage` |
| Schedules | 1202-1694 | 计划页面逻辑 |
| System | 1696-2007 | 系统页面（配置/Cookies/Security）+ `loginWithApiKey`, `checkAuth` |
| Logs | 2009-2351 | 日志 SSE、错误面板、Auth Dialog |
| Init | 2353-2475 | `showAuthDialog`, `submitAuthKey`, `checkAuth`, DOMContentLoaded, window exports |

## 事件系统

### onclick 模式（主要）

```html
<button class="btn btn-primary" onclick="saveConfigFields()">Save</button>
<button class="btn btn-ghost btn-sm" onclick="closeModal()">Cancel</button>
```

对应函数在文件底部导出：

```js
window.ENDPOINTS = ENDPOINTS;
window.apiBase = apiBase;
```

### addEventListener 模式（导航/标签切换）

```js
document.querySelectorAll('.nav-item').forEach(el => {
  el.addEventListener('click', () => navigateTo(el.dataset.page));
});
configTabs.addEventListener('click', (e) => {
  const tab = e.target.closest('[data-configtab]');
  if (!tab) return;
  // ...
});
```

## API 调用模式

```js
// API 调用通过 ENDPOINTS 对象
const r = await ENDPOINTS.tasks();
const stats = await ENDPOINTS.taskStats();
await ENDPOINTS.cancelTask(taskId);

// 标准 try/catch + toast 错误处理
try {
  const r = await ENDPOINTS.xxx();
  toast('Success message', 'success');
} catch(e) {
  toast(e.message, 'error');
}
```

## 渲染模式

### 函数签名

```js
async function renderTasksPage(container) { ... }
function renderSystemPage(container) { ... }
```

### 容器赋值

```js
// 设置
container.innerHTML = `
  <div class="section">
    ...
  </div>`;

// 追加（罕见，仅系统页面）
// 使用 += 追加额外内容
```

### 局部更新（特殊场景）

```js
// 逐元素更新（错误面板、安全状态）
function updateSecStatus(msg, color) {
  const st = document.getElementById('sec-status');
  if (st) { st.textContent = msg; st.style.color = color || 'var(--text)'; }
}
```

## XSS 安全

| 场景 | 函数 | 说明 |
|---|---|---|
| HTML 内容插值 | `esc(str)` | 使用 `document.createTextNode` 白名单方式 |
| JS 字符串插值 | `jsEsc(str)` | 用于内联 onclick 参数中的字符串 |
| 属性 | 不常用 — 模板字面量中很少拼接属性值 | 但若需要，使用 `esc()` 也可以 |

## 状态管理

### 全局变量

```js
let pageTasks = [];
let sseConnected = false;
let pageRenderers = {};
let currentPage = 'tasks';
let _errorsData = null;
let _logSSETimer = null;
```

### SSE 数据流

```js
// SSE 事件 → debounce → 更新变量 → 重新渲染当前页面
sseSource.addEventListener('tasks', (e) => {
  const tasks = JSON.parse(e.data);
  if (Array.isArray(tasks)) {
    debouncedTasksUpdate(tasks);  // → pageTasks = tasks → updateTasksView()
    loadErrors();                 // 在 tasks SSE 到达时自动刷新错误
  }
});
```

## 错误处理标准模式

```js
try { await ENDPOINTS.xxx(); toast('Success', 'success'); }
catch(e) { toast(e.message, 'error'); }
```

## Modal 使用

```js
openModal(`
  <div class="modal-header"><h2>Title</h2></div>
  <div class="modal-body">
    <p>Content</p>
  </div>
  <div class="modal-footer">
    <button class="btn btn-ghost btn-sm" onclick="closeModal()">Cancel</button>
    <button class="btn btn-primary btn-sm" onclick="submitAction()">Confirm</button>
  </div>
`);
```

## CSS 变量（与 web1 相同体系，不同类名）

| web2 类 | web1 类 | 用途 |
|---|---|---|
| `.badge-{queued,running,completed,failed,cancelled}` | `.tag-{queued,running,completed,failed,cancelled}` | 状态标签 |
| `.btn-ghost` | `.btn-secondary` | 次要/幽灵按钮 |
| `.text-muted` | 无内置 | 次要文字颜色 |
| `.mono` 类 | `.font-mono` 内联 | 等宽字体 |
| `.form-row` | `.form-group` 内 flex | 表单行 |
| `.form-row-flex` | 无 | 动态 cookie 行的 flex 变体 |

变量系统完全相同（`--bg`、`--text`、`--accent`、`--green`、`--red`、`--border` 等）。

## 常见陷阱

1. **onclick 函数必须在 `window` 上**：所有被 onclick 调用的函数需在底部 `window.X = X` 导出
2. **`esc()` 先转义**：所有模板中的用户输入内容先经 `esc()` 处理
3. **没有 store**：直接操纵 `let` 变量 + `container.innerHTML = '...'`。注意不要在异步函数之间共享可变的 `container` 引用
4. **JWT 令牌**：全部来自 `localStorage.getItem('tmd_jwt_token')`，由 `API._fetch` 自动注入
5. **Log SSE**：有自己的 `connectLogSSE`/`disconnectLogSSE` 和 `_logSSETimer`/`_logReconnectAttempts` 状态
6. **`connectSSE` 的首次延迟**：若无 JWT，`connectSSE` 通过 `window._sseAuthChecked` 延迟到 `checkAuth` 确认后才真正连接

## 修改前必读

- 新功能：在相应的 `render*Page` 函数中添加模板，并在 `ENDPOINTS` 中添加 API 调用
- 新 System tab：将标签按钮添加到 `#config-tabs`，并在 `loadConfigTab` 的 switch 中添加 case
- 新函数若需 onclick 访问：在文件底部 `window.X = X` 导出
- 新 endpoint 方法：在 `API` 对象中方法存在于 `get/post/put/patch/del`，在 `ENDPOINTS` 中添加绑定
- 新任务/交互：使用 `<button onclick="fn()">`，不要使用 web1 的 `data-action` 模式
