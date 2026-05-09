# foo.db 跨平台迁移说明

本文档说明如何在不同平台之间迁移 `foo.db`，以及仓库内新增的独立迁移工具 `tmd-db-migrate` 的使用方式。

适用场景：

- Windows 主机迁移到 Linux / Docker
- Linux 主机迁移到 Windows
- 本机下载根目录发生变化，例如 `F:\twitter_dl` 改为 `/data`

不适用场景：

- 多个实例同时写同一个 `foo.db`
- 只拷贝 `foo.db`，不拷贝实际下载目录
- 期望把数据库里的绝对路径自动改成相对路径

## 1. 为什么需要这个工具

`foo.db` 位于下载根目录下的 `.data/foo.db`，数据库中的以下字段保存的是实体父目录路径：

- `user_entities.parent_dir`
- `lst_entities.parent_dir`

历史数据里这些字段通常是绝对路径，例如：

```text
F:\twitter_dl\users
F:\twitter_dl
```

当下载目录整体迁移到另一台机器或另一种平台后，例如改为：

```text
/data/users
/data
```

原数据库中的绝对路径不会自动变化，结果就是：

- 程序仍然认为实体目录在旧路径下
- 已有下载记录和当前磁盘目录脱节
- 后续查询、同步、目录定位可能异常

这个工具只做一件事：把 `parent_dir` 从旧根目录重写到新根目录。

## 2. 工具位置

命令入口：

- [C:\Users\leeexxx\Documents\trae_projects\tmd\tools\tmd-db-migrate\main.go](C:\Users\leeexxx\Documents\trae_projects\tmd\tools\tmd-db-migrate\main.go)

核心实现：

- [C:\Users\leeexxx\Documents\trae_projects\tmd\internal\database\parent_dir_migration.go](C:\Users\leeexxx\Documents\trae_projects\tmd\internal\database\parent_dir_migration.go)

## 3. 工具行为

工具执行时会：

1. 打开目标 `foo.db`
2. 扫描 `user_entities.parent_dir` 和 `lst_entities.parent_dir`
3. 找出所有以 `from-root` 开头的绝对路径
4. 重写为 `to-root`
5. 在正式写入前检查是否会撞上唯一约束
6. 正式写入前备份 `foo.db`

备份行为：

- 备份主文件：`foo.db.backup.YYYYMMDD_HHMMSS`
- 如果存在，也会一并备份：
  - `foo.db-wal`
  - `foo.db-shm`

正式迁移完成后，程序不会另外生成一个新的正式 `foo.db`。它会直接原地更新 `--db` 指向的那个数据库文件，备份文件也放在同一目录下。

## 4. 支持的路径风格

工具不依赖当前运行平台的 `filepath` 语义，而是自己识别路径风格。因此可以在 Linux 上处理 Windows 风格数据库，也可以在 Windows 上把路径改成 Docker/Linux 风格。

当前支持：

- Windows drive path  
  例如：`F:\twitter_dl`

- Windows UNC path  
  例如：`\\server\share\twitter_dl`

- POSIX path  
  例如：`/data`

匹配规则：

- Windows 路径前缀匹配大小写不敏感
- POSIX 路径前缀匹配大小写敏感
- 相对路径不会被改写

## 5. 基本用法

先预演：

```bash
go run ./tools/tmd-db-migrate --db "F:\twitter_dl\.data\foo.db" --from-root "F:\twitter_dl" --to-root "/data" --dry-run
```

正式迁移：

```bash
go run ./tools/tmd-db-migrate --db "F:\twitter_dl\.data\foo.db" --from-root "F:\twitter_dl" --to-root "/data"
```

命令参数：

- `--db`
  - 必填
  - 目标数据库文件路径

- `--from-root`
  - 必填
  - 旧下载根目录

- `--to-root`
  - 必填
  - 新下载根目录

- `--dry-run`
  - 可选
  - 只输出迁移摘要，不写库

## 6. 输出说明

工具会输出类似信息：

```text
dry_run=false
user_entities: 12/25 updated
lst_entities: 2/4 updated
backup=F:\twitter_dl\.data\foo.db.backup.20260509_224500
samples:
- user_entities id=1: "F:\\twitter_dl\\users" -> "/data/users"
- lst_entities id=3: "F:\\twitter_dl" -> "/data"
```

含义：

- `updated/total` 表示该表总记录数中有多少条被改写
- `backup` 表示正式迁移前生成的备份文件
- `samples` 只展示部分样例，便于快速确认方向是否正确

## 7. 迁移前建议

建议按这个顺序做：

1. 停止所有正在使用该下载目录的 TMD 进程
2. 完整备份整个下载根目录，而不只是 `foo.db`
3. 先执行 `--dry-run`
4. 确认输出的样例路径是你期望的目标路径
5. 再执行正式迁移

不要在程序运行中直接改库。当前项目 SQLite 使用 WAL 且连接串行化，但运行中改库仍然会引入不必要风险。

## 8. 迁移后建议校验

建议至少检查以下几点：

1. 数据库是否还能打开
2. 用户实体和列表实体能否正确定位目录
3. 主程序启动后是否能正常连接数据库
4. 随机抽查几个用户目录和列表目录

推荐校验方式：

```bash
go test ./...
```

或直接启动主程序，用现有下载根目录做一次只读/轻量操作验证。

## 9. 冲突保护

数据库 schema 对以下组合有唯一约束：

- `user_entities(user_id, parent_dir)`
- `lst_entities(lst_id, parent_dir)`

因此如果迁移后会出现两条记录落到同一个 `(owner, parent_dir)` 上，工具会直接失败并拒绝写入。

例如：

```text
旧记录1: user_id=1, parent_dir=F:\twitter_dl\users
旧记录2: user_id=1, parent_dir=/data/users
```

当你把 `F:\twitter_dl` 改写成 `/data` 时，这两条会冲突。此时工具会报错，数据库保持原样。

这一步是故意做严格的，不自动合并记录。

## 10. 当前边界

当前工具只处理 `parent_dir` 的根目录改写，不处理：

- schema 升级
- `is_accessible` 等历史 migration
- 把绝对路径批量转换为相对路径
- 清理重复实体
- 修改用户目录名或列表目录名

也就是说，它解决的是：

> “数据库里路径前缀还是旧平台/旧磁盘的值”

而不是一切数据库兼容问题。

补充说明：

- 在当前真实数据模型里，`user_entities.parent_dir` 通常是 `F:\twitter_dl\users`
- `lst_entities.parent_dir` 通常直接是下载根目录本身，例如 `F:\twitter_dl`

因此迁移时推荐始终传：

```text
--from-root "旧下载根目录"
--to-root "新下载根目录"
```

不要把 `--from-root` 误写成 `F:\twitter_dl\users`。那样只会命中 `user_entities`，`lst_entities` 不会被改写。

## 11. 设计取舍

当前没有直接把 `parent_dir` 改成相对路径，原因是：

- 现有数据库读写逻辑仍然按绝对路径工作
- 一次性改成相对路径会扩大兼容面
- 当前最直接的问题是跨平台根路径变化，不是 schema 重设计

所以当前方案是一个保守工具：

- 不改业务模型
- 不改主程序现有路径语义
- 只做可预期的前缀重写

如果后续要把 `foo.db` 彻底改成“与宿主路径无关”的相对路径模型，那会是下一阶段的 schema/兼容改造，不在这个工具范围内。
