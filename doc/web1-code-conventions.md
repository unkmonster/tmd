# web1 前端代码规范速查

## 文件结构

```
internal/api/web/web1/
├── index.html      # 单页 HTML（~150行），所有 CSS/JS 通过内联 `<style>` 和 `<script src="/static/app.js">` 加载
├── styles.css      # 全部样式（~2260行），CSS 变量体系
└── app.js          # 全部 JS（~5100行），单文件无模块
```

## app.js 段落布局（按顺序）

| 段落 | 行号范围 | 内容 |
|---|---|---|
| Init guard | 1-4 | `_initComplete` |
| Utility Functions | 6-49 | `debounce`, `glowNewFirstItem`, `readListIDsFromTextarea`, `readTextareaLines` |
| Search Helpers | 51-71 | `updateSearchState`, `restoreSearchValue` |
| State Management | 73-191 | `store` 对象（state + setState + subscribe）|
| API Client | 201-438 | `api` 对象 |
| SSE Manager | 440-628 | `sseManager` 对象 |
| JWT Helpers | 630-646 | `tryRefreshJWT`, `appendJWTToken` |
| Toast | 648-692 | `toast` 对象 |
| Drawer | 694-720 | `drawer` 对象 |
| Page Renderers | 722-1009 | `pages` 对象（overview/tasks/data/schedules/system/logs）|
| Module State | 1011-1082 | `_state` 对象 + `makeChangeDetector` + task form 状态保存/恢复 |
| Helper Functions | 1084-1330 | `getStageText`, `getTaskTarget`, `renderTaskItem`, `renderTaskForm`, `renderCheckboxes` |
| DB Table Rendering | 1332-1581 | `renderTable`, 6个 `renderDB*Table`, `renderDBTable`, `renderDBMobileCards`, `renderPageNumbers` |
| DB Table Actions | 1583-1990 | `refreshDBData`, `changeDBPage`, `editDBItem`, `saveDBItem`, `deleteDBItem`, `dbGetFns/dbUpdateFns/dbDeleteFns` |
| Actions | 1992-2496 | `handleQuickDownload`, `create*Task`, `apiTask`, `getCheckedOptions`, `cancelTask`, `showTaskDetail` |
| Log/Config/Schedule Rendering | 2498-3870 | `escapeHtml`, `renderConfigForm`, `renderCookiesForm`, `renderScheduleForm`, load/save 函数 |
| Routing & Navigation | 3876-4515 | `navigateTo`, `parseRoute`, `render` |
| Auth Dialog | 4517-4667 | `showAuthDialog`, `hideAuthDialog`, `submitAuthKey` |
| State Sync | 4669-5040 | 3个 `makeChangeDetector` + `sync*Page` 函数 |
| Event Listeners | 5042-5176 | `data-action` dispatch + `contentContainer` 事件 + `init()` |
| Start | 5178-5180 | `init()` |

## 命名约定

- 模块级函数: `camelCase` — `createUserTask`, `renderTaskForm`
- 私有前导 `_`: `_initComplete`, `_scheduleReconnect`, `api._abortControllers`
- 对象方法: ES6 简写 `method() { ... }`，不是 `method: function()`
- data-action: 全小写 kebab-case — `closeDrawer`, `navigateTo`, `saveConfigForm`
- API 方法: 动词 + 资源 — `createUserDownload`, `getDBUsers`, `deleteDBList`

## 事件系统

### 统一 data-action dispatch

```js
document.addEventListener('click', (e) => {
  const el = e.target.closest('[data-action]');
  if (!el) return;
  const action = el.dataset.action;
  switch (action) {
    case 'navigateTo': navigateTo(el.dataset.page); break;
    case 'closeDrawer': drawer.close(); return;
    // ...
  }
});
```

所有交互按钮使用 `data-action` 属性，不要用 `onclick=`.
唯一保留 onclick 的例外:
- `toast-close` — 动态创建的元素
- `menuToggle` — toggle 逻辑，非简单 set
- `sseIndicator` — 多分支刷新

### DOM 事件监听分布

| 监听目标 | 用途 |
|---|---|
| `document` click | data-action 统一分发 |
| `#contentContainer` click | task-item 点击展示详情 |
| `#contentContainer` input/change/focusout/keydown | 表单双向绑定 |
| `window` resize | 响应式布局 |
| `window` unhandledrejection | 全局 Promise 错误兜底 |
| `#app` keydown | Enter 提交 auth dialog |

## 渲染模式

### 模板字面量

所有渲染函数返回 HTML 字符串：

```js
function renderXxx(data) {
  return `
    <div class="card">
      <div class="card-title">${escapeHtml(data.title)}</div>
      <div>${renderActionButtons(data)}</div>
    </div>
  `;
}
```

### XSS 安全 — 必须遵守

| 场景 | 函数 | 说明 |
|---|---|---|
| 内容插值 `${...}` | `escapeHtml(str)` | 所有用户数据必须包裹 |
| 属性插值 `value="..."` | `escapeAttr(str)` | 所有用户数据属性必须包裹 |
| 不要使用 | 裸 `${}` 在 innerHTML 中 | 除非值来自代码常量 |

### 空状态模板

```html
<div class="empty-state">
  <div class="empty-icon">📊</div>
  <div class="empty-title">暂无数据</div>
  <div class="empty-desc">数据库中还没有记录</div>
</div>
```

### Loading 模板

```html
<div class="empty-state">
  <div class="skeleton skeleton-icon"></div>
  <div class="empty-title">加载中...</div>
  <div class="empty-desc">正在加载...</div>
</div>
```

## 数据流

### 状态管理

```
store.setState({ key: value })  →  deepMerge → _scheduleNotify → listeners
```

- 读: `store.state.xxx`
- 写: `store.setState({ xxx: value })`
- 监听: `store.subscribe(fn)` 返回 unsubscribe 函数
- 3 个 `makeChangeDetector` 用于 data/schedule/overview 页面的增量更新

### DB 数据流

```
refreshDBData()
  → api.getDBXxx(params)
  → store.setState({ dbData, dbPagination })
  → syncDataPage (store.subscribe)
  → renderDBTable(type, data, sort)
    → renderTable(columns, data, sort)
```

### Task 创建数据流

```
handleQuickDownload / createUserTask
  → apiTask() wrapper (try/catch + toast)
    → api.createXxxDownload()
    → success → toast + clear input
    → error → toast(err.message, 'error')
```

## 错误处理

标准模式:

```js
try {
  await api.xxx();
  toast.show('成功消息');
} catch (err) {
  toast.show(err.message, 'error');
}
```

已提取 `apiTask()` 给简单 task 创建函数复用（返回 `true` 成功 / `undefined` 失败）。

## 已存在的复用模式

### renderTable(columns, data, sort)

列定义: `{ key, label, sortable, sortBy, render(item) }`

```js
return renderTable([
  { key: 'id', label: 'ID', sortable: true },
  { key: 'name', label: 'Name', sortable: true },
  { label: 'Actions', sortable: false, render: i => renderActionButtons(type, i) },
], data, sort);
```

### renderCheckboxes(prefix)

生成 4 个标准 checkbox（auto_follow / follow_members / skip_profile / no_retry）。

### getCheckedOptions(prefix)

读取 4 个 checkbox 的值，返回 `{ auto_follow, follow_members, skip_profile, no_retry }`。

### dbGetFns / dbUpdateFns / dbDeleteFns

类型→API 方法查表，集中定义在 `deleteDBItem` 上方：

```js
const dbGetFns = {
  users: id => api.getDBUser(id),
  lists: id => api.getDBList(id),
  // ...
};
```

### tryRefreshJWT(label, done) / appendJWTToken(params)

JWT 预刷新和 token 参数追加工具函数。

## CSS 变量体系

| 变量 | 用途 |
|---|---|
| `--bg-primary/secondary/tertiary` | 背景色 |
| `--text-primary/secondary/tertiary` | 文字色 |
| `--border-primary/secondary` | 边框色 |
| `--accent-primary/hover/active` | 主题色 |
| `--success/danger/warning/info` | 语义色 |
| `--success-bg/danger-bg/warning-bg/info-bg` | 语义背景色 |
| `--radius-md/lg` | 圆角 |
| `--space-[2-8]` | 间距 |
| `--duration-normal/fast` | 动画时长 |

状态标签使用预定义 CSS 类:

| 状态 | 类 |
|---|---|
| queued | `.tag-queued` / `.status-queued` |
| running | `.tag-running` / `.status-running` |
| completed | `.tag-completed` / `.status-completed` |
| failed | `.tag-failed` / `.status-failed` |
| cancelled | `.tag-cancelled` / `.status-cancelled` |

## 常见陷阱

1. **go build 必须通过**: 前端文件通过 `//go:embed` 打包进 Go 二进制，JS/CSS/HTML 语法错误会让 Go 编译失败
2. **不要写 `innerHTML +=`**: 会导致已有 DOM 事件丢失。用字符串拼接 + 一次 `innerHTML =`
3. **不要在渲染函数中直接引用 `document.getElementById`**: 应通过参数或 `store.state` 取值
4. **data-action 元素在 #app 外部**: 事件监听器已改为 `document` 级别，外部元素也能触发
5. **不要用 `classname`（小写 n）**: JS 中 className 用于 SVG，HTML 元素用 `classList` 或字符串 `class`
6. **所有 onerror/onclose 回调**: 需要在闭包外维护引用，避免闭包陷阱

## 修改前必读

- 新功能先检查 `pages` 对象中是否有对应的模板入口
- 新 API 端点先在 `api` 对象中添加方法
- 新交互按钮使用 `data-action` + 在 dispatch switch 中添加 case
- 新 task 类型需要: `api` 方法 → `createXxxTask` 函数 → `renderTaskForm` 模板 → dispatch case → 可选 `buildTaskRunFunc`（Go 后端）
- DB 表类型需要: `dbGetFns/dbUpdateFns/dbDeleteFns` 条目 → `renderTable` 列定义 → `renderDBMobileCards` renderer → `dataSubPageMap` 条目
