# QWeather CLI

An agent-friendly and human-friendly command-line client for QWeather.

[中文说明](README.md)

> This is an unofficial QWeather client. It is not affiliated with, sponsored by, or endorsed by QWeather or its affiliates. Project code is licensed under Apache License 2.0; access to QWeather services, use of QWeather data, attribution, and trademark use remain subject to the applicable QWeather terms.

## Project status

QWeather CLI is preparing its first public release and npm distribution adapter. The npm package and Release binaries are not available until that release is published; the source, design documents, and tests are available for review and development.

## Capabilities

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

After the first public Release, install a fixed npm version:

```sh
npm install --global qweather-cli@<version>
```

Project-local installation and `npx` are also supported:

```sh
npm install --save-dev qweather-cli@<version>
npx --package=qweather-cli@<version> qweather --help
```

The installer selects the same-version public GitHub Release asset for `process.platform` and `process.arch`, then verifies its SHA256. Normal `qweather` execution never downloads, compiles, updates, or repairs the binary.

If npm lifecycle scripts were disabled, repair a global install with:

```sh
npm rebuild -g qweather-cli
```

For a project-local install, run this from the project directory:

```sh
npm rebuild qweather-cli
```

Unsupported platforms fail clearly and never fall back to compiling Go source.

## Run from source

Use a Go toolchain compatible with the repository's `go.mod`:

```sh
go run ./cmd/qweather --help
go build -o qweather ./cmd/qweather
```

Maintainers can run deterministic checks with the Makefile:

```sh
make check
```

Normal tests never call the live QWeather API. Credentialed, quota-consuming smoke tests run only through the protected Release process.

## Configuration and authentication

The default configuration file is `qweather/config.toml` under the directory returned by Go's `os.UserConfigDir()`. On Linux this is usually `${XDG_CONFIG_HOME:-~/.config}/qweather/config.toml`; macOS and Windows use their respective system user-config directories.

Linux example:

```text
${XDG_CONFIG_HOME:-~/.config}/qweather/config.toml
```

The CLI supports Ed25519 JWT (recommended) and API KEY authentication. Do not put credentials in command-line arguments, commit them to Git, or paste them into public Issues. Example configuration using an environment variable for an API KEY:

```toml
[profiles.default]
api_host = "YOUR_ACCOUNT_API_HOST"
auth = "api_key"
api_key_env = "QWEATHER_API_KEY"
language = "auto"
unit = "metric"
```

Validate configuration without making a provider request:

```sh
qweather config check
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

## Product acknowledgements and attribution

Storm (tropical cyclone), Marine, Solar, and sensitive Account capabilities require explicit acknowledgement before network I/O. The three Storm commands and Marine use `--allow-product marine`; Solar uses `--allow-product solar`:

```sh
qweather marine tide \
  --tide-station-id <station-id> \
  --date <YYYY-MM-DD> \
  --allow-product marine

qweather account finance --allow-sensitive-output account
```

Acknowledgements are explicit flags rather than interactive prompts, so they work in automation. The Marine tide date must be between today in UTC and nine days from today. Marine, Solar, and Storm capabilities may be billable or have no free allowance; read the current QWeather pricing and product terms first.

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
