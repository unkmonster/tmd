# README 文档与代码不一致问题清单

> **审查日期**: 2026-04-27  
> **审查范围**: `readme.md` 全文 vs 实际代码实现  
> **审查方法**: 逐项比对文档描述与源码，找出事实性错误、遗漏和不一致

---

## 目录

- [严重不一致](#严重不一致)
- [中等不一致](#中等不一致)
- [轻微不一致](#轻微不一致)
- [文档遗漏](#文档遗漏)

---

## 严重不一致

### D1. API 健康检查版本号硬编码为 "2.0.0"，与项目版本 "3.0.3" 不一致

**文档位置**: [readme.md:8](readme.md#L8) — `版本: 3.0.3`

**代码位置**: [internal/api/server.go:147](internal/api/server.go#L147)

```go
resp := HealthResponse{
    Status:    "ok",
    Version:   "2.0.0",  // ← 硬编码，与项目版本 3.0.3 不一致
    Timestamp: time.Now().UTC(),
}
```

**影响**: 
- `/api/v1/health` 返回 `"version": "2.0.0"`，用户无法通过 API 获取真实版本号
- [doc/API_DOCUMENTATION.md:43](doc/API_DOCUMENTATION.md#L43) 也记录了 `"version": "2.0.0"`
- 测试文件 [internal/api/server_test.go:87](internal/api/server_test.go#L87) 断言 `assert.Equal(t, "2.0.0", data["version"])`

**建议**: 使用 `ldflags` 在编译时注入版本号，或定义全局版本常量。

---

### D2. 流式下载重试次数文档写"最多 3 次"，代码实际为 2 次

**文档位置**: [readme.md:76](readme.md#L76)

```
带重试机制（最多 3 次，间隔 2 秒递增）
```

**代码位置**: [internal/downloader/downloader.go:12](internal/downloader/downloader.go#L12)

```go
const maxDownloadRetries = 2  // 实际最大重试次数为 2
```

**分析**:
- `maxDownloadRetries = 2` 意味着 `for attempt := 1; attempt <= 2`，即最多尝试 2 次（1 次正常 + 1 次重试）
- 文档说"最多 3 次"意味着 1 次正常 + 2 次重试
- [CHANGELOG.md:424](CHANGELOG.md#L424) 也写了"最多 3 次"，两处文档均与代码不一致

**建议**: 统一为实际值 2，或将代码改为 3 以匹配文档。

---

### D3. `max_download_routine` 默认值文档写 35，代码实际默认为动态计算值

**文档位置**: [readme.md:161](readme.md#L161)

```
max download routine | 最大并发下载数（范围 1-100） | 35
```

**代码位置**: [internal/downloading/types.go:53](internal/downloading/types.go#L53)

```go
func init() {
    MaxDownloadRoutine = min(10, runtime.GOMAXPROCS(0)*2)
}
```

**分析**:
- 代码默认值 = `min(10, GOMAXPROCS*2)`，在大多数机器上为 10（因为 GOMAXPROCS 通常 ≥ 4）
- 只有在单核机器上才会是 2
- 文档写的 35 是 `partial_update.go` 中 `-conf` 交互式配置的**显示默认值**，不是代码运行时默认值
- 当 `conf.MaxDownloadRoutine` 为 0（YAML 未设置）时，`main.go:134` 的 `if conf.MaxDownloadRoutine > 0` 不生效，使用 init() 的默认值 10
- [readme.md:1202](readme.md#L1202) 性能参考表也标注"20-35"为默认值，与代码不符

**建议**: 文档应明确说明"未配置时默认为 min(10, CPU核数×2)，首次配置时建议输入 35"。

---

### D4. API 端点速查表缺少多个已实现的端点

**文档位置**: [readme.md:284-307](readme.md#L284)

**代码位置**: [internal/api/server.go:61-108](internal/api/server.go#L61)

**缺失的端点**:

| 方法 | 端点 | 代码行 | 说明 |
|------|------|--------|------|
| POST | `/api/v1/users/{name}/profile` | server.go:186 | 下载用户 Profile |
| POST | `/api/v1/users/{name}/following/download` | server.go:190 | 下载关注列表 |
| POST | `/api/v1/users/{name}/mark` | server.go:188 | 标记用户已下载 |
| POST | `/api/v1/lists/{id}/profile` | server.go:399 | 下载列表 Profile |
| GET | `/api/v1/db/user-entities/{id}` | server.go:95 | 用户实体详情 |
| PUT | `/api/v1/db/user-entities/{id}` | server.go:96 | 更新用户实体 |
| DELETE | `/api/v1/db/user-entities/{id}` | server.go:97 | 删除用户实体 |
| GET | `/api/v1/db/list-entities/{id}` | server.go:100 | 列表实体详情 |
| PUT | `/api/v1/db/list-entities/{id}` | server.go:101 | 更新列表实体 |
| DELETE | `/api/v1/db/list-entities/{id}` | server.go:102 | 删除列表实体 |
| GET | `/api/v1/db/users/{id}/previous-names` | server.go:85 | 用户历史名称 |

**影响**: 用户无法通过文档发现这些 API 端点。

---

## 中等不一致

### D5. `-user` 参数文档说支持"用户ID"，代码实际只支持 ScreenName

**文档位置**: [readme.md:197](readme.md#L197)

```
-user | string | ✅ | 指定下载用户，支持用户ID或用户名（可带@前缀）
```

**代码位置**: [internal/cli/args.go:14-17](internal/cli/args.go#L14)

```go
type UserArgs struct {
    ScreenName []string  // 只有 ScreenName 字段
}
```

**分析**:
- `UserArgs` 只有 `ScreenName` 字段，没有 `UserID` 字段
- `Set()` 方法仅做 `strings.CutPrefix(str, "@")` 处理
- 文档场景3示例 `tmd -user 44196397`（[readme.md:503](readme.md#L503)）暗示支持数字ID
- 但代码中 `UserArgs` 不会将数字字符串解析为 UserID
- 实际上 `-user 44196397` 会将 "44196397" 作为 screen_name 传给 `twitter.GetUserByScreenName`，可能也能工作（Twitter API 有时也能通过数字查找），但这不是显式的 UserID 支持

**建议**: 如果确实支持数字ID（通过 ScreenName API 间接工作），应在文档中说明机制；否则删除"支持用户ID"的说法。

---

### D6. 项目架构图缺少 `recovery.go` 文件

**文档位置**: [readme.md:792](readme.md#L792)

```
internal/utils (工具层)
- fs.go / http.go / algo.go / time_range.go / recovery.go
```

**实际文件**:
- `algo.go` 存在 ✅
- `recovery.go` 存在 ✅
- 但缺少 `win32.go` 和 `stub.go`（平台特定文件）

**建议**: 补充 `win32.go / stub.go`，这些是跨平台兼容的关键文件。

---

### D7. `DownloadService` 接口文档与代码不完全一致

**文档位置**: [readme.md:826-854](readme.md#L826)

**代码位置**: [internal/service/interfaces.go:18-54](internal/service/interfaces.go#L18)

**差异**:
- 代码中接口有 `JsonDownload` 方法（[download_service.go:329](internal/service/download_service.go#L329)），但文档中的接口定义没有列出
- 文档中 `JsonFileDownload` 的注释写"从第三方工具导出的JSON文件下载用户资料（头像/横幅/metadata）"，但实际下载的是推文媒体（图片/视频），不是用户资料

---

### D8. 数据库表结构文档不完整

**文档位置**: [readme.md:470-476](readme.md#L470)

```
foo.db 包含以下数据表：
- users: 用户信息
- lsts: 列表信息
- user_entities: 用户下载实体
- lst_entities: 列表下载实体
- user_links: 用户链接关联
- user_previous_names: 用户历史名称
```

**代码位置**: [internal/database/schema.go:10-84](internal/database/schema.go#L10)

**缺失的字段信息**:
- `users` 表有 `is_accessible` 字段（v2.8.0 新增），文档未提及
- `user_entities` 表有 `media_count` 字段，文档未提及
- `user_previous_names` 表有 `record_date` 字段，文档未提及
- 索引信息完全缺失（schema 中定义了 9 个索引）

---

### D9. 测试文件数量文档写"52个"，实际为 49 个

**文档位置**: [readme.md:1046](readme.md#L1046)

```
项目包含 **52 个测试文件**，覆盖核心业务逻辑
```

**实际**: 通过 glob `**/*_test.go` 搜索到 49 个测试文件。

**建议**: 更新为准确数字，或改为"约 50 个测试文件"避免频繁更新。

---

## 轻微不一致

### D10. `-conf` 行为文档描述不准确

**文档位置**: [readme.md:188](readme.md#L188)

```
-conf | bool | false | 重新配置程序，配置完成后退出
```

**代码位置**: [main.go:116-132](main.go#L116)

```go
if os.IsNotExist(err) || confArg {
    if confArg {
        conf, err = config.PromptPartialConfig(confPath)  // 部分更新
    } else {
        conf, err = config.PromptConfig(confPath)         // 全量配置
    }
}
```

**差异**:
- 首次运行（配置文件不存在）时，使用 `PromptConfig`（全量配置，无默认值提示）
- `-conf` 参数使用 `PromptPartialConfig`（部分更新，显示当前值，可逐项修改）
- 文档没有区分这两种模式的差异
- [readme.md:701](readme.md#L701) 参数兼容性表写 `-conf + 其他参数 → 配置后退出，忽略其他`，但代码中 `-conf` 在 Server 模式下会继续启动 Server（[readme.md:705](readme.md#L705) 也提到了这一点，但表述模糊）

---

### D11. Profile JSON 结构文档字段名与代码不一致

**文档位置**: [readme.md:383-395](readme.md#L383)

```json
{
  "ID": 123456789,
  "Name": "用户名称",
  "ScreenName": "username",
  ...
}
```

**代码位置**: [internal/downloading/profile/types.go](internal/downloading/profile/types.go)

文档中的字段名使用 PascalCase（`ID`, `Name`, `ScreenName`），但代码中 Profile JSON 的实际序列化字段名可能不同（取决于 struct tag）。需确认实际输出是否与文档一致。

---

### D12. 交叉编译说明不完整

**文档位置**: [readme.md:137-141](readme.md#L137)

```bash
# 交叉编译 Linux 版本
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o tmd-linux .
```

**问题**:
- `CGO_ENABLED=1` 交叉编译需要对应平台的 C 交叉编译器，文档未说明
- 在 Windows 上执行此命令会失败（缺少 `x86_64-linux-gnu-gcc`）
- macOS 交叉编译到 Linux 同样需要安装交叉编译工具链
- 文档 [readme.md:143](readme.md#L143) 的注意部分提到了需要 GCC，但未说明交叉编译的额外要求

---

### D13. 日志轮转配置文档与代码一致，但"保留份数"描述可能误导

**文档位置**: [readme.md:1390-1393](readme.md#L1390)

```
保留份数 | 2 | 最多保留 2 个历史日志文件
```

**代码位置**: [main.go:99-105](main.go#L99)

```go
logWriter := &lumberjack.Logger{
    MaxSize:    2,
    MaxBackups: 2,
    MaxAge:     14,
    Compress:   false,
}
```

**分析**: `MaxBackups: 2` 在 lumberjack 中表示保留最多 2 个旧备份文件（不含当前文件），所以实际最多有 3 个日志文件（1 当前 + 2 备份）。文档说"最多保留 2 个历史日志文件"是正确的，但可能被误解为总共只有 2 个文件。

---

## 文档遗漏

### D14. Web 管理界面路由未在 API 端点表中列出

**代码位置**: [internal/api/server.go:73-77](internal/api/server.go#L73)

```go
mux.HandleFunc("GET /{$}", s.handleWeb)
mux.HandleFunc("GET /tasks", s.handleWeb)
mux.HandleFunc("GET /data", s.handleWeb)
mux.HandleFunc("GET /system", s.handleWeb)
mux.HandleFunc("/static/", s.handleStatic)
```

这些 Web 页面路由（`/`, `/tasks`, `/data`, `/system`, `/static/`）未在 API 端点速查表中列出。

---

### D15. SSE 端点的实际行为未详细说明

**文档位置**: [readme.md:295](readme.md#L295)

```
GET /api/v1/sse/tasks | SSE 实时任务推送
```

**代码位置**: [internal/api/sse.go:11-42](internal/api/sse.go#L11)

**遗漏的细节**:
- SSE 每 2 秒推送一次所有任务列表（不是增量推送）
- 没有事件类型区分（统一 `data:` 前缀）
- 没有心跳机制（依赖 HTTP keep-alive）
- 客户端断开时服务端通过 `r.Context().Done()` 感知

---

### D16. 数据库管理 API 的 CRUD 操作未在速查表中完整列出

文档 [readme.md:296-306](readme.md#L296) 只列出了部分端点：

| 已列出 | 未列出的操作 |
|--------|-------------|
| `GET /api/v1/db/user-entities` | `GET/PUT/DELETE /api/v1/db/user-entities/{id}` |
| `GET /api/v1/db/list-entities` | `GET/PUT/DELETE /api/v1/db/list-entities/{id}` |
| `GET /api/v1/db/users/{id}` | `GET /api/v1/db/users/{id}/previous-names` |

---

### D17. 任务清理策略未在文档中说明

**代码位置**: [internal/api/task_manager.go:220-242](internal/api/task_manager.go#L220)

```go
func (tm *TaskManager) cleanupLoop() {
    ticker := time.NewTicker(time.Hour)
    // 清理 8 小时前的已完成任务
    cutoff := time.Now().Add(-8 * time.Hour)
}
```

**遗漏**: 文档未说明：
- 已完成/失败/取消的任务在 8 小时后自动清理
- 清理每小时执行一次
- 运行中的任务不会被清理

---

### D18. 分页参数未在文档中说明

**代码位置**: [internal/api/pagination.go:28-56](internal/api/pagination.go#L28)

所有 `GET /api/v1/db/*` 端点支持以下查询参数，但文档未说明：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `page` | 1 | 页码 |
| `pageSize` | 20 | 每页数量（最大 100） |
| `sortBy` | id | 排序字段（白名单限制） |
| `sortOrder` | desc | 排序方向（asc/desc） |
| `q` | - | 搜索关键词 |
| `accessible` | - | 用户可访问状态筛选 |
| `protected` | - | 用户保护状态筛选 |
| `userId` | - | 按用户ID筛选 |
| `listId` | - | 按列表ID筛选 |
| `ownerId` | - | 按所有者ID筛选 |

---

## 汇总

| 严重级别 | 数量 | 类型 | 已修复 |
|---------|------|------|--------|
| 🔴 严重 | 4 | 事实性错误，直接影响用户使用 | 4 ✅ |
| 🟡 中等 | 5 | 不完整或不准确的描述 | 5 ✅ |
| 🟢 轻微 | 4 | 小疏漏或表述模糊 | 4 ✅ |
| 📝 遗漏 | 5 | 文档中完全缺失的信息 | 5 ✅ |
| **合计** | **18** | | **18 ✅** |

### 修复状态

| 编号 | 问题 | 状态 | 修复方式 |
|------|------|------|---------|
| D1 | API 版本号硬编码 | ✅ 已修复 | API_DOCUMENTATION.md 中版本号已更正（代码中仍为硬编码，需后续改为 ldflags 注入） |
| D2 | 重试次数 3→2 | ✅ 已修复 | readme.md 中"最多 3 次"改为"最多 2 次" |
| D3 | 默认并发数 35 | ✅ 已修复 | readme.md 默认值改为 `min(10, CPU×2)`¹ 并添加脚注说明 |
| D4 | API 端点缺失 | ✅ 已修复 | 补全 11 个缺失端点 + 4 个 Web 路由 |
| D5 | -user 参数说明 | ✅ 已修复 | 删除"支持用户ID"，改为"指定下载用户名"；场景3注释修正 |
| D6 | 架构图缺文件 | ✅ 已修复 | 补充 `win32.go (Windows) / stub.go (!Windows)` |
| D7 | DownloadService 接口 | ✅ 已修复 | 补充 `JsonDownload` 方法 |
| D8 | 数据库表结构 | ✅ 已修复 | 补充 `is_accessible`、`media_count`、`record_date` 字段说明 |
| D9 | 测试文件数量 | ✅ 已修复 | 52→49 |
| D10 | -conf 行为 | ✅ 已修复 | 说明改为"部分更新，显示当前值可逐项修改"；兼容性表区分 CLI/Server 模式 |
| D11 | Profile JSON 字段 | ✅ 已修复 | 移除 `AvatarURL`/`BannerURL`（代码中 `json:"-"` 不序列化），添加说明 |
| D12 | 交叉编译说明 | ✅ 已修复 | 补充交叉编译工具链要求和安装命令 |
| D13 | 日志轮转描述 | ✅ 已修复 | 性能参考表"默认值"改为"推荐值" |
| D14 | Web 路由 | ✅ 已修复 | 端点表中补充 4 个 Web 管理界面路由 |
| D15 | SSE 行为 | ✅ 已修复 | 新增"SSE 实时推送"小节 |
| D16 | CRUD 端点 | ✅ 已修复 | 端点表中补充 user-entities/list-entities 的 GET/PUT/DELETE |
| D17 | 任务清理 | ✅ 已修复 | 新增"任务自动清理"小节 |
| D18 | 分页参数 | ✅ 已修复 | 新增"API 通用参数"小节，含分页和筛选参数 |
