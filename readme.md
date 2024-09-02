# Twitter Media Downloader

[![Go Reference](https://pkg.go.dev/badge/github.com/unkmonster/tmd.svg)](https://pkg.go.dev/github.com/unkmonster/tmd)
[![Go Report Card](https://goreportcard.com/badge/github.com/unkmonster/tmd)](https://goreportcard.com/report/github.com/unkmonster/tmd)
[![Coverage Status](https://coveralls.io/repos/github/unkmonster/tmd/badge.svg?branch=master)](https://coveralls.io/github/unkmonster/tmd?branch=master)
[![Go](https://github.com/unkmonster/tmd/actions/workflows/go.yml/badge.svg)](https://github.com/unkmonster/tmd/actions/workflows/go.yml)
![GitHub Release](https://img.shields.io/github/v/release/unkmonster/tmd) 
![GitHub License](https://img.shields.io/github/license/unkmonster/tmd?logo=github)

跨平台的推特媒体下载器。用于轻松，快速，安全，整洁，批量的下载推特上用户的推文。支持手动指定用户或通过列表、用户关注批量下载。。。开箱即用！

## Feature

- 下载指定用户的媒体推文 (video, img, gif)
- 保留推文标题
- 保留推文发布日期，设置为文件的修改时间
- 以列表为单位批量下载
- 关注中的用户批量下载
- 在文件系统中保留列表/关注结构
- 同步用户/列表信息：名称，是否受保护，等。。。
- 记录用户曾用名
- 避免重复下载
  - 每次工作后记录用户的最新发布时间，下次工作仅从这个时间点开始拉取用户推文
  - 向列表目录发送指向用户目录的符号链接，无论多少列表包含同一用户，本地仅保存一份用户存档
- 避免重复获取时间线：任意一段时间内的推文仅仅会从 twitter 上拉取一次，即使这些推文下载失败。如果下载失败将它们存储到本地，以待重试或丢弃
- 避免重复同步用户（更新用户信息，获取时间线，下载推文）
- 速率限制：避免触发 Twitter API 速率限制

## How to use

### 下载/编译

**直接下载**

前往 [Release](https://github.com/unkmonster/tmd/releases/latest) 自行选择合适的版本并下载

**自行编译**

```bash
git clone https://github.com/unkmonster/tmd
cd tmd
go build .
```

### 更新/填写配置

第一次运行程序时，程序会询问如下配置信息，请按要求将配置项依次填入

#### 配置项介绍

1. `storeage path`：存储路径(可以不存在)
2. `auth_token`：用于登录，[获取方式](https://github.com/unkmonster/tmd/blob/master/help.md#获取-cookie)
3. `ct0`：用于登录，[获取方式](https://github.com/unkmonster/tmd/blob/master/help.md#获取-cookie)
4. `max_download_routine`：最大并发下载协程数（如果为0取默认值）

#### 更新配置

```shell
tmd --conf
```

> **执行上述命令将导致引导配置程序重新运行，这将重新配置整个配置文件，而不是单独的配置项。单独修改配置项**请至 `%appdata%/.tmd/conf.yaml` 或 `$HOME/.tmd/conf.yaml`手动修改

### 命令说明

```
tmd --help                 // 显示帮助
tmd --conf                 // 重新运行配置程序
tmd --user <user_id>       // 下载由 user_id 指定的用户的推文
tmd --user <screen_name>   // 下载由 screen_name 指定的用户的推文
tmd --list <list_id>       // 批量下载由 list_id 指定的列表中的每个用户
tmd --foll <user_id>       // 批量下载由 user_id 指定的用户正关注的每个用户
tmd --foll <screen_name>   // 批量下载由 screen_name 指定的用户正关注的每个用户
```

> 为了创建符号链接，在 Windows 上应该以管理员身份运行程序

[不知道啥是 user_id/list_id/screen_name?](https://github.com/unkmonster/tmd/blob/master/help.md#%E8%8E%B7%E5%8F%96-list_id-user_id-screen_name)

### 示例

```
tmd --user elonmusk  // 下载 screen_name 为 ‘eronmusk’ 的用户
tmd --user 1234567   // 下载 user_id 为 1234567 的用户
tmd --list 8901234   // 下载 list_id 为 8901234 的列表
tmd --foll 567890    // 下载 user_id 为 567890 的用户正关注的所有用户
```

更推荐的做法：一次运行

```shell
tmd --user elonmusk --user 1234567 --list 8901234 --foll 567890
```

### 设置代理

运行前通过环境变量指定代理服务器（TUN 模式跳过这一步）

```bash
set HTTP_PROXY=url
set HTTPS_PROXY=url
```

示例：
```bash
set HTTP_PROXY=http://127.0.0.1:7890
set HTTPS_PROXY=http://127.0.0.1:7890
tmd --user elonmusk
```

### 忽略用户

程序默认会忽略被静音或被屏蔽的用户，所以当你想要下载的列表中包含你不想包含的用户，可以在推特将他们屏蔽或静音

## Detail

### 关于速率限制

Twitter API 限制一段时间内过快的请求 （例如某端点每15分钟仅允许请求500次，超出这个次数会以429响应），**所以当某一端点的请求次数将要达到这段时间内允许的上限，程序会打印一条信息后 Sleep 直到可用次数刷新。但这仅会阻塞尝试请求此端点的 goroutine，所以后续可能有来自其余 goroutine 打印的内容迅速将这条 Sleep 通知覆盖 （程序是流水线式的工作流），让人认为程序莫名没有反应了**，等待至可用次数刷新后程序会继续工作，这最多是 `15` 分钟

## Contributors

![](https://contrib.rocks/image?repo=unkmonster/tmd) 

