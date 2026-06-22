# TMD 架构图

> 本文档独立呈现项目整体架构，结合 `AGENTS.md` 和原 `readme.md` 两处架构图的优点绘制，
> 且已对照实际代码验证各包的接口与职责。

```
┌──────────────────────────────────────────────────────────────────────────┐
│                              main.go                                    │
│                   进程入口：配置 / 日志 / 数据库 / 分流                    │
│          ┌─ 初始化顺序：config → logrus → database → twitter client      │
└──────────────────────────────┬───────────────────────────────────────────┘
                               │
                ┌──────────────┴──────────────┐
                │                             │
                ▼                             ▼
┌───────────────────────────┐   ┌────────────────────────────────────────────┐
│      internal/cli         │   │              internal/api                  │
│        CLI 模式            │   │              Server 模式                   │
│                           │   │                                            │
│    同步执行，直接返回结果    │   │  ┌────────────┐  ┌─────────────────────┐   │
│                           │   │  │TaskManager │  │   DownloadQueue      │   │
│   args.go → 解析参数       │   │  │ 任务管理    │  │  任务队列 + Target锁 │   │
│   executor.go → 调用      │   │  └─────┬──────┘  │ 单 worker 逐条消费    │   │
│   DownloadService         │   │        │          └─────────────────────┘   │
│                           │   │        │                                    │
│                           │   │  ┌─────▼──────────────────────────────┐    │
│                           │   │  │          EventBus                  │    │
│                           │   │  │   coalesced: tasks / schedules     │    │
│                           │   │  │   replayable: notification / ...   │    │
│                           │   │  │   JSON 预序列化 → 扇出所有订阅者     │    │
│                           │   │  └───────────────────────────────────┘    │
│                           │   │        │                                    │
│                           │   │  ┌─────▼──────────────────────────────┐    │
│                           │   │  │         SSE → Web UI              │    │
│                           │   │  │   GET /api/v1/sse/tasks           │    │
│                           │   │  │   GET /api/v1/logs/stream         │    │
│                           │   │  │   心跳 25s / 慢消费者保护 4096     │    │
│                           │   │  └───────────────────────────────────┘    │
│                           │   │                                            │
│                           │   │  ┌────────────────────────────────────┐    │
│                           │   │  │  Scheduler (独立包 internal/scheduler) │    │
│                           │   │  │  interval / daily 两种调度模式      │    │
│                           │   │  │  到期 → scheduledDownload 回调      │    │
│                           │   │  │  → 创建 Task → DownloadQueue 入队  │    │
│                           │   │  │  仅 Server 模式，CLI 不使用         │    │
│                           │   │  └────────────────────────────────────┘    │
└──────────┬────────────────┘   └───────────────────┬────────────────────────┘
           │
           └──────────────┬─────────────────────────┘
                          │
                          ▼
         ┌─────────────────────────────────────────────────────────────┐
         │                internal/service                             │
         │                ★ 统一应用服务层 ★                             │
         │                                                             │
         │      DownloadService 接口（11 种操作的唯一入口）                │
         │      被 CLI executor 和 API handler 共同调用                   │
         │                                                             │
         │    UserDownload / ListDownload / FollowingDownload           │
         │    ProfileDownload / JsonFileDownload / JsonFolderDownload   │
         │    MarkDownloaded / BatchDownload / RetryAllFailed           │
         │                                                             │
         │    内部使用 executeDownloadTemplate 模板方法统一编排流程        │
         └────────────────────┬────────────────────────────────────────┘
                              │
            ┌─────────────────┼─────────────────┐
            │                 │                 │
            ▼                 ▼                 ▼
┌─────────────────────┐ ┌──────────────┐ ┌──────────────────────┐
│   internal/         │ │  internal/   │ │  internal/           │
│   downloading       │ │  twitter     │ │  database            │
│    业务编排层        │ │  API 客户端   │ │  数据持久化           │
│                     │ │              │ │                      │
│  • BatchDownload    │ │ • GraphQL    │ │ • SQLite (WAL 模式)   │
│  • TweetDownload    │ │   endpoint   │ │ • 6 张表              │
│  • Retry / Dumper   │ │ • 多账号     │ │   users / lsts       │
│  • ListSync / Entity│ │   分流       │ │   user_entities      │
│  • MarkDownloaded   │ │ • 限流管理   │ │   lst_entities       │
│                     │ │ • Bearer     │ │   user_links         │
│  ┌───────────────┐  │ │   Token      │ │   user_prev_names    │
│  │profile 子包   │  │ │              │ │                      │
│  │头像/横幅/简介  │  │ │              │ │ • 迁移 / 事务 / 查询  │
│  │版本备份       │  │ │              │ │ • latest_release_time │
│  └───────────────┘  │ │              │ │   增量下载依据        │
└──────────┬──────────┘ └──────────────┘ └──────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                             下载基础设施层                                 │
│                                                                          │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────┐  ┌────────┐ │
│  │ downloader  │  │  entity    │  │  naming    │  │  path  │  │ utils  │ │
│  │            │  │            │  │            │  │        │  │        │ │
│  │ 3 个接口：  │  │ Entity 接口 │  │ 3 种策略：  │  │ Store  │  │ 通用   │ │
│  │ Downloader │  │ 7 个方法   │  │ UserNaming │  │ Path   │  │ 工具   │ │
│  │ FileWriter │  │ Path Create│  │ ListNaming │  │ 管理   │  │        │ │
│  │ VersionMgr │  │ Rename ... │  │ TweetNaming│  │ 6 个   │  │ 算法   │ │
│  │            │  │            │  │            │  │ 路径   │  │ HTTP   │ │
│  │ HEAD→策略  │  │ Sync 函数  │  │ UniquePath │  │ 字段   │  │ 文件   │ │
│  │ 选Buffer/  │  │ 处理用户名  │  │ Resolver   │  │        │  │ 用户   │ │
│  │ 流式       │  │ 变更重命名  │  │ 去重       │  │        │  │ win32  │ │
│  └────────────┘  └────────────┘  └────────────┘  └────────┘  └────────┘ │
└──────────────────────────────────────────────────────────────────────────┘

              ┌──────────────┐    ┌──────────────────┐
              │   config     │    │   consolelog     │
              │  配置管理     │    │  日志捕获与分发    │
              │              │    │                  │
              │ YAML 配置    │    │ 接管 stdout/     │
              │ 环境变量覆盖  │    │ stderr → OS pipe │
              │ (6 个 TMD_)  │    │ → 解析行         │
              │ 交互式配置    │    │ → 环形缓冲区     │
              │              │    │ → 扇出 subscribers│
              │ 被 main      │    │                  │
              │ 读取后注入    │    │ SSE handler 订阅  │
              │ service /    │    │ filter by level/q │
              │ scheduler    │    │                  │
              └──────────────┘    └──────────────────┘

                    其他支撑层：不直接参与下载流程，为各层提供基础能力
```

## 图例说明

| 符号 | 含义 |
|------|------|
| `▼` 实线箭头 | 调用/控制流（上层调用下层） |
| 框内缩进列表 `•` | 该模块的核心能力/接口/文件 |
| 内嵌子框 `┌──┐` | 子包（如 downloading/profile） |
| Server 内部并列框 | 各组件独立但通过 EventBus 协作 |
| 底部 5 个并列框 | 基础设施层，被上层调用 |
| 底部 2 个独立框 | 支撑层（config / consolelog），不参与主调用链 |

## 调用主线速记

```
main.go
  ├── CLI  ──→  service ──→ downloading ──→ downloader/fileWriter
  └── Server ──→                ├── twitter
                                └── database
```

## 关键设计点

| 设计 | 体现在图中 |
|------|-----------|
| **Service 复用** | CLI 和 Server 汇聚到同一个 service 框，标注"被 CLI executor 和 API handler 共同调用" |
| **模板方法模式** | service 框标注 `executeDownloadTemplate` |
| **任务异步化** | Server 框内 TaskManager + DownloadQueue + EventBus + SSE 四件套协作 |
| **多账号分流** | twitter 框内标注"多账号分流"和"限流管理" |
| **增量下载** | database 框标注 `latest_release_time` 增量下载依据 |
| **失败重试** | downloading 框内 Dumper + Retry |
| **三接口分离** | downloader 框标注 Downloader / FileWriter / VersionManager 三个独立接口 |
| **原子写入** | downloader 框标注"Buffer/流式策略选择" |
| **用户名变更处理** | entity 框标注 Sync 函数处理用户名变更重命名 |
| **三种命名策略** | naming 框标注 UserNaming / ListNaming / TweetNaming + UniquePathResolver |
| **EventBus 两种事件** | EventBus 框标注 coalesced / replayable |
| **Scheduler 回调机制** | api 框内 Scheduler 组件，标注"独立包"和"scheduledDownload 回调" |

---

## 项目目录结构

```
tmd/
├── main.go                        # 应用入口（命令行解析、模式选择）
├── start-server.bat               # Windows Server 模式启动脚本
├── internal/
│   ├── api/                       # API Server 模块
│   ├── cli/                       # CLI 命令模块
│   ├── config/                    # 配置管理
│   ├── service/                   # Service 层（核心业务编排）
│   ├── database/                  # 数据持久化层
│   ├── downloading/               # 核心下载逻辑
│   ├── downloader/                # 通用下载基础设施
│   ├── twitter/                   # Twitter API 客户端
│   ├── naming/                    # 命名服务
│   ├── entity/                    # 数据实体层
│   ├── path/                      # 路径管理
│   ├── scheduler/                 # 定时任务调度器
│   ├── consolelog/                # 控制台日志捕获与分发
│   └── utils/                     # 工具函数
├── doc/                           # 详细文档
├── tools/                         # 工具脚本（迁移工具、Tampermonkey脚本）
├── .github/workflows/             # CI/CD 配置
├── go.mod                         # Go 模块定义
├── go.sum                         # 依赖校验和
├── Dockerfile                     # Docker 镜像构建
├── docker-compose.yml             # Docker Compose 部署
├── readme.md                      # 用户手册
├── CHANGELOG.md                   # 变更日志
├── LICENSE                        # GPL-3.0 许可证
├── convert_db_to_legacy.py        # 数据库格式转换脚本
└── .gitignore                     # Git 忽略规则
```