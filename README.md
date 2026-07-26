# QWeather CLI

一个面向人类和智能代理的 QWeather 命令行客户端。

简体中文 | [English](README_EN.md)

> 本项目是非官方的 QWeather 客户端，与 QWeather 或其关联方没有隶属、赞助或背书关系。项目代码采用 Apache License 2.0；QWeather API 的访问、数据使用、归属声明和商标使用仍受 QWeather 适用条款约束。

## 功能概览

CLI 使用项目维护的稳定 capability（能力）目录，而不是把 QWeather 的上游 URL 或任意请求字段暴露给调用方。目前包含 28 个可执行能力：

- 地理：城市查询、热门城市、POI 查询和附近 POI；
- 天气：城市/网格实况、逐日、逐小时、历史、生活指数和分钟降水；
- 预警：当前天气预警；
- 空气：当前、逐日、逐小时和监测站空气质量；
- 热带气旋：列表、路径和预报；
- 海洋：潮汐预报；
- 太阳辐射：辐射预报；
- 天文：日出日落、月相和太阳位置；
- 账户：财务摘要和请求统计。

运行 `qweather --help` 或 `qweather capability list` 查看当前命令树。已弃用的上游操作只作为不可执行的生命周期记录保留，不会自动生成 CLI 命令。

## 安装

### 手动下载二进制

有可用 Release 时，从 [GitHub Releases](https://github.com/Nativu5/qweather-cli/releases) 下载适合操作系统和架构的压缩包。每个发布版本会同时提供 `checksums.txt`，下载后可用它校验 SHA256。解压后，将 `qweather`（Windows 为 `qweather.exe`）放入 `PATH` 中的目录。

发布版本将提供 macOS、Linux 和 Windows 的 `arm64`、`amd64` 二进制。

### 从源代码构建

需要与仓库 `go.mod` 兼容的 Go 工具链：

```sh
go run ./cmd/qweather --help
go build -o qweather ./cmd/qweather
```

## 配置与认证

CLI 支持 Ed25519 JWT（推荐）和 API KEY。凭据不要写入命令行参数、提交到 Git 或放入公开 Issue。将配置保存为 TOML 文件，并使用 `--config` 指定；下面是一个通过环境变量读取 API KEY 的示例：

```toml
[profiles.default]
api_host = "YOUR_ACCOUNT_API_HOST"
auth = "api_key"
api_key_env = "QWEATHER_API_KEY"
language = "auto"
unit = "metric"
```

检查配置不会发起 QWeather API 请求：

```sh
qweather config check --config /path/to/config.toml
```

API Host 必须是账户对应的 HTTPS host。认证、API Host 和 JWT 的官方说明见 [QWeather authentication](https://dev.qweather.com/docs/configuration/authentication/)、[API Host](https://dev.qweather.com/docs/configuration/api-host/) 和 [API request configuration](https://dev.qweather.com/docs/configuration/api-config/)。

## 基本用法

默认输出是适合人阅读的确定性 Text View：

```sh
qweather weather city current --place "Shanghai"
qweather weather city daily --place-id <location-id> --days 3
qweather air current --coordinate geo:31.2304,121.4737
qweather geo city lookup --query "Shanghai" --limit 5
```

使用 `geo:<latitude>,<longitude>` 表示坐标，纬度在前、经度在后。地点名称解析如果产生多个候选，CLI 会报告歧义并要求调用方明确选择；Geo 数据只在当前调用中使用，不写入持久化缓存。

机器处理时显式选择 JSON：

```sh
qweather --output json weather city current --place-id <location-id>
qweather --output json version
```

可用输出模式：

- `text`：默认的确定性 Text View；
- `json`：版本化的 `qweather.result/v1` 或 `qweather.problem/v1`；
- `body`：成功响应的原始 QWeather Provider Body。

缓存和网络行为可以通过全局选项控制：

```sh
qweather --refresh weather city current --place-id <location-id>
qweather --no-cache --output json air current --coordinate geo:31.2304,121.4737
qweather cache status
qweather cache clear
```

## 付费能力与数据归属

`--allow-product` 是真实的 CLI 选项，用于在请求前明确确认可能计费的产品。热带气旋和潮汐使用 `--allow-product marine`，太阳辐射使用 `--allow-product solar`。敏感的账户输出使用 `--allow-sensitive-output account`：

```sh
qweather marine tide \
  --tide-station-id <station-id> \
  --date <YYYY-MM-DD> \
  --allow-product marine

qweather account finance --allow-sensitive-output account
```

这些确认参数适合自动化环境，不会触发交互式提示。潮汐日期必须是 UTC 今天至未来 9 天内。热带气旋、潮汐和太阳辐射能力可能产生费用或没有免费额度，请先阅读 QWeather 当前定价和产品条款。

展示或再利用 QWeather 数据时，请保留所需的 Provider、来源和归属信息。参见 [QWeather pricing](https://dev.qweather.com/docs/finance/pricing/)、[Attribution](https://dev.qweather.com/docs/terms/attribution/) 和 [Developers terms](https://dev.qweather.com/docs/terms/)。

## 设计与文档

- [CLI contract](docs/design/cli-contract.md)：命令、参数、输出和退出码契约；
- [Architecture](docs/design/architecture.md)：模块边界和请求流；
- [Runtime and distribution](docs/design/runtime-and-distribution.md)：配置、认证、缓存、Skill 和分发；
- [CONTEXT.md](CONTEXT.md)：领域术语和项目约束；
- [Architecture decision records](docs/adr/)：已接受的架构决策。

## 许可证与免责声明

项目代码以 [Apache License 2.0](LICENSE) 发布。

本项目不授予 QWeather API 服务、QWeather 数据、QWeather 商标或第三方文档内容的额外权利。使用服务和数据前，请遵守 QWeather Developers EULA、适用的 API 条款、定价和归属要求。
