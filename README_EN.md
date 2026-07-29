# QWeather CLI

QWeather Skill and command-line client for agents.

[简体中文](README.md) | English

> This is an unofficial QWeather CLI implementation. It is not affiliated with, sponsored by, or endorsed by QWeather or its affiliates. Project code is licensed under Apache License 2.0. Access to QWeather services, use of QWeather data, attribution, and trademark use remain subject to the applicable terms.

## Feature overview

QWeather CLI provides the following data-query and account-management features:

- Geo: city lookup, top cities, POI lookup, and nearby POIs;
- Weather: city/grid current, daily, and hourly weather, historical weather, indices, and minutely precipitation;
- Alerts: current weather alerts;
- Air: current, daily, hourly, and station air quality;
- Tropical cyclones: list, track, and forecast;
- Marine: tide forecasts;
- Solar radiation: forecasts;
- Astronomy: sunrise/sunset, moon events, and solar position;
- Account: finance summary and request statistics.

Run `qweather --help` to see the available commands and flags.

## Installation

### Install with npm

> [!TIP]
> You can send this page URL directly to your Agent.

Install the `qweather` Skill for your Agent and the CLI binary:

```sh
npx skills add Nativu5/qweather-cli
npm install --global qweather-cli
```

The first command installs the Skill into the Agent environment detected by `npx skills`; the second installs the `qweather` CLI binary. Neither command reads or writes QWeather credentials.

### Download a binary manually

Download the archive for your operating system and architecture from [GitHub Releases](https://github.com/Nativu5/qweather-cli/releases).

Extract it and place `qweather` (`qweather.exe` on Windows) in a directory on your `PATH`. Releases provide `arm64` and `amd64` binaries for macOS, Linux, and Windows.

If you also need the Skill, run the `npx skills add` command above, or place the contents of the repository's `skills` directory manually where your Agent expects them.

### Build from source

Use a Go toolchain compatible with the repository's `go.mod`:

```sh
go run ./cmd/qweather --help
go build -o qweather ./cmd/qweather
```

## Configuration and authentication

QWeather supports Ed25519 JWT (recommended) and API KEY authentication. You can use environment variables or save configuration as TOML and select it with `--config`.

- Environment variable example:

```sh
# For API KEY authentication
export QWEATHER_API_HOST="YOUR_ACCOUNT_API_HOST"
export QWEATHER_API_KEY="YOUR_ACCOUNT_API_KEY"
```

- Configuration file example:

Copy the [config.toml](skills/qweather/config.toml) template from the Skill. It uses API KEY authentication by default and includes a commented Ed25519 JWT alternative. Follow its comments to choose exactly one authentication method and replace the placeholders.

Validate the configuration without making an actual API request:

```sh
qweather config check --config /path/to/config.toml
```

The API Host must be the account-specific HTTPS host. See the official [QWeather authentication](https://dev.qweather.com/docs/configuration/authentication/), [API Host](https://dev.qweather.com/docs/configuration/api-host/), and [API request configuration](https://dev.qweather.com/docs/configuration/api-config/) documentation.

## Basic usage

The default output is a Text View intended for people:

```sh
qweather weather city current --place "Shanghai"
qweather weather city daily --place-id <location-id> --days 3
qweather air current --coordinate geo:31.2304,121.4737
qweather geo city lookup --query "Shanghai" --limit 5
```

> [!TIP]
> Use `geo:<latitude>,<longitude>` for coordinates, with latitude first and longitude second. If a place name is ambiguous, the CLI reports the candidates and requires an explicit choice.

Select JSON for automation:

```sh
qweather --output json weather city current --place-id <location-id>
qweather --output json version
```

Output modes are:

- `text`: the default deterministic Text View;
- `json`: versioned `qweather.result/v1` or `qweather.problem/v1` output;
- `body`: the successful raw QWeather Provider Body.

Control caching and refresh explicitly:

```sh
qweather --refresh weather city current --place-id <location-id>
qweather --no-cache --output json air current --coordinate geo:31.2304,121.4737
qweather cache status
qweather cache clear
```

> [!TIP]
> Geo data is not cached, as required by the QWeather documentation.

## Product Gates

QWeather bills by product, some products have no free allowance, and Account commands return sensitive account data. Tropical cyclone, tide, solar-radiation, and Account commands require the leaf-local `--yes` flag to acknowledge the current invocation.

```sh
qweather marine tide \
  --tide-station-id <station-id> \
  --date <YYYY-MM-DD> \
  --yes

qweather solar forecast \
  --coordinate geo:31.2304,121.4737 \
  --yes

qweather account usage \
  --yes
```

`--yes` acknowledges only the current command. It never triggers an interactive `y/N` prompt and cannot be configured or persisted. The tide date must be between today in UTC and nine days from today. See the official [QWeather pricing](https://dev.qweather.com/docs/finance/pricing/) documentation for current charges.

## Design documentation

- [CLI contract](docs/design/cli-contract.md): command, flag, output, and exit-code contract;
- [Architecture](docs/design/architecture.md): module boundaries and request flow;
- [Runtime and distribution](docs/design/runtime-and-distribution.md): configuration, authentication, cache, Skill, and distribution;
- [CONTEXT.md](CONTEXT.md): domain vocabulary and project constraints;
- [Architecture decision records](docs/adr/): accepted architectural decisions.

## License

Project code is released under the [Apache License 2.0](LICENSE).

This project grants no additional rights to QWeather API services, data, trademarks, or third-party documentation. Use the service and data in accordance with the QWeather Developers EULA and applicable API and pricing requirements.
