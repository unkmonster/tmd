# TMD 版本对比分析报告

## 概述

本报告对比分析 Twitter Media Downloader (TMD) 的两个版本：
- **旧版本**: `tmd-2.4.4` (历史版本)
- **当前版本**: 基于 `unkmonster/tmd` 的修改版本（当前 v3.4.19）

---

## 一、项目结构对比

### 1.1 目录结构变化

| 层级 | 旧版本 (2.4.4) | 当前版本 |
|------|----------------|----------|
| 应用层 | `main.go` | `main.go` |
| 配置层 | 内嵌在 `main.go` | `internal/config/config.go` (独立包) |
| API客户端层 | `internal/twitter/` | `internal/twitter/` |
| 数据持久化层 | `internal/database/` | `internal/database/` (扩展) |
| 业务层-推文下载 | `internal/downloading/` | `internal/downloading/` (扩展) |
| 业务层-用户资料 | ❌ 无 | `internal/downloading/profile/` (新增) |
| 基础设施层 | ❌ 无 | `internal/downloader/` (新增) |
| 命名服务 | ❌ 无 | `internal/naming/` (新增) |
| 实体层 | ❌ 无 | `internal/entity/` (新增) |
| 工具层 | `internal/utils/` | `internal/utils/` (扩展) |

### 1.2 代码文件数量对比

```
旧版本 (2.4.4):
- main.go
- internal/
  - database/
    - crud.go, db_test.go, model.go
  - downloading/
    - download_test.go, dumper.go, entity.go, features.go
  - twitter/
    - api.go, client.go, errors.go, list.go, timeline.go, tweet.go, twitter_test.go, user.go
  - utils/
    - algo.go, fs.go, http.go, stub.go, time_range.go, utils_test.go, win32.go

当前版本:
- main.go
- internal/
  - config/ (新增)
    - config.go
  - database/ (扩展, crud.go 已拆分为以下文件)
    - connect.go, helpers.go, lst.go, lst_entity.go, model.go, schema.go, user.go, user_entity.go, user_link.go, user_sync.go, user_sync_test.go, db_test.go
  - downloader/ (新增)
    - downloader.go, file_writer.go, helpers.go, types.go, version_manager.go, downloader_test.go
  - downloading/ (扩展)
    - batch_any.go, batch_download.go, download_test.go, dumper.go, entity.go, json_download.go, list_download.go, list_sync.go, mark_downloaded.go, retry.go, tweet_download.go, types.go, user_sync.go
  - entity/ (新增)
    - interface.go, list.go, sync.go, user.go
  - naming/ (新增)
    - base.go, list_naming.go, tweet_naming.go, user_naming.go, naming_test.go
  - profile/ (新增)
    - downloader.go, fetcher.go, storage.go, types.go, fetcher_test.go
  - twitter/ (扩展)
    - api.go, batch_login.go, client.go, errors.go, list.go, timeline.go, tweet.go, twitter_test.go, user.go
  - utils/ (扩展)
    - algo.go, fs.go, http.go, recovery.go, stub.go, time_range.go, user.go, utils_test.go, win32.go
```

---

## 二、架构设计对比

### 2.1 旧版本架构 (2.4.4)

```
┌─────────────────────────────────────────┐
│  main.go (应用层 + 配置层混合)            │
└──────────┬──────────────────────────────┘
           │
┌──────────▼──────────────────────────────┐
│  internal/twitter (API 客户端层)         │
│  - api.go, client.go, user.go, tweet.go │
└──────────┬──────────────────────────────┘
           │
┌──────────▼──────────────────────────────┐
│  internal/downloading (业务层)           │
│  - features.go (核心下载逻辑)            │
│  - dumper.go (失败推文持久化)            │
│  - entity.go (UserEntity/ListEntity)     │
└──────────┬──────────────────────────────┘
           │
┌──────────▼──────────────────────────────┐
│  internal/database (数据层)              │
│  - crud.go, model.go                     │
└─────────────────────────────────────────┘
```

**特点**: 扁平化架构，职责混合，业务逻辑集中在 `features.go` 中

### 2.2 当前版本架构

```
┌─────────────────────────────────────────────────────────────┐
│  main.go (应用层)                                            │
│  - 命令行解析、依赖注入、流程编排                              │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────────┐
│  internal/config (配置层)                                    │
│  - config.go: 配置结构、读写、Cookie 管理、附加 Cookie 加载   │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────┐  ┌────────────────────────────▼┐
│  internal/twitter           │  │  internal/database          │
│  (API 客户端层)              │  │  (数据持久化层)               │
└──────────┬──────────────────┘  └─────────────┬───────────────┘
           │                                    │
┌──────────▼────────────────────────────────────┴─────────────┐
│  internal/downloading (业务层 - 推文下载)                    │
│  internal/downloading/profile (业务层 - 用户资料)                        │
└──────────┬──────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────────┐
│  internal/downloader (基础设施层 - 通用下载)                 │
│  internal/naming (命名服务)                                  │
│  internal/entity (数据实体层)                                │
└─────────────────────────────────────────────────────────────┘
```

**特点**: 分层解耦，职责清晰，接口隔离

---

## 三、核心功能对比

### 3.1 功能特性对比表

| 功能 | 旧版本 (2.4.4) | 当前版本 | 说明 |
|------|----------------|----------|------|
| 推文媒体下载 | ✅ | ✅ | 基础功能 |
| 列表批量下载 | ✅ | ✅ | 基础功能 |
| 关注列表下载 | ✅ | ✅ | 基础功能 |
| 多账号支持 | ✅ | ✅ | 附加 Cookie |
| 速率限制处理 | ✅ | ✅ | 改进实现 |
| 自动关注 | ✅ | ✅ | 基础功能 |
| **Profile 下载** | ❌ | ✅ | 新增功能 |
| **推文 JSON 保存** | ❌ | ✅ | 新增功能 |
| **JSON 文件导入** | ❌ | ✅ | 新增功能 |
| **标记已下载** | ❌ | ✅ | 新增功能 |
| **版本管理** | ❌ | ✅ | 新增功能 |
| **原子文件写入** | ❌ | ✅ | 新增功能 |
| **MD5 去重** | ❌ | ✅ | 新增功能 |

### 3.2 新增功能详解

#### 3.2.1 Profile 下载功能

**旧版本**: 无此功能

**当前版本**: 
```go
// internal/downloading/profile/downloader.go
func (pd *ProfileDownloader) DownloadMultiple(ctx context.Context, requests []DownloadRequest) []DownloadResult
```

支持下载：
- 高清头像 (400x400)
- 个人主页横幅
- 用户简介
- 完整资料信息 (JSON)
- 版本管理（资料变更时自动备份旧版本）

#### 3.2.2 推文 JSON 保存

**旧版本**: 无此功能

**当前版本**:
```go
// internal/downloading/tweet_download.go
func saveTweetJson(cfg *workerConfig, dir string, tweet *twitter.Tweet, namingObj *naming.TweetNaming)
func saveLoongTweet(cfg *workerConfig, dir string, tweet *twitter.Tweet, namingObj *naming.TweetNaming)
```

保存内容：
- `{tweet_id}.json`: 格式化 JSON
- `{tweet_id}.txt`: 人类可读文本

#### 3.2.3 JSON 文件导入

**旧版本**: 无此功能

**当前版本**:
```go
// internal/downloading/json_download.go
func DownloadJsonDir(ctx context.Context, client *resty.Client, root string, dwn downloader.Downloader, fileWriter downloader.FileWriter, paths ...string) []JsonDownloadResult
```

支持从其他工具导出的 JSON 批量下载媒体

---

## 四、代码实现对比

### 4.1 媒体下载逻辑对比

#### 旧版本 (features.go:L46-84)

```go
// 任何一个 url 下载失败直接返回
// TODO: 要么全做，要么不做
func downloadTweetMedia(ctx context.Context, client *resty.Client, dir string, tweet *twitter.Tweet) error {
    text := utils.WinFileName(tweet.Text)
    
    for _, u := range tweet.Urls {
        ext, err := utils.GetExtFromUrl(u)
        if err != nil {
            return err  // 直接返回错误，中断整个推文
        }
        
        resp, err := client.R().SetContext(ctx).SetQueryParam("name", "4096x4096").Get(u)
        if err != nil {
            return err  // 直接返回错误
        }
        
        mutex.Lock()
        path, err := utils.UniquePath(filepath.Join(dir, text+ext))
        if err != nil {
            mutex.Unlock()
            return err
        }
        file, err := os.Create(path)
        mutex.Unlock()
        if err != nil {
            return err
        }
        
        defer os.Chtimes(path, time.Time{}, tweet.CreatedAt)
        defer file.Close()
        
        _, err = file.Write(resp.Body())
        if err != nil {
            return err  // 直接返回错误
        }
    }
    return nil
}
```

**特点**: 
- 任一媒体下载失败即返回错误
- 使用简单的文件写入
- 无重试机制
- 无去重机制

#### 当前版本 (tweet_download.go:L219-276)

```go
func downloadTweetMedia(cfg *workerConfig, dir string, tweet *twitter.Tweet, skipLoongTweet bool) error {
    // ... 保存 JSON/TXT ...
    
    for _, u := range tweet.Urls {
        ext, err := utils.GetExtFromUrl(u)
        if err != nil {
            ext = ".jpg"  // 默认扩展名，不中断
        }
        
        queryParams := make(map[string]string)
        if !strings.Contains(u, "tweet_video") && !strings.Contains(u, "video.twimg.com") && !strings.Contains(u, "?name=") {
            queryParams["name"] = "4096x4096"
        }
        
        mediaMutex.Lock()
        path, err := tweetNaming.FilePath(dir, ext)
        // ... 错误处理 ...
        mediaMutex.Unlock()
        
        req := downloader.DownloadRequest{
            Context:     cfg.ctx,
            Client:      cfg.client,
            URL:         u,
            Destination: path,
            Options: downloader.DownloadOptions{
                QueryParams: queryParams,
                SetModTime:  &tweet.CreatedAt,
            },
        }
        
        result, err := cfg.downloader.Download(req)
        if err != nil {
            log.Warnln("failed to download media:", u, "-", err)
            continue  // 继续下一个媒体，不中断
        }
        if !result.Success {
            log.Warnln("media download reported failure:", u, "-", result.Error)
            continue  // 继续下一个媒体
        }
    }
    return nil
}
```

**特点**:
- 单个媒体失败继续处理其他媒体
- 使用 `downloader.Downloader` 接口
- 支持版本管理和原子写入
- 支持 MD5 去重

### 4.2 客户端选择逻辑对比

#### 旧版本 (client.go:L436-479)

```go
func SelectClient(ctx context.Context, clients []*resty.Client, path string) *resty.Client {
    for ctx.Err() == nil {
        errs := 0
        for _, client := range clients {
            if GetClientError(client) != nil {
                errs++
                continue
            }
            
            rl := GetClientRateLimiter(client)
            if rl == nil || !rl.wouldBlock(path) {
                return client
            }
        }
        
        if errs == len(clients) {
            return nil
        }
        
        // 等待逻辑...
        select {
        case <-ctx.Done():
        case <-time.After(3 * time.Second):
        }
    }
    return nil
}
```

#### 当前版本 (client.go:L532-594)

```go
// SelectClientMFQ 带指数退避的 MFQ 客户端选择
// Q1: 只用附加账户（非受保护用户优先）
// Q2: 附加账户 + 主账户 + 指数退避
// Q3: 主账户独占（受保护用户）
func SelectClientMFQ(ctx context.Context, master *resty.Client, additional []*resty.Client, user *User, path string) *resty.Client {
    // Q3: 受保护用户 → 主账户独占
    if user.IsProtected {
        return master
    }
    
    // Q1: 只用附加账户
    for _, cli := range additional {
        if GetClientError(cli) != nil {
            continue
        }
        rl := GetClientRateLimiter(cli)
        if rl == nil || !rl.wouldBlock(path) {
            return cli
        }
    }
    
    // Q2: 附加账户 + 主账户 + 指数退避
    backoff := 3 * time.Second
    maxBackoff := 60 * time.Second
    clients := append(append([]*resty.Client{}, additional...), master)
    
    for ctx.Err() == nil {
        available := false
        errs := 0
        for _, cli := range clients {
            if GetClientError(cli) != nil {
                errs++
                continue
            }
            rl := GetClientRateLimiter(cli)
            if rl == nil || !rl.wouldBlock(path) {
                return cli
            }
            available = true
        }
        
        if errs == len(clients) {
            return nil
        }
        
        if !available {
            break
        }
        
        // 指数退避
        select {
        case <-ctx.Done():
            return nil
        case <-time.After(backoff):
            backoff = min(backoff*2, maxBackoff)
        }
    }
    return nil
}
```

**改进**:
- 引入 MFQ (Multi-Factor Queue) 选择策略
- 受保护用户优先使用主账户
- 指数退避机制避免频繁轮询

### 4.3 重试条件对比

#### 旧版本 (client.go:L59-78)

```go
client.SetRetryCount(5)
client.AddRetryCondition(func(r *resty.Response, err error) bool {
    if err == ErrWouldBlock {
        return false
    }
    // For TCP Error
    _, ok := err.(*TwitterApiError)
    _, ok2 := err.(*utils.HttpStatusError)
    return !ok && !ok2 && err != nil
})
client.AddRetryCondition(func(r *resty.Response, err error) bool {
    // For Twitter API Error
    v, ok := err.(*TwitterApiError)
    return ok && r.Request.RawRequest.Host == "x.com" && 
           (v.Code == ErrTimeout || v.Code == ErrOverCapacity || v.Code == ErrDependency)
})
client.AddRetryCondition(func(r *resty.Response, err error) bool {
    // For Http 429
    v, ok := err.(*utils.HttpStatusError)
    return ok && r.Request.RawRequest.Host == "x.com" && v.Code == 429
})
```

#### 当前版本 (client.go:L84-136)

```go
client.SetRetryCount(5)

// 条件 1: TCP/网络错误（非 Twitter API 错误）
client.AddRetryCondition(func(r *resty.Response, err error) bool {
    if err == ErrWouldBlock {
        return false
    }
    // 网络连接错误（连接重置、断开等）
    if err != nil {
        errStr := err.Error()
        if strings.Contains(errStr, "connection reset") ||
            strings.Contains(errStr, "broken pipe") ||
            strings.Contains(errStr, "timeout") {
            return true
        }
    }
    _, ok := err.(*TwitterApiError)
    _, ok2 := err.(*utils.HttpStatusError)
    return !ok && !ok2 && err != nil
})

// 条件 2: Twitter API 错误（包括服务器内部错误）
client.AddRetryCondition(func(r *resty.Response, err error) bool {
    v, ok := err.(*TwitterApiError)
    if !ok {
        return false
    }
    switch v.Code {
    case ErrTimeout, ErrOverCapacity, ErrDependency, -1:  // 新增 -1 处理
        return true
    }
    return false
})

// 条件 3: HTTP 状态码错误（新增）
client.AddRetryCondition(func(r *resty.Response, err error) bool {
    if r == nil {
        return false
    }
    // HTTP 5xx 服务器错误
    if r.StatusCode() >= 500 && r.StatusCode() < 600 {
        return true
    }
    // HTTP 429 速率限制
    v, ok := err.(*utils.HttpStatusError)
    return ok && r.Request != nil && r.Request.RawRequest != nil && 
           r.Request.RawRequest.Host == "x.com" && v.Code == 429
})
```

**改进**:
- 新增 HTTP 5xx 错误重试
- 更详细的网络错误检测
- 添加 nil 检查避免 panic

---

## 五、依赖库对比

### 5.1 go.mod 对比

| 依赖 | 旧版本 | 当前版本 | 说明 |
|------|--------|----------|------|
| Go 版本 | 1.22.3 | 1.25.0 | 升级 |
| resty/v2 | v2.14.0 | v2.14.0 | 相同 |
| sqlx | v1.4.0 | v1.4.0 | 相同 |
| sqlite3 | v1.14.22 | v1.14.22 | 相同 |
| gjson | v1.17.3 | v1.17.3 | 相同 |
| yaml.v3 | v3.0.1 | v3.0.1 | 相同 |
| logrus | v1.9.3 (indirect) | v1.9.3 (direct) | 提升为直接依赖 |
| ants/v2 | v2.10.0 (indirect) | v2.10.0 (direct) | 提升为直接依赖 |
| **lumberjack** | ❌ | v2.0.0+incompatible | 新增（日志轮转） |
| **testify** | ❌ | v1.8.4 | 新增（测试框架） |

### 5.2 新增依赖说明

**lumberjack**: 日志文件轮转管理
```go
logWriter := &lumberjack.Logger{
    Filename:   logPath,
    MaxSize:    2,    // MB
    MaxBackups: 2,
    MaxAge:     14,   // days
    Compress:   false,
}
```

**testify**: 更完善的单元测试支持

---

## 六、数据库 Schema 对比

### 6.1 旧版本 Schema

```sql
-- users 表
CREATE TABLE IF NOT EXISTS users (
    id INTEGER NOT NULL, 
    screen_name VARCHAR NOT NULL, 
    name VARCHAR NOT NULL, 
    protected BOOLEAN NOT NULL, 
    friends_count INTEGER NOT NULL, 
    PRIMARY KEY (id), 
    UNIQUE (screen_name)
);

-- user_previous_names 表
CREATE TABLE IF NOT EXISTS user_previous_names (...);

-- lsts 表
CREATE TABLE IF NOT EXISTS lsts (...);

-- lst_entities 表
CREATE TABLE IF NOT EXISTS lst_entities (...);

-- user_entities 表
CREATE TABLE IF NOT EXISTS user_entities (
    id INTEGER NOT NULL, 
    user_id INTEGER NOT NULL, 
    name VARCHAR NOT NULL, 
    latest_release_time DATETIME, 
    parent_dir VARCHAR COLLATE NOCASE NOT NULL, 
    media_count INTEGER,
    PRIMARY KEY (id), 
    UNIQUE (user_id, parent_dir), 
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- user_links 表
CREATE TABLE IF NOT EXISTS user_links (...);
```

### 6.2 当前版本 Schema

与旧版本基本相同，但增加了：
- 更完善的索引
- 数据库迁移支持（`MigrateDatabase` 函数）
- `is_accessible` 字段（users 表，v2.8.0 新增）
- `SetUserAccessible` 函数支持标记不可访问用户

---

## 七、性能与稳定性改进

### 7.1 文件写入改进

| 方面 | 旧版本 | 当前版本 |
|------|--------|----------|
| 写入方式 | 直接写入 | 原子写入（临时文件+重命名） |
| 去重机制 | ❌ 无 | ✅ MD5 校验 |
| 版本管理 | ❌ 无 | ✅ 自动备份旧版本 |
| 并发安全 | Mutex 锁 | 更细粒度的锁管理 |

### 7.2 网络请求改进

| 方面 | 旧版本 | 当前版本 |
|------|--------|----------|
| 代理支持 | 标准库 | 增强（支持 HTTP_PROXY 回退） |
| 重试条件 | 3 种 | 4 种（新增 5xx 重试） |
| 客户端选择 | 简单轮询 | MFQ 策略 + 指数退避 |
| 速率限制 | 基本支持 | 改进的日志和等待提示 |

### 7.3 并发控制改进

```go
// 旧版本
var mutex sync.Mutex  // 全局锁

// 当前版本
var mediaMutex sync.Mutex  // 更细粒度的锁
// 使用 downloader.FileWriter 接口抽象
```

---

## 八、代码质量对比

### 8.1 代码组织

| 指标 | 旧版本 | 当前版本 |
|------|--------|----------|
| 包数量 | 4 | 8 |
| 代码行数 | ~3000 | ~6000+ |
| 接口定义 | 1 (PackgedTweet) | 6+ (Entity, Downloader, FileWriter, VersionManager, PackagedTweet) |
| 测试文件 | 3 | 49 |

### 8.2 设计模式应用

**当前版本新增**:
- **依赖注入**: `downloader.Downloader` 接口注入
- **接口隔离**: 小接口设计
- **单一职责**: 每个包职责明确
- **工厂模式**: `NewDownloader`, `NewFileWriter`

### 8.3 错误处理

**旧版本**:
```go
if err != nil {
    return err  // 简单返回
}
```

**当前版本**:
```go
if err != nil {
    log.Warnln("failed to download media:", u, "-", err)
    continue  // 更优雅的处理
}
```

---

## 九、潜在问题与改进点

### 9.1 当前版本存在的问题

1. **媒体下载失败不记录到 errors.json**
   - 原因: `downloadTweetMedia` 中媒体下载失败只打印日志，不返回错误
   - 影响: 网络中断导致的 `unexpected EOF` 无法自动重试

2. **代码复杂度增加**
   - 架构分层导致代码量增加
   - 学习成本提高

3. **依赖增多**
   - 新增 lumberjack, testify 等依赖
   - 维护成本增加

### 9.2 改进建议

1. **增强媒体下载失败处理**
   ```go
   // 建议：添加部分失败检测
   type MediaDownloadResult struct {
       SuccessCount int
       FailedURLs   []string
   }
   ```

2. **添加配置热重载**
   - 支持运行时修改配置

3. **完善监控指标**
   - 下载成功率统计
   - 速率限制触发次数

---

## 十、总结

### 10.1 版本演进路线

```
tmd-2.4.4 (基础版本)
    ↓
[架构重构]
    ↓
当前版本 (功能增强版)
    ├── Profile 下载
    ├── JSON 导入/导出
    ├── 版本管理
    └── 原子文件写入
```

### 10.2 关键改进点

1. **架构升级**: 从扁平化到分层架构
2. **功能增强**: 新增 Profile、JSON、标记等功能
3. **稳定性提升**: 原子写入、MD5 去重、版本管理
4. **性能优化**: MFQ 客户端选择、指数退避

### 10.3 适用场景建议

| 场景 | 推荐版本 |
|------|----------|
| 简单下载需求 | 旧版本 (2.4.4) |
| 需要 Profile 备份 | 当前版本 |
| 数据完整性要求高 | 当前版本 |
| 与其他工具集成 | 当前版本 (JSON 支持) |

---

*报告生成时间: 2026-04-17*
*对比版本: tmd-2.4.4 vs v3.4.19*
