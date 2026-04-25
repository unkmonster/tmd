# Phase 3: CLI 适配计划

## 1. 目标

将 CLI 层从直接执行下载逻辑，改为调用 Service 层。移除重复的初始化代码，实现统一的进度报告（日志输出）。

## 2. 设计原则

遵循 CLAUDE.md 准则：
- **简单优先**：CLI 只做参数解析和调用 Service，不处理业务逻辑
- **外科手术式修改**：保留 CLI 的入口行为，只修改内部实现
- **目标驱动**：确保 CLI 行为与重构前完全一致

## 3. 当前 CLI 架构分析

```
当前 CLI 流程:
main.go → cli.Execute() → ParseArgs() → MakeTask() → executeBatchDownload()
                                    ↓
                              初始化 downloader
                              初始化 dumper
                              调用 downloading 包
```

```
目标 CLI 流程:
main.go → cli.Execute() → ParseArgs() → Service.UserDownload/ListDownload/...
                                    ↓
                              统一的依赖注入
                              日志进度报告
```

## 4. 模块划分

```
internal/
├── cli/
│   ├── executor.go         # 修改：调用 Service
│   ├── executor_download.go # 删除：逻辑移至 Service
│   ├── executor_json.go     # 删除：逻辑移至 Service
│   ├── executor_mark.go     # 删除：逻辑移至 Service
│   ├── executor_profile.go  # 删除：逻辑移至 Service
│   ├── args.go             # 保持不变
│   ├── helpers.go          # 修改：简化 MakeTask
│   ├── paths.go            # 保持不变
│   └── progress.go         # 新增：CLI 进度报告实现
├── service/                # Phase 2 创建
│   └── ...
```

## 5. 详细步骤

### 步骤 1: 创建 CLI 进度报告实现

**文件**: `internal/cli/progress.go`

**新增代码**:
```go
package cli

import (
	"github.com/unkmonster/tmd/internal/service"
	log "github.com/sirupsen/logrus"
)

// LogProgressReporter 日志进度报告器
type LogProgressReporter struct{}

func NewLogProgressReporter() service.ProgressReporter {
	return &LogProgressReporter{}
}

func (l *LogProgressReporter) OnProgress(taskID string, p service.Progress) {
	switch p.Stage {
	case "syncing":
		log.Infof("[%s] Syncing: %s", taskID, p.Current)
	case "downloading":
		if p.Total > 0 {
			log.Infof("[%s] Downloading: %s (%d/%d)", taskID, p.Current, p.Completed, p.Total)
		} else {
			log.Infof("[%s] Downloading: %s", taskID, p.Current)
		}
	case "profile":
		log.Infof("[%s] Downloading profiles...", taskID)
	case "marking":
		log.Infof("[%s] Marking: %s", taskID, p.Current)
	default:
		log.Infof("[%s] %s: %s", taskID, p.Stage, p.Current)
	}
}

func (l *LogProgressReporter) OnComplete(taskID string, r service.Result) {
	if r.Downloaded > 0 || r.Failed > 0 {
		log.Infof("[%s] Completed: %d downloaded, %d failed, %d skipped", 
			taskID, r.Downloaded, r.Failed, r.Skipped)
	} else {
		log.Infof("[%s] Completed: %s", taskID, r.Message)
	}
}

func (l *LogProgressReporter) OnError(taskID string, err error) {
	log.Errorf("[%s] Error: %v", taskID, err)
}
```

**风险评估**:
- 风险: 低
- 注意: 保持与现有日志格式一致

**测试要点**:
- 编译通过
- 日志输出格式正确

---

### 步骤 2: 修改 Dependencies 结构体

**文件**: `internal/cli/executor.go`

**修改前**:
```go
// Dependencies 执行依赖
type Dependencies struct {
	Client            *resty.Client
	AdditionalClients []*resty.Client
	DB                *sqlx.DB
	Conf              *config.Config
	AppRootPath       string
}
```

**修改后**:
```go
// Dependencies 执行依赖
type Dependencies struct {
	Client            *resty.Client
	AdditionalClients []*resty.Client
	DB                *sqlx.DB
	Conf              *config.Config
	AppRootPath       string
	
	// Service 层依赖（新增）
	DownloadService service.DownloadService
}
```

**风险评估**:
- 风险: 低
- 注意: 需要在 main.go 中初始化 Service

---

### 步骤 3: 重写 Execute 函数

**文件**: `internal/cli/executor.go`

**修改前**:
```go
// Execute 执行 CLI 命令
func Execute(ctx context.Context, args []string, deps *Dependencies) error {
	// 解析参数
	_, cfg, err := ParseArgs(args)
	if err != nil {
		return fmt.Errorf("parse args failed: %w", err)
	}

	// 获取存储路径
	pathHelper, err := NewStorePath(deps.Conf.RootPath)
	if err != nil {
		return fmt.Errorf("failed to make store dir: %w", err)
	}

	// 初始化下载器
	versionManager := downloader.NewVersionManagerWithWriter(".versions", nil)
	fileWriter := downloader.NewFileWriter(versionManager)
	versionManager.SetFileWriter(fileWriter)
	dwn := downloader.NewDownloader(fileWriter)

	// 初始化 Dumper
	dumper := downloading.NewDumper()
	_ = dumper.Load(pathHelper.ErrorJ)

	// 创建任务
	task, err := MakeTask(ctx, deps.Client, deps.DB, cfg.UsrArgs, cfg.ListArgs, cfg.FollArgs)
	if err != nil {
		return fmt.Errorf("failed to make task: %w", err)
	}

	// 保存 Dumper 的 defer
	defer func() {
		if dumper.Count() > 0 {
			dumper.Dump(pathHelper.ErrorJ)
			log.Infof("%d tweets have been dumped", dumper.Count())
		}
	}()

	// 检查是否有下载任务
	if len(task.Users) == 0 && len(task.Lists) == 0 && len(cfg.JsonArgs.GetPaths()) == 0 {
		return handleProfileOnly(ctx, cfg, deps, pathHelper, versionManager, fileWriter, dwn)
	}

	log.Infoln("start working for...")
	PrintTask(task)

	// 执行下载
	if cfg.MarkDownloaded {
		return executeMarkDownloaded(ctx, cfg, deps, task, pathHelper)
	}

	if len(cfg.JsonArgs.GetPaths()) > 0 {
		return executeJsonDownload(ctx, cfg, deps, pathHelper, dwn, fileWriter)
	}

	return executeBatchDownload(ctx, cfg, deps, task, pathHelper, dwn, fileWriter, versionManager, dumper)
}
```

**修改后**:
```go
// Execute 执行 CLI 命令
func Execute(ctx context.Context, args []string, deps *Dependencies) error {
	// 解析参数
	_, cfg, err := ParseArgs(args)
	if err != nil {
		return fmt.Errorf("parse args failed: %w", err)
	}

	// 如果没有初始化 Service，创建默认实例
	if deps.DownloadService == nil {
		deps.DownloadService = service.NewDownloadService(&service.Dependencies{
			Client:            deps.Client,
			AdditionalClients: deps.AdditionalClients,
			DB:                deps.DB,
			Config:            deps.Conf,
			AppRootPath:       deps.AppRootPath,
		})
	}

	// 创建进度报告器
	reporter := NewLogProgressReporter()

	// 生成任务 ID（CLI 模式使用简单标识）
	taskID := "cli"

	// 处理不同类型的下载
	
	// 1. JSON 下载
	if len(cfg.JsonArgs.GetPaths()) > 0 {
		return deps.DownloadService.JsonDownload(ctx, taskID, cfg.JsonArgs.GetPaths(), cfg.NoRetry, reporter)
	}

	// 2. 标记已下载
	if cfg.MarkDownloaded {
		if len(cfg.UsrArgs.ScreenName) > 0 {
			return deps.DownloadService.MarkDownloaded(ctx, taskID, cfg.UsrArgs.ScreenName[0], &cfg.MarkTime, reporter)
		}
		return fmt.Errorf("mark-downloaded requires -user")
	}

	// 3. 仅 Profile 下载
	if len(cfg.ProfileUsers.ScreenName) > 0 || len(cfg.ProfileList.ID) > 0 {
		opts := service.DownloadOptions{SkipProfile: cfg.NoProfile}
		
		// Profile 用户
		if len(cfg.ProfileUsers.ScreenName) > 0 {
			return deps.DownloadService.ProfileDownload(ctx, taskID, cfg.ProfileUsers.ScreenName, reporter)
		}
		
		// Profile 列表
		if len(cfg.ProfileList.ID) > 0 {
			return deps.DownloadService.ListProfileDownload(ctx, taskID, cfg.ProfileList.ID[0], reporter)
		}
	}

	// 4. 批量下载（包括用户、列表、关注）
	var users []string
	var lists []uint64

	// 收集用户
	users = append(users, cfg.UsrArgs.ScreenName...)

	// 收集列表
	lists = append(lists, cfg.ListArgs.ID...)

	// 收集关注（需要获取用户信息，然后转换为 Following）
	for _, screenName := range cfg.FollArgs.ScreenName {
		users = append(users, screenName) // Following 通过 Service 层处理
	}
	for _, id := range cfg.FollArgs.ID {
		// 通过 ID 获取 screen name，这里简化处理
		// 实际实现可能需要先查询用户信息
		log.Warnf("Following download by ID not fully supported in simplified mode: %d", id)
	}

	if len(users) == 0 && len(lists) == 0 {
		return fmt.Errorf("no download target specified")
	}

	opts := service.DownloadOptions{
		AutoFollow:  cfg.AutoFollow,
		SkipProfile: cfg.NoProfile,
		NoRetry:     cfg.NoRetry,
	}

	return deps.DownloadService.BatchDownload(ctx, taskID, users, lists, opts, reporter)
}
```

**风险评估**:
- 风险: **高**（核心入口函数）
- 注意:
  - 确保所有参数组合都能正确处理
  - Following 参数的处理需要特别注意
  - 保持与现有 CLI 行为一致

**测试要点**:
- 编译通过
- 各种参数组合测试:
  - `-user username`
  - `-list 12345`
  - `-foll username`
  - `-profile-user username`
  - `-profile-list 12345`
  - `-mark-downloaded -user username`
  - `-json path/to/file.json`
  - 组合参数

---

### 步骤 4: 简化 MakeTask（可选）

**文件**: `internal/cli/helpers.go`

**分析**: `MakeTask` 原来用于创建任务对象，现在 Service 层直接处理用户/列表获取，可以简化或移除。

**方案 A: 完全移除（推荐）**
直接删除 `MakeTask` 和 `PrintTask`，因为 Service 层已经处理了这些逻辑。

**方案 B: 保留简化版本**
如果其他地方依赖 `Task` 结构体，可以保留简化版本：

```go
// Task 任务（简化版，仅用于信息展示）
type Task struct {
	Users []string
	Lists []uint64
}

// PrintTask 打印任务信息
func PrintTask(task *Task) {
	if len(task.Users) != 0 {
		fmt.Printf("users: %d\n", len(task.Users))
	}
	for _, u := range task.Users {
		fmt.Printf("    - %s\n", u)
	}
	if len(task.Lists) != 0 {
		fmt.Printf("lists: %d\n", len(task.Lists))
	}
	for _, l := range task.Lists {
		fmt.Printf("    - %d\n", l)
	}
}
```

**风险评估**:
- 风险: 中
- 注意: 检查是否有其他包使用 `Task` 结构体

---

### 步骤 5: 删除冗余的执行器文件

**删除以下文件**:
- `internal/cli/executor_download.go`
- `internal/cli/executor_json.go`
- `internal/cli/executor_mark.go`
- `internal/cli/executor_profile.go`

**原因**: 这些文件中的逻辑已经移至 Service 层。

**风险评估**:
- 风险: 低
- 注意: 确保这些文件中的逻辑已在 Service 层完整实现

---

### 步骤 6: 修改 main.go 初始化 Service

**文件**: `main.go`

**修改前**:
```go
// 构造依赖
deps := &cli.Dependencies{
	Client:            client,
	AdditionalClients: additional,
	DB:                db,
	Conf:              conf,
	AppRootPath:       appRootPath,
}

// 将 cli 参数传递给 Execute
if err := cli.Execute(ctx, cliArgs, deps); err != nil {
	log.Fatalln("execute failed:", err)
}
```

**修改后**:
```go
// 创建 Service
downloadService := service.NewDownloadService(&service.Dependencies{
	Client:            client,
	AdditionalClients: additional,
	DB:                db,
	Config:            conf,
	AppRootPath:       appRootPath,
})

// 构造依赖
deps := &cli.Dependencies{
	Client:            client,
	AdditionalClients: additional,
	DB:                db,
	Conf:              conf,
	AppRootPath:       appRootPath,
	DownloadService:   downloadService,
}

// 将 cli 参数传递给 Execute
if err := cli.Execute(ctx, cliArgs, deps); err != nil {
	log.Fatalln("execute failed:", err)
}
```

**风险评估**:
- 风险: 低
- 注意: 确保 import 正确

---

## 6. 与现有代码的关系

### 6.1 删除的代码
- `internal/cli/executor_download.go` - 逻辑移至 Service
- `internal/cli/executor_json.go` - 逻辑移至 Service
- `internal/cli/executor_mark.go` - 逻辑移至 Service
- `internal/cli/executor_profile.go` - 逻辑移至 Service

### 6.2 修改的代码
- `internal/cli/executor.go` - 重写 Execute 函数
- `internal/cli/helpers.go` - 简化或移除 MakeTask
- `main.go` - 初始化 Service

### 6.3 新增的代码
- `internal/cli/progress.go` - 日志进度报告

### 6.4 保持不变的代码
- `internal/cli/args.go` - 参数解析
- `internal/cli/paths.go` - 路径管理
- `internal/cli/helpers_test.go` - 如有测试，保持或更新

---

## 7. 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| 参数处理不一致 | 高 | 高 | 1. 仔细对照原 CLI 参数处理逻辑<br>2. 编写全面的参数组合测试<br>3. 保留原代码作为参考 |
| Following 下载逻辑错误 | 中 | 高 | 1. 特别注意 -foll 参数的处理<br>2. 测试关注列表下载功能 |
| 进度报告格式变化 | 中 | 中 | 1. 保持与原日志格式一致<br>2. 用户可能依赖日志格式进行解析 |
| Service 层未完整实现 | 中 | 高 | 1. 确保 Phase 2 完成后再进行 Phase 3<br>2. 检查所有功能点 |

---

## 8. 验证步骤

### 8.1 编译验证
```bash
cd C:\Users\leeexxx\Documents\trae_projects\tmd
go build ./...
```

### 8.2 功能测试
```bash
# 测试用户下载
tmd -user testuser

# 测试列表下载
tmd -list 12345

# 测试关注下载
tmd -foll testuser

# 测试 Profile 下载
tmd -profile-user testuser

# 测试标记已下载
tmd -user testuser -mark-downloaded

# 测试 JSON 下载
tmd -json path/to/file.json

# 测试组合参数
tmd -user user1 -user user2 -list 12345 -auto-follow
```

### 8.3 对比测试
- 对比重构前后 CLI 的日志输出
- 对比下载结果的一致性
- 对比错误处理行为

---

## 9. 成功标准

- [ ] CLI 可以正常编译
- [ ] 所有参数组合正常工作
- [ ] 日志输出格式与重构前一致
- [ ] 下载功能与重构前行为一致
- [ ] 代码符合 CLAUDE.md 准则

---

## 10. 预计时间

- 步骤 1 (进度报告): 1 小时
- 步骤 2-3 (重写 Execute): 3-4 小时
- 步骤 4 (简化 helpers): 1 小时
- 步骤 5 (删除冗余文件): 0.5 小时
- 步骤 6 (修改 main.go): 0.5 小时
- 测试与验证: 2-3 小时
- **总计**: 8-10 小时

---

## 11. 回滚方案

如果出现问题，可以：
1. 恢复 `internal/cli/executor.go` 到原版本
2. 恢复被删除的 `executor_*.go` 文件
3. 移除 Service 相关代码

建议在进行 Phase 3 前，先备份当前可以工作的代码。
