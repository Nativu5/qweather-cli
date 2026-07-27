# 和风天气 QWeather CLI

简体中文 | [English](README_EN.md)

面向 Agent 的和风天气（QWeather）Skill & 命令行客户端。

> 本项目是非官方的和风天气 CLI 实现，与和风天气或其关联方没有隶属、赞助或背书关系。项目代码采用 Apache License 2.0。和风天气 API 的访问、数据使用、归属声明和商标使用仍受相关适用条款约束。

## 功能概览

QWeather CLI 提供以下数据查询和账户管理功能：

- 地理：城市查询、热门城市、POI 查询和附近 POI；
- 天气：城市/网格实况、逐日、逐小时、历史、生活指数和分钟降水；
- 预警：当前天气预警；
- 空气：当前、逐日、逐小时和监测站空气质量；
- 热带气旋：列表、路径和预报；
- 海洋：潮汐预报；
- 太阳辐射：辐射预报；
- 天文：日出日落、月相和太阳位置；
- 账户：财务摘要和请求统计。

运行 `qweather --help` 查看命令和参数。

## 安装

### 使用 npm 安装

> [!TIP]
> 可直接将此页面 URL 发送给您的 Agent。

安装供 Agent 使用的 `qweather` Skill 和 CLI：

```sh
npx skills add Nativu5/qweather-cli
npm install --global qweather-cli@0.1.0
```

第一条命令将 Skill 安装到 `npx skills` 自动识别的 Agent 环境中，第二条命令安装 `qweather` CLI 二进制。两条命令都不会读取或写入 QWeather 凭据。

### 手动下载二进制

从 [GitHub Releases](https://github.com/Nativu5/qweather-cli/releases) 下载适合操作系统和架构的压缩包。

解压后，将 `qweather`（Windows 为 `qweather.exe`）放入 `PATH` 中的目录。发布版本提供 macOS、Linux 和 Windows 的 `arm64`、`amd64` 二进制。

需要 Skill 时，可再执行上面的 `npx skills add` 命令，或手动将仓库 `skills` 目录下的内容放置到需要的位置。

### 从源代码构建

需要与仓库 `go.mod` 兼容的 Go 工具链：

```sh
go run ./cmd/qweather --help
go build -o qweather ./cmd/qweather
```

## 配置与认证

QWeather 支持 Ed25519 JWT（推荐）和 API KEY 两种鉴权方式。可使用环境变量或将配置保存为 TOML 文件，并使用 `--config` 指定。

- 环境变量示例：

```
# 如使用 API KEY 鉴权
export QWEATHER_API_HOST="YOUR_ACCOUNT_API_HOST"
export QWEATHER_API_KEY="YOUR_ACCOUNT_API_KEY"
```

- 配置文件示例：

可复制 Skill 中的 [config.toml](skills/qweather/config.toml) 作为参考模板。模板默认使用 API KEY，也包含注释形式的 Ed25519 JWT 配置；请根据注释选择一种鉴权方式，并替换其中的占位符。

- 配置检查（不会发起实际 API 请求）：

```sh
qweather config check --config /path/to/config.toml
```

API Host 必须是账户对应的 HTTPS host。认证、API Host 和 JWT 的官方说明见 [QWeather authentication](https://dev.qweather.com/docs/configuration/authentication/)、[API Host](https://dev.qweather.com/docs/configuration/api-host/) 和 [API request configuration](https://dev.qweather.com/docs/configuration/api-config/)。

## 基本使用

默认输出是适合人类阅读的 Text View：

```sh
qweather weather city current --place "Shanghai"
qweather weather city daily --place-id <location-id> --days 3
qweather air current --coordinate geo:31.2304,121.4737
qweather geo city lookup --query "Shanghai" --limit 5
```

> [!TIP]
> 使用 `geo:<latitude>,<longitude>` 表示坐标，纬度在前、经度在后。地点名称解析如果产生多个候选，CLI 会报告歧义并要求调用方明确选择。

机器处理时建议选择 JSON：

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

> [!TIP]
> 根据 QWeather 文档要求，GEO 数据不会被缓存。

## 付费 API

QWeather 按产品计费，部分产品没有免费额度。根据 QWeather 当前计费规则，CLI 会要求在请求前通过命令行明确确认这些产品：热带气旋和潮汐使用 `--allow-product marine`，太阳辐射使用 `--allow-product solar`。

```sh
qweather marine tide \
  --tide-station-id <station-id> \
  --date <YYYY-MM-DD> \
  --allow-product marine

qweather solar forecast \
  --coordinate geo:31.2304,121.4737 \
  --allow-product solar
```

这些参数不会触发交互式提示，适合自动化环境。潮汐日期必须是 UTC 今天至未来 9 天内。具体收费标准以官方文档 [QWeather pricing](https://dev.qweather.com/docs/finance/pricing/) 为准。

## 设计文档

- [CLI contract](docs/design/cli-contract.md)：命令、参数、输出和退出码契约；
- [Architecture](docs/design/architecture.md)：模块边界和请求流；
- [Runtime and distribution](docs/design/runtime-and-distribution.md)：配置、认证、缓存、Skill 和分发；
- [CONTEXT.md](CONTEXT.md)：领域术语和项目约束；
- [Architecture decision records](docs/adr/)：已接受的架构决策。

## 许可证

项目代码以 [Apache License 2.0](LICENSE) 发布。

本项目不授予和风天气 API 服务、数据、商标或第三方文档内容的额外权利。使用服务和数据前，请遵守 QWeather Developers EULA、适用的 API 条款、定价等要求。
