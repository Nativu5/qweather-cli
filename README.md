# QWeather CLI

一个面向人类和智能代理的 QWeather 命令行客户端。

[English version](README_EN.md)

> 本项目是非官方的 QWeather 客户端，与 QWeather 或其关联方没有隶属、赞助或背书关系。项目代码采用 Apache License 2.0；QWeather API 的访问、数据使用、归属声明和商标使用仍受 QWeather 适用条款约束。

## 项目状态

QWeather CLI 正在完善首个公开版本和 npm 分发适配器。发布 npm 包和 Release 二进制之前，安装命令暂不可用；源码、设计文档和测试可以用于审阅和开发。

## 能做什么

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

首个公开 Release 发布后，推荐使用固定版本安装 npm 包：

```sh
npm install --global qweather-cli@<version>
```

也支持项目本地安装和 `npx`：

```sh
npm install --save-dev qweather-cli@<version>
npx --package=qweather-cli@<version> qweather --help
```

安装过程会根据 `process.platform` 和 `process.arch` 选择同版本的公开 GitHub Release 二进制，并校验 SHA256。正常运行 `qweather` 时不会自动下载、编译、更新或修复二进制。

如果 npm lifecycle scripts 被禁用，全局安装可运行：

```sh
npm rebuild -g qweather-cli
```

本地安装则在项目目录运行：

```sh
npm rebuild qweather-cli
```

不支持的平台不会回退到源码编译。

## 从源码运行

需要与仓库 `go.mod` 兼容的 Go 工具链。开发时可以直接运行：

```sh
go run ./cmd/qweather --help
go build -o qweather ./cmd/qweather
```

维护者可以使用 Makefile 运行确定性检查：

```sh
make check
```

正常测试不会访问真实 QWeather API；需要配额和凭据的 smoke test 仅在受保护的 Release 流程中执行。

## 配置与认证

配置文件默认位于 Go `os.UserConfigDir()` 返回目录下的 `qweather/config.toml`。在 Linux 上通常是 `${XDG_CONFIG_HOME:-~/.config}/qweather/config.toml`；macOS 和 Windows 使用各自的系统用户配置目录。

```text
${XDG_CONFIG_HOME:-~/.config}/qweather/config.toml
```

CLI 支持 Ed25519 JWT（推荐）和 API KEY。凭据不要写入命令行参数、提交到 Git 或放入公开 Issue。一个使用环境变量保存 API KEY 的配置示例：

```toml
[profiles.default]
api_host = "YOUR_ACCOUNT_API_HOST"
auth = "api_key"
api_key_env = "QWEATHER_API_KEY"
language = "auto"
unit = "metric"
```

先检查配置，不会发起 provider 请求：

```sh
qweather config check
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

## 产品确认和数据归属

Storm（热带气旋）、Marine、Solar 和敏感 Account 能力在网络请求前要求显式确认。三个 Storm 命令和 Marine 使用 `--allow-product marine`；Solar 使用 `--allow-product solar`：

```sh
qweather marine tide \
  --tide-station-id <station-id> \
  --date 2026-01-01 \
  --allow-product marine

qweather account finance --allow-sensitive-output account
```

确认不是交互式提示；它必须作为明确的命令参数提供，适合自动化环境。Marine、Solar 和部分 Storm 能力可能产生费用或没有免费额度，请先阅读 QWeather 当前定价和产品条款。

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
