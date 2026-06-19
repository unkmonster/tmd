# `-mark-downloaded` 功能详解

> 本文档详细说明 Twitter Media Downloader (tmd) 中 `-mark-downloaded` 参数的实现原理、使用方法和内部机制。

---

## 目录

1. [功能概述](#功能概述)
2. [参数说明](#参数说明)
3. [核心原理](#核心原理)
4. [使用方法](#使用方法)
5. [实现细节](#实现细节)
6. [数据库结构](#数据库结构)
7. [时间过滤机制](#时间过滤机制)
8. [新用户处理流程](#新用户处理流程)
9. [错误处理](#错误处理)
10. [日志输出](#日志输出)
11. [常见场景](#常见场景)
12. [注意事项](#注意事项)

---

## 功能概述

`-mark-downloaded` 是一个标记功能，用于**在不下载任何内容的情况下**，更新数据库中用户的 `latest_release_time` 时间戳。这个时间戳决定了下次下载时从哪个时间点开始获取推文。

### 核心用途

| 用途 | 说明 |
|------|------|
| **跳过历史** | 标记为当前时间，下次只下载新推文 |
| **指定起点** | 标记为指定时间，从该时间点开始下载 |
| **重置记录** | 设置为 NULL，允许全量重新下载 |

---

## 参数说明

### 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-mark-downloaded` | bool | false | 启用标记模式，不下载内容 |
| `-mark-time` | string | "" | 时间戳值 |

### `-mark-time` 支持的值

| 值 | 效果 | 数据库操作 |
|----|------|-----------|
| **空（不提供）** | 使用当前时间 | `UPDATE ... SET latest_release_time=NOW()` |
| `"2024-01-01T00:00:00"` | 使用指定时间 | `UPDATE ... SET latest_release_time='2024-01-01 00:00:00'` |
| `"null"` 或 `"nil"` | 设置为 NULL | `UPDATE ... SET latest_release_time=NULL` |

### 时间格式

- 格式：`2006-01-02T15:04:05`
- 示例：`2024-06-15T10:30:00`
- 时区：使用本地时区解析

---

## 核心原理

### 工作流程图

```
┌─────────────────────────────────────────────────────────────┐
│   tmd -user xxx -mark-downloaded [-mark-time "xxx"]         │
│   CLI: internal/cli/executor.go                             │
│   API: POST /api/v1/.../mark                                │
└──────────────────────────┬──────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│  service.DownloadService.MarkDownloaded                      │
│  internal/service/download_service.go:552                   │
│    ├── resolveUsers / resolveLists / resolveFollowings       │
│    └── downloading.MarkUsersAsDownloaded()                   │
└──────────────────────────┬──────────────────────────────────┘
                           ↓
┌──────────────────────────────────────────────────────────────┐
│  downloading.MarkUsersAsDownloaded()                         │
│  internal/downloading/mark_downloaded.go:23                  │
│    ├── 解析 markTimeStr → timestamp/nil                      │
│    ├── 遍历 lists → lst.GetMembers() → markSingleUserWithInfo│
│    └── 遍历 users → markSingleUserWithInfo                   │
└──────────────────────────┬──────────────────────────────────┘
                           ↓
        ┌──────────────────┼──────────────────┐
        ↓                  ↓                  ↓
  syncUserAndEntity   SetLatestReleaseTime  ClearLatestReleaseTime
  (同步用户+实体)      (设置时间戳)           (清除时间戳=NULL)
```

### 与正常下载的关系

```
正常下载流程:
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│ 获取推文  │ → │ 更新时间戳 │ → │ 下载媒体  │ → │ 保存文件  │
└──────────┘    └──────────┘    └──────────┘    └──────────┘

-mark-downloaded 流程:
┌──────────┐    ┌──────────┐
│ 同步用户  │ → │ 更新时间戳 │  (跳过获取推文和下载)
└──────────┘    └──────────┘
```

---

## 使用方法

### 基础用法

```bash
# 标记单个用户为当前时间
tmd -user elonmusk -mark-downloaded

# 标记多个用户
tmd -user user1 -user user2 -user user3 -mark-downloaded

# 使用 @ 前缀
tmd -user @elonmusk -mark-downloaded
```

### 指定时间

```bash
# 从指定时间点开始下载
tmd -user elonmusk -mark-downloaded -mark-time "2024-06-01T00:00:00"

# 标记为一年前
tmd -user elonmusk -mark-downloaded -mark-time "2023-01-01T00:00:00"
```

### 重置为全量下载

```bash
# 清除时间记录，允许全量下载
tmd -user elonmusk -mark-downloaded -mark-time "null"

# "nil" 效果相同
tmd -user elonmusk -mark-downloaded -mark-time "nil"
```

### 批量操作

```bash
# 标记整个列表
tmd -list 123456789 -mark-downloaded

# 标记关注列表
tmd -foll myusername -mark-downloaded

# 混合操作
tmd -user user1 -list 123456 -foll myusername -mark-downloaded
```

### 不同终端的引号处理

```powershell
# PowerShell
tmd -user elonmusk -mark-downloaded -mark-time "null"
tmd -user elonmusk -mark-downloaded -mark-time "2024-01-01T00:00:00"

# CMD
tmd -user elonmusk -mark-downloaded -mark-time null
tmd -user elonmusk -mark-downloaded -mark-time 2024-01-01T00:00:00
```

### API 模式

```bash
# 标记某个用户
curl -X POST http://localhost:25556/api/v1/users/elonmusk/mark

# 标记某个列表
curl -X POST http://localhost:25556/api/v1/lists/123456789/mark

# 批量标记（带自定义时间）
curl -X POST http://localhost:25556/api/v1/batch/mark \
  -H "Content-Type: application/json" \
  -d '{"users":["elonmusk"],"lists":["123456"],"timestamp":"2024-06-01T00:00:00"}'
```

---

## 实现细节

### 整体调用链

```text
CLI 模式:
  internal/cli/executor.go
    → service.DownloadService.MarkDownloaded(ctx, "cli", screenNames, listIDs, followingNames, markTime, reporter)

API 模式:
  handleUserMark / handleListMark / handleFollowingMark / handleBatchMark
    → taskManager.CreateTask(TaskTypeMarkDownloaded, data)
    → downloadQueue.Enqueue(task, runFunc)
    → service.DownloadService.MarkDownloaded(ctx, taskID, ...)
```

### Service 层入口 (`internal/service/download_service.go:552`)

```go
func (s *downloadServiceImpl) MarkDownloaded(ctx context.Context, taskID string,
    screenNames []string, listIDs []uint64, followingNames []string,
    markTime *string, reporter ProgressReporter) error {

    reporter = s.getReporterOrDefault(reporter)
    reporter.OnProgress(taskID, Progress{Stage: "resolving"})

    // 1. 解析用户/列表/关注
    users := s.resolveUsers(ctx, screenNames)
    lists := s.resolveLists(ctx, listIDs)
    lists = append(lists, s.resolveFollowings(ctx, followingNames)...)

    if len(users) == 0 && len(lists) == 0 {
        return fmt.Errorf("no users or lists to mark (all failed to resolve)")
    }

    reporter.OnProgress(taskID, Progress{Stage: "marking",
        Total: len(users) + len(lists),
        Current: fmt.Sprintf("%d users, %d lists", len(users), len(lists))})

    // 2. 准备 markTime 字符串
    var markTimeStr string
    if markTime != nil {
        markTimeStr = *markTime
    }

    // 3. 委托 downloading 层执行
    pathHelper, _ := path.NewStorePath(s.deps.Config.RootPath)
    results, err := downloading.MarkUsersAsDownloaded(ctx, s.deps.Client,
        s.deps.DB, lists, users, pathHelper.Users, markTimeStr, s.maxFileNameLen())

    if err != nil {
        return err
    }
    reporter.OnComplete(taskID, Result{
        Message: fmt.Sprintf("Marked %d users as downloaded", len(results))})
    return nil
}
```

### 核心函数 (`internal/downloading/mark_downloaded.go:23`)

```go
func MarkUsersAsDownloaded(ctx context.Context, client *resty.Client,
    db *sqlx.DB, lists []twitter.ListBase, users []*twitter.User,
    dir string, markTimeStr string, maxLen int) ([]MarkedUserInfo, error) {

    // 1. 解析时间戳
    var timestamp *time.Time
    if markTimeStr == "" {
        now := time.Now()
        timestamp = &now
    } else if strings.ToLower(markTimeStr) == "null" ||
              strings.ToLower(markTimeStr) == "nil" {
        timestamp = nil
    } else {
        parsedTime, err := time.ParseInLocation("2006-01-02T15:04:05", markTimeStr, time.Local)
        if err != nil {
            return nil, fmt.Errorf("invalid mark-time format: %v", err)
        }
        timestamp = &parsedTime
    }

    // 2. 处理列表中的用户
    for _, lst := range lists {
        membersResult, err := lst.GetMembers(ctx, client)
        // ... 遍历 membersResult.Users ...
        for _, user := range membersResult.Users {
            info := markSingleUserWithInfo(db, user, dir, timestamp, maxLen)
            results = append(results, info)
        }
    }

    // 3. 处理直接指定的用户
    for _, user := range users {
        info := markSingleUserWithInfo(db, user, dir, timestamp, maxLen)
        results = append(results, info)
    }

    return results, nil
}
```

### 标记单个用户 (`internal/downloading/mark_downloaded.go:109`)

```go
func markSingleUserWithInfo(db *sqlx.DB, user *twitter.User,
    dir string, timestamp *time.Time, maxLen int) (info MarkedUserInfo) {

    if user == nil {
        info.Success = false
        info.Error = "user is nil"
        return info
    }

    defer func() {
        if r := recover(); r != nil {
            info.Success = false
            info.Error = fmt.Sprintf("panic: %v", r)
        }
    }()

    // 同步用户和实体（与正常下载使用相同的底层逻辑）
    entity, err := syncUserAndEntity(db, user, dir, maxLen)
    if err != nil {
        info.Error = fmt.Sprintf("failed to sync user and entity: %v", err)
        return info
    }

    // 设置 latest_release_time
    if timestamp == nil {
        if err := entity.ClearLatestReleaseTime(); err != nil {
            info.Error = fmt.Sprintf("failed to clear latest release time: %v", err)
            return info
        }
    } else {
        if err := entity.SetLatestReleaseTime(*timestamp); err != nil {
            info.Error = fmt.Sprintf("failed to set latest release time: %v", err)
            return info
        }
    }

    info.Success = true
    eid, err := entity.Id()
    if err != nil {
        info.Error = fmt.Sprintf("failed to get entity id: %v", err)
        return info
    }
    info.EntityID = eid
    return info
}
```

### 用户同步 (`internal/downloading/user_sync.go:11`)

```go
func syncUserAndEntity(db *sqlx.DB, user *twitter.User, dir string, maxLen int) (*entity.UserEntity, error) {
    // 1. 同步用户信息到 users 表
    if err := database.SyncUser(db, user.Id, user.Name, user.ScreenName,
        user.IsProtected, user.FriendsCount, true); err != nil {
        return nil, err
    }

    // 2. 生成期望的文件夹名（name(screen_name) 格式，按 maxLen 截断）
    userNaming := naming.NewUserNaming(user.Name, user.ScreenName, maxLen)
    expectedTitle := userNaming.SanitizedTitle()

    // 3. 创建或定位用户实体
    ent, err := entity.NewUserEntity(db, user.Id, dir)
    if err != nil {
        return nil, err
    }

    // 4. 同步实体路径（创建/重命名目录）
    if err = entity.Sync(ent, expectedTitle); err != nil {
        return nil, err
    }
    return ent, nil
}
```

### 用户名变更检测 — 数据库层 (`internal/database/user_sync.go:9`)

当前使用 UPSERT 实现幂等写入，用户名变更检测在 upsert 之前完成：

```go
func SyncUser(db *sqlx.DB, userId uint64, name string, screenName string,
    isProtected bool, friendsCount int, accessible bool) error {

    usrdb, err := GetUserById(db, userId)
    if err != nil {
        return err
    }

    renamed := false
    isNew := usrdb == nil
    if !isNew {
        renamed = usrdb.Name != name || usrdb.ScreenName != screenName
    }

    // 改名：在 UPSERT 之前记录旧名称
    if renamed {
        if err := RecordUserPreviousName(db, userId, usrdb.Name, usrdb.ScreenName); err != nil {
            return err
        }
    }

    // UPSERT：消除并发下的 TOCTOU 竞态
    stmt := `INSERT INTO users(id, screen_name, name, protected, friends_count, is_accessible)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            screen_name=excluded.screen_name, name=excluded.name,
            protected=excluded.protected, friends_count=excluded.friends_count,
            is_accessible=excluded.is_accessible`
    db.Exec(stmt, userId, screenName, name, isProtected, friendsCount, accessible)

    // 新用户：记录初始名称作为基线
    if isNew {
        RecordUserPreviousName(db, userId, name, screenName)
    }
    return nil
}
```

### 文件夹重命名逻辑 (`internal/entity/sync.go:11`)

```go
func Sync(e Entity, expectedName string) error {
    if !e.Recorded() {
        return e.Create(expectedName)   // 新用户：创建文件夹
    }

    name, err := e.Name()
    if err != nil {
        return err
    }
    if name != expectedName {
        return e.Rename(expectedName)   // 用户名变更：重命名文件夹
    }

    p, err := e.Path()
    if err != nil {
        return err
    }
    return os.MkdirAll(p, 0755)          // 名称一致：确保目录存在
}
```

### Entity 接口 (`internal/entity/interface.go`)

```go
type Entity interface {
    Path() (string, error)
    Create(name string) error
    Rename(string) error
    Remove() error
    Name() (string, error)
    Id() (int, error)
    Recorded() bool
}
```

### 用户实体重命名 (`internal/entity/user.go:63`)

```go
func (ue *UserEntity) Rename(title string) error {
    if !ue.created {
        return fmt.Errorf("user entity was not created")
    }
    old, _ := ue.Path()
    newPath := filepath.Join(filepath.Dir(old), title)

    // 使用系统调用重命名文件夹
    err := os.Rename(old, newPath)
    if os.IsNotExist(err) {
        // 边界情况：原文件夹被手动删除，创建新文件夹
        err = os.Mkdir(newPath, 0755)
    }
    if err != nil && !os.IsExist(err) {
        return err
    }

    // 更新数据库记录
    ue.record.Name = title
    return database.UpdateUserEntity(ue.db, ue.record)
}
```

---

## 数据库结构

### users 表

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER NOT NULL,              -- Twitter 用户 ID
    screen_name VARCHAR NOT NULL,     -- 用户名
    name VARCHAR NOT NULL,            -- 显示名称
    protected BOOLEAN NOT NULL,       -- 是否受保护
    friends_count INTEGER NOT NULL,   -- 关注数
    is_accessible BOOLEAN NOT NULL DEFAULT 1, -- 是否可访问
    PRIMARY KEY (id),
    UNIQUE (screen_name)
);
```

### user_entities 表

```sql
CREATE TABLE IF NOT EXISTS user_entities (
    id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    name VARCHAR NOT NULL,                  -- 文件夹名称
    latest_release_time DATETIME,           -- 最新推文时间（可为NULL）
    parent_dir VARCHAR COLLATE NOCASE NOT NULL,
    media_count INTEGER,
    PRIMARY KEY (id),
    UNIQUE (user_id, parent_dir),
    FOREIGN KEY(user_id) REFERENCES users (id)
);
```

### 数据库操作函数 (`internal/database/user_entity.go`)

```go
// 设置时间戳
func SetUserEntityLatestReleaseTime(db *sqlx.DB, id int, t time.Time) error {
    stmt := `UPDATE user_entities SET latest_release_time=? WHERE id=?`
    _, err := db.Exec(stmt, t, id)
    return err
}

// 清除时间戳（设置为NULL）
func ClearUserEntityLatestReleaseTime(db *sqlx.DB, id int) error {
    stmt := `UPDATE user_entities SET latest_release_time=NULL WHERE id=?`
    _, err := db.Exec(stmt, id)
    return err
}
```

### 标记返回结构

```go
type MarkedUserInfo struct {
    UserID     uint64 `json:"user_id"`
    ScreenName string `json:"screen_name"`
    EntityID   int    `json:"entity_id"`
    Success    bool   `json:"success"`
    Error      string `json:"error,omitempty"`
}
```

---

## 时间过滤机制

### 下载时的时间过滤 — 增量下载原理

每次下载用户时，`BatchDownload` 从 `user_entities.latest_release_time` 读取时间戳，作为 `TimeRange.Min` 传入 `GetMedias()`，实现增量下载：

```go
// batch_download.go — 简化示意
minTime, err := ent.LatestReleaseTime()
// ...
tweets, err := user.GetMedias(ctx, cli, &utils.TimeRange{Min: minTime})
```

### 关键行为

| `latest_release_time` 值 | 过滤行为 |
|-------------------------|----------|
| NULL | 不过滤，获取全部历史推文 |
| 零值 `time.Time{}` | 不参加过滤，获取全部 |
| 有效时间 | 只获取该时间点**之后**的推文 |

---

## 新用户处理流程

### 流程图

```
用户不存在于数据库
        ↓
    syncUserAndEntity(db, user, dir, maxLen)
        ↓
    ├── database.SyncUser(db, ...)
    │       ↓
    │   GetUserById() 返回 nil → isNew = true
    │       ↓
    │   UPSERT 写入 users 表
    │       ↓
    │   RecordUserPreviousName() 记录初始名称
    │
    ├── naming.NewUserNaming(name, screenName, maxLen)
    │       ↓
    │   expectedTitle = "DisplayName(screen_name)"
    │
    ├── entity.NewUserEntity(db, userId, dir)
    │       ↓
    │   LocateUserEntity() 返回 nil
    │       ↓
    │   创建 UserEntity 记录（内存），created = false
    │
    └── entity.Sync(ent, expectedTitle)
            ↓
        ent.Recorded() 返回 false
            ↓
        ent.Create(expectedTitle)
            ↓
        os.MkdirAll() 创建文件夹
            ↓
        database.CreateUserEntity() INSERT
            ↓
        created = true
```

---

## 错误处理

### 防御性检查

```go
// 1. nil 用户检查
if user == nil {
    info.Success = false
    info.Error = "user is nil"
    return info
}

// 2. panic 恢复
defer func() {
    if r := recover(); r != nil {
        info.Success = false
        info.Error = fmt.Sprintf("panic: %v", r)
    }
}()

// 3. 同步失败处理
entity, err := syncUserAndEntity(db, user, dir, maxLen)
if err != nil {
    info.Error = fmt.Sprintf("failed to sync user and entity: %v", err)
    return info
}

// 4. 时间戳设置失败处理
if err := entity.SetLatestReleaseTime(*timestamp); err != nil {
    info.Error = fmt.Sprintf("failed to set latest release time: %v", err)
    return info
}
```

### 列表访问错误

```go
membersResult, err := lst.GetMembers(ctx, client)
if err != nil {
    if strings.Contains(err.Error(), "does not exist or is not accessible") {
        return nil, fmt.Errorf("list %s does not exist or is not accessible", lst.Title())
    }
    log.Warnln("✗", lst.Title(), "-", "failed to get list members:", err)
    continue  // 继续处理其他列表
}
```

---

## 日志输出

CLI 模式下，使用 `LogReporter` 输出日志（不输出 `=== MARK_DOWNLOADED_RESULTS ===` 格式）：

```
INFO[0000] marking users as downloaded, timestamp: 2024-06-15T10:30:00+08:00
INFO[0001] ✓ Elon Musk(elonmusk) - marked as downloaded
INFO[0001] finished marking users as downloaded, success: 3 failed: 0
```

---

## 常见场景

### 场景1：首次下载后跳过历史

```bash
# 首次下载
tmd -user elonmusk

# 以后只想下载新推文，跳过历史
tmd -user elonmusk -mark-downloaded
```

### 场景2：重新下载特定时间段

```bash
# 重新下载 2024 年的推文
tmd -user elonmusk -mark-downloaded -mark-time "2024-01-01T00:00:00"
```

### 场景3：完全重新下载

```bash
# 清除记录，全量下载
tmd -user elonmusk -mark-downloaded -mark-time "null"
tmd -user elonmusk
```

### 场景4：批量管理列表

```bash
# 标记整个列表为已下载
tmd -list 123456789 -mark-downloaded

# 重置整个列表
tmd -list 123456789 -mark-downloaded -mark-time "null"
```

### 场景5：新用户预处理

```bash
# 添加新用户但不下载历史
tmd -user newuser123 -mark-downloaded
# 以后只下载新推文
```

---

## 注意事项

### 1. 不下载任何内容

`-mark-downloaded` **只更新数据库**，不会：
- 获取推文
- 下载媒体文件
- 创建 .loongtweet 文件

### 2. 幂等性

可以重复执行，每次都会覆盖 `latest_release_time`：
```bash
tmd -user elonmusk -mark-downloaded -mark-time "2024-01-01T00:00:00"
tmd -user elonmusk -mark-downloaded -mark-time "2024-06-01T00:00:00"  # 覆盖
```

### 3. 参数优先级

`-mark-downloaded` 在 CLI 参数中具有**第三高优先级**：
1. `-jsonfile`（独占）
2. `-jsonfolder`（独占）
3. **`-mark-downloaded`（独占）**
4. `-user/-list/-foll`（可组合，可与 -profile 组合）
5. `-profile-user/-profile-list`（可组合）

当 `-mark-downloaded` 生效时，同一命令行中的其他参数会被忽略。

### 4. CLI vs API 差异

| 方面 | CLI 模式 | API 模式 |
|------|----------|----------|
| 入口 | `internal/cli/executor.go` | `internal/api/download_handlers.go` |
| 执行 | 同步等待 | 异步任务（返回 task_id） |
| 进度 | `LogReporter` | `SSEProgressReporter` |
| 结果 | 日志输出 | 通过 SSE 推送到前端 |
| 任务ID | `"cli"` | `task_<uuid>` |

### 5. MongoDB 用户 ID 格式

`-user` 参数接受 Twitter **screen_name**（例如 `elonmusk`），不是数字 user_id。

### 6. 时间格式严格

格式必须为 `2006-01-02T15:04:05`：
```bash
# ✅ 正确
tmd -user elonmusk -mark-downloaded -mark-time "2024-06-15T10:30:00"

# ❌ 错误
tmd -user elonmusk -mark-downloaded -mark-time "2024-06-15"
tmd -user elonmusk -mark-downloaded -mark-time "2024/06/15 10:30:00"
```

### 7. 数据库文件位置

| 系统 | 路径 |
|------|------|
| Windows | `{存储目录}\.data\foo.db` |
| macOS/Linux | `{存储目录}/.data/foo.db` |

---

## 附录：相关源码文件

| 文件 | 说明 |
|------|------|
| `internal/cli/args.go` | CLI 参数解析（`-mark-downloaded` / `-mark-time`） |
| `internal/cli/executor.go` | CLI 模式执行入口，调用 Service 层 |
| `internal/service/download_service.go` | `DownloadService.MarkDownloaded` 实现 |
| `internal/service/interfaces.go` | `DownloadService` 接口定义 |
| `internal/downloading/mark_downloaded.go` | `MarkUsersAsDownloaded` / `markSingleUserWithInfo` 核心实现 |
| `internal/downloading/user_sync.go` | `syncUserAndEntity` 用户同步 |
| `internal/database/user_sync.go` | `SyncUser` 用户信息同步（UPSERT） |
| `internal/entity/user.go` | `SetLatestReleaseTime` / `ClearLatestReleaseTime` |
| `internal/entity/sync.go` | `Sync` 实体路径同步函数 |
| `internal/entity/interface.go` | `Entity` 接口定义 |
| `internal/api/download_handlers.go` | API 模式的标记处理器 |
| `internal/api/types.go` | API 任务数据结构 |
| `internal/naming/user_naming.go` | `UserNaming` 文件夹名生成 |

---

*文档更新日期：2026-06-04*
