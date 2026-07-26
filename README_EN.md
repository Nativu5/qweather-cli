# QWeather CLI

An agent-friendly and human-friendly command-line client for QWeather.

[简体中文](README.md) | English

> This is an unofficial QWeather client. It is not affiliated with, sponsored by, or endorsed by QWeather or its affiliates. Project code is licensed under Apache License 2.0; access to QWeather services, use of QWeather data, attribution, and trademark use remain subject to the applicable QWeather terms.

## Feature overview

The CLI uses a curated, project-owned capability catalog. It does not expose upstream URLs or arbitrary provider request fields. The current catalog contains 28 executable capabilities:

- Geo: city lookup, top cities, POI lookup, and nearby POIs;
- Weather: city/grid current, daily, and hourly weather, historical weather, indices, and minutely precipitation;
- Alerts: current weather alerts;
- Air: current, daily, hourly, and station air quality;
- Tropical cyclones: list, track, and forecast;
- Marine: tide forecasts;
- Solar radiation: forecasts;
- Astronomy: sunrise/sunset, moon events, and solar position;
- Account: finance summary and request statistics.

Run `qweather --help` or `qweather capability list` to inspect the current command tree. Deprecated upstream operations remain lifecycle records and never become executable CLI commands.

## Installation

### Download a binary manually

When a Release is available, download the archive for your operating system and architecture from [GitHub Releases](https://github.com/Nativu5/qweather-cli/releases). Each published version includes `checksums.txt`, which you can use to verify the archive's SHA256. Extract the archive and place `qweather` (`qweather.exe` on Windows) in a directory on your `PATH`.

Published versions will provide `arm64` and `amd64` binaries for macOS, Linux, and Windows.

### Build from source

Use a Go toolchain compatible with the repository's `go.mod`:

```sh
go run ./cmd/qweather --help
go build -o qweather ./cmd/qweather
```

## Configuration and authentication

The CLI supports Ed25519 JWT (recommended) and API KEY authentication. Do not put credentials in command-line arguments, commit them to Git, or paste them into public Issues. Save the configuration as a TOML file and select it with `--config`; this example reads an API KEY from an environment variable:

```toml
[profiles.default]
api_host = "YOUR_ACCOUNT_API_HOST"
auth = "api_key"
api_key_env = "QWEATHER_API_KEY"
language = "auto"
unit = "metric"
```

Validate configuration without making a QWeather API request:

```sh
qweather config check --config /path/to/config.toml
```

The API Host must be the account-specific HTTPS host. See the official [QWeather authentication](https://dev.qweather.com/docs/configuration/authentication/), [API Host](https://dev.qweather.com/docs/configuration/api-host/), and [API request configuration](https://dev.qweather.com/docs/configuration/api-config/) documentation.

## Basic usage

The default output is a deterministic Text View intended for people:

```sh
qweather weather city current --place "Shanghai"
qweather weather city daily --place-id <location-id> --days 3
qweather air current --coordinate geo:31.2304,121.4737
qweather geo city lookup --query "Shanghai" --limit 5
```

Use `geo:<latitude>,<longitude>` for coordinates, with latitude first and longitude second. If a place name is ambiguous, the CLI reports the candidates and requires an explicit choice. Geo data is used only for the current invocation and is never persisted.

Select JSON explicitly for automation:

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

## Billable capabilities and attribution

`--allow-product` is a real CLI option that explicitly acknowledges a potentially billable product before the request. Tropical cyclone and tide commands use `--allow-product marine`; solar radiation uses `--allow-product solar`. Sensitive Account output uses `--allow-sensitive-output account`:

```sh
qweather marine tide \
  --tide-station-id <station-id> \
  --date <YYYY-MM-DD> \
  --allow-product marine

qweather account finance --allow-sensitive-output account
```

These acknowledgement flags work in automation and do not trigger an interactive prompt. The tide date must be between today in UTC and nine days from today. Tropical cyclone, tide, and solar radiation capabilities may be billable or have no free allowance; read the current QWeather pricing and product terms first.

When QWeather data is displayed or reused, preserve the required provider, source, and attribution information. See [QWeather pricing](https://dev.qweather.com/docs/finance/pricing/), [Attribution](https://dev.qweather.com/docs/terms/attribution/), and the [Developers terms](https://dev.qweather.com/docs/terms/).

## Design and documentation

- [CLI contract](docs/design/cli-contract.md): command, flag, output, and exit-code contract;
- [Architecture](docs/design/architecture.md): module boundaries and request flow;
- [Runtime and distribution](docs/design/runtime-and-distribution.md): configuration, authentication, cache, Skill, and distribution;
- [CONTEXT.md](CONTEXT.md): domain vocabulary and project constraints;
- [Architecture decision records](docs/adr/): accepted architectural decisions.

## License and disclaimer

Project code is released under the [Apache License 2.0](LICENSE).

This project grants no additional rights to QWeather API services, QWeather data, QWeather trademarks, or third-party documentation. Use the service and data in accordance with the QWeather Developers EULA and applicable API, pricing, and attribution requirements.
