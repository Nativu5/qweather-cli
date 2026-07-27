# QWeather CLI contract

Status: accepted design

Schema version: `qweather.result/v1` and `qweather.problem/v1`

Last verified against QWeather documentation: 2026-07-22

This document defines the public process interface of `qweather`. Within a released major version, command paths, required flags, Machine Result and Machine Problem meaning, and exit-code meaning remain compatible. Text View wording and layout may improve because Text is not a machine parsing contract.

## Interface principles

- Business domains are direct Cobra subcommands; there is no generic `fetch` layer.
- Every executable leaf represents one Current Capability and one current QWeather endpoint.
- Capability IDs are stable machine identifiers used by the registry, catalog, output, tests, and generated Skill references.
- Upstream versions such as `/v7` and provider parameter names are implementation details.
- Target kinds are explicit. Location IDs, coordinates, station IDs, and storm IDs are never inferred from opaque string shape.
- There is no arbitrary path, method, query, host, or header passthrough.
- Text is the default readable presentation. Callers select JSON explicitly when they require a versioned machine interface.

## Command tree

```text
qweather
├── geo
│   ├── city
│   │   ├── lookup
│   │   └── top
│   └── poi
│       ├── lookup
│       └── nearby
├── weather
│   ├── city
│   │   ├── current
│   │   ├── daily
│   │   └── hourly
│   ├── grid
│   │   ├── current
│   │   ├── daily
│   │   └── hourly
│   ├── minutely
│   ├── indices
│   └── history
├── alert
│   └── current
├── air
│   ├── current
│   ├── daily
│   ├── hourly
│   └── station
├── storm
│   ├── list
│   ├── track
│   └── forecast
├── marine
│   └── tide
├── solar
│   └── forecast
├── astronomy
│   ├── sun
│   ├── moon
│   └── position
├── account
│   ├── finance
│   └── usage
├── capability
│   ├── list
│   └── show
├── cache
│   ├── status
│   └── clear
├── config
│   └── check
└── version
```

The first nine branches contain 28 network capabilities. `capability`, `cache`, `config`, and `version` are local control commands and do not count toward the provider surface.

## Common execution flags

| Flag | Meaning |
| --- | --- |
| `--config <path>` | Select a TOML file. This is not a secret flag. |
| `--profile <name>` | Select one configured profile. |
| `--timeout <duration>` | Set the invocation deadline; default `10s`. |
| `--output text\|json\|body` | Select the invocation presentation; default `text`. `body` is valid only for provider query commands. |
| `--refresh` | Skip cache read and replace the entry after a successful request. |
| `--no-cache` | Skip both cache read and cache write. |
| `--debug` | Emit secret-free diagnostics on stderr. |

Authentication secrets, private-key contents, API KEYs, and complete JWTs never have command-line flags.

## Target grammar

### Place-aware commands

A place-aware command accepts exactly one of:

| Flag | Public form | Behaviour |
| --- | --- | --- |
| `--place <name>` | Human text | Resolve through GeoAPI. Continue only when the result is unambiguous. |
| `--place-id <id>` | QWeather Location ID | Use directly when supported; resolve only if the endpoint requires coordinates. |
| `--coordinate geo:<lat>,<lon>` | RFC 5870 order | Use directly when supported; resolve only if the endpoint requires a Location ID. |

`--country <ISO-3166>` and `--adm <text>` are valid only with `--place`. A coordinate must use the `geo:` prefix, latitude first, longitude second, and values accepted by QWeather's documented precision. The HTTP adapter owns any conversion to provider `lon,lat` queries or `/{latitude}/{longitude}` paths.

In the coverage tables, `coordinate target` means a place-aware command whose Resolved Place must contain coordinates; `Location ID target` means one whose Resolved Place must contain a Location ID. Both still accept the common Place Spec flags unless the row names a domain-specific ID.

Place resolution follows these rules:

1. Pass country and administrative filters to `geo.city.lookup`.
2. Continue when there is exactly one normalized exact-name candidate, or when the provider returns exactly one candidate.
3. Never select by response order or rank.
4. Return `AMBIGUOUS_PLACE` with safe candidate references when several candidates remain.
5. Return `PLACE_NOT_FOUND` when no candidate remains.
6. Consume Geo Data only in the current invocation and never persist it.

### Domain-specific targets

The following flags are not interchangeable:

- `--air-station-id` for `air station`;
- `--tide-station-id` for `marine tide`;
- `--storm-id` for `storm track/forecast`.

## Capability coverage

### Geo — 4

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `geo city lookup` | `geo.city.lookup` | exactly one of `--query`, `--place-id`, `--coordinate` | [`GET /geo/v2/city/lookup`](https://dev.qweather.com/docs/api/geoapi/city-lookup/) |
| `geo city top` | `geo.city.top` | none | [`GET /geo/v2/city/top`](https://dev.qweather.com/docs/api/geoapi/top-city/) |
| `geo poi lookup` | `geo.poi.lookup` | `--query` and `--poi-type` | [`GET /geo/v2/poi/lookup`](https://dev.qweather.com/docs/api/geoapi/poi-lookup/) |
| `geo poi nearby` | `geo.poi.nearby` | `--coordinate` and `--poi-type` | [`GET /geo/v2/poi/range`](https://dev.qweather.com/docs/api/geoapi/poi-range/) |

Common optional Geo flags are `--country`, `--adm`, `--limit 1..20`, and `--lang` when the endpoint supports them. Stable POI kinds are `scenic` and `tide-station`. The upstream OpenAPI-only `CSTA` value is not a stable v0.1 command option because the rendered official documentation does not currently describe it.

### Weather — 9

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `weather city current` | `weather.city.current` | Place Spec | [`GET /v7/weather/now`](https://dev.qweather.com/docs/api/weather/weather-now/) |
| `weather city daily` | `weather.city.forecast.daily` | Place Spec; `--days 3\|7\|10\|15\|30` | [`GET /v7/weather/{days}`](https://dev.qweather.com/docs/api/weather/weather-daily-forecast/) |
| `weather city hourly` | `weather.city.forecast.hourly` | Place Spec; `--hours 24\|72\|168` | [`GET /v7/weather/{hours}`](https://dev.qweather.com/docs/api/weather/weather-hourly-forecast/) |
| `weather grid current` | `weather.grid.current` | Place Spec resolving to coordinates | [`GET /v7/grid-weather/now`](https://dev.qweather.com/docs/api/weather/grid-weather-now/) |
| `weather grid daily` | `weather.grid.forecast.daily` | coordinate target; `--days 3\|7` | [`GET /v7/grid-weather/{days}`](https://dev.qweather.com/docs/api/weather/grid-weather-daily-forecast/) |
| `weather grid hourly` | `weather.grid.forecast.hourly` | coordinate target; `--hours 24\|72` | [`GET /v7/grid-weather/{hours}`](https://dev.qweather.com/docs/api/weather/grid-weather-hourly-forecast/) |
| `weather minutely` | `weather.precipitation.minutely` | coordinate target | [`GET /v7/minutely/5m`](https://dev.qweather.com/docs/api/minutely/minutely-precipitation/) |
| `weather indices` | `weather.indices.forecast` | Place Spec; `--days 1\|3`; repeated `--index` or `--all-indices` | [`GET /v7/indices/{days}`](https://dev.qweather.com/docs/api/indices/indices-forecast/) |
| `weather history` | `weather.history` | target resolving to Location ID; `--date YYYY-MM-DD` | [`GET /v7/historical/weather`](https://dev.qweather.com/docs/api/time-machine/time-machine-weather/) |

`--unit metric|imperial` is exposed only on grid and history commands. `--lang` is exposed only where the endpoint documents language support. `--all-indices` is mutually exclusive with individual index values.

### Alert — 1

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `alert current` | `alert.current` | coordinate target | [`GET /weatheralert/v1/current/{latitude}/{longitude}`](https://dev.qweather.com/docs/api/warning/weather-alert/) |

Optional flags are `--local-time` and `--lang`. An empty alert set is a valid success.

### Air quality — 4

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `air current` | `air.current` | coordinate target | [`GET /airquality/v1/current/{latitude}/{longitude}`](https://dev.qweather.com/docs/api/air-quality/air-current/) |
| `air daily` | `air.forecast.daily` | coordinate target | [`GET /airquality/v1/daily/{latitude}/{longitude}`](https://dev.qweather.com/docs/api/air-quality/air-daily-forecast/) |
| `air hourly` | `air.forecast.hourly` | coordinate target | [`GET /airquality/v1/hourly/{latitude}/{longitude}`](https://dev.qweather.com/docs/api/air-quality/air-hourly-forecast/) |
| `air station` | `air.station.current` | `--air-station-id` | [`GET /airquality/v1/stations/{locationId}`](https://dev.qweather.com/docs/api/air-quality/air-station/) |

All four commands preserve provider Attribution and do not collapse multiple AQI standards into one number.

### Storm — 3

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `storm list` | `storm.list` | `--year` (current or previous UTC calendar year) | [`GET /v7/tropical/storm-list`](https://dev.qweather.com/docs/api/tropical-cyclone/storm-list/) |
| `storm track` | `storm.track` | `--storm-id` | [`GET /v7/tropical/storm-track`](https://dev.qweather.com/docs/api/tropical-cyclone/storm-track/) |
| `storm forecast` | `storm.forecast` | `--storm-id` | [`GET /v7/tropical/storm-forecast`](https://dev.qweather.com/docs/api/tropical-cyclone/storm-forecast/) |

All storm commands require `--allow-product marine` before any network I/O. The current provider basin is fixed to `NP`; the CLI does not expose unsupported basin flexibility.

### Marine — 1

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `marine tide` | `marine.tide` | `--tide-station-id`; `--date YYYY-MM-DD` from UTC today through `+9` days | [`GET /v7/ocean/tide`](https://dev.qweather.com/docs/api/ocean/tide/) |

The command requires `--allow-product marine` before any network I/O.

### Solar radiation — 1

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `solar forecast` | `solar.radiation.forecast` | coordinate target | [`GET /solarradiation/v1/forecast/{latitude}/{longitude}`](https://dev.qweather.com/docs/api/solar-radiation/solar-radiation-forecast/) |

Optional flags include `--hours 1..60`, `--interval-min 15|30|60`, repeated `--include weather|poa`, integer `--tilt-deg 0..90`, integer `--azimuth-deg 0..359`, and `--local-time`. Hours default to 24 and the interval defaults to 60 minutes. Including `poa` requires both tilt and azimuth. The command requires `--allow-product solar` before any network I/O.

### Astronomy — 3

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `astronomy sun` | `astronomy.sun.events` | Place Spec; `--date YYYY-MM-DD` from UTC today through `+59` days | [`GET /v7/astronomy/sun`](https://dev.qweather.com/docs/api/astronomy/sunrise-sunset/) |
| `astronomy moon` | `astronomy.moon.events` | Place Spec; `--date YYYY-MM-DD` from UTC today through `+59` days | [`GET /v7/astronomy/moon`](https://dev.qweather.com/docs/api/astronomy/moon-and-moon-phase/) |
| `astronomy position` | `astronomy.solar.position` | coordinate target; `--at RFC3339`; `--altitude-m` | [`GET /v7/astronomy/solar-elevation-angle`](https://dev.qweather.com/docs/api/astronomy/solar-elevation-angle/) |

The position command converts one RFC3339 timestamp into the provider's coupled date, time, and timezone parameters.

### Account — 2

| CLI path | Capability ID | Required typed input | Upstream |
| --- | --- | --- | --- |
| `account finance` | `account.finance.summary` | none | [`GET /finance/v1/summary`](https://dev.qweather.com/docs/api/console/finance/) |
| `account usage` | `account.requests.stats` | optional, mutually exclusive `--project-id` or `--credential-id` | [`GET /metrics/v1/stats`](https://dev.qweather.com/docs/api/console/stats/) |

Both commands require `--allow-sensitive-output account` before network I/O. They are never used as configuration or authentication probes.

The coverage count is `4 + 9 + 1 + 4 + 3 + 1 + 1 + 3 + 2 = 28`.

## Lifecycle catalog

The five endpoints already deprecated at design time have no executable command. `capability list --lifecycle all` and `capability show` expose read-only Tombstones:

| Tombstone | Upstream | Replacement |
| --- | --- | --- |
| `legacy.alert.current` | `/v7/warning/now` | `alert.current` |
| `legacy.air.current` | `/v7/air/now` | `air.current` |
| `legacy.air.forecast.daily` | `/v7/air/5d` | `air.forecast.daily` |
| `legacy.solar.radiation.forecast` | `/v7/solar-radiation/{hours}` | `solar.radiation.forecast` |
| `legacy.air.history` | `/v7/historical/air` | none documented |

Tombstones cannot be activated by a flag and are never accepted by a generic dispatcher.

## Local control commands

### Capability discovery

```text
qweather capability list [--domain <name>] [--billing-group basic|marine|solar]
                           [--lifecycle current|deprecated|all]
qweather capability show <capability-id>
```

These commands are offline and read the same compiled registry used for execution. Their Text and JSON forms use the global `--output`; command-local format flags do not exist.

### Configuration

`qweather config check` runs the production loader and validator without creating a QWeather request. It may report whether a secret source is present, but never the secret value. When the default file is absent and no provider or authentication environment variables are present, it returns exit 3 with `CONFIG_INVALID` and reports that QWeather is not configured; it never creates an empty configuration file. A complete environment-only configuration remains valid without a file.

### Cache

`qweather cache status` reports entry count, bytes, expiration bounds, and policy without revealing targets or raw cache keys. `qweather cache clear` clears the selected profile; `--capability` narrows the operation, and `--all-profiles` is an explicit broader action.

### Version

`qweather version` prints the synchronized semantic version in Text. `qweather version --output json` also returns Go version, commit, build time, and registry hash.

All local control commands support `--output text` and `--output json`. They reject `--output body` as an exit-2 invocation error because no Provider Body exists.

## Output modes

One global option controls QWeather-owned success and failure presentation:

| Mode | Successful provider command | Successful local command | QWeather-owned failure |
| --- | --- | --- | --- |
| `text` (default) | Text View | command-specific readable Text | Text Problem |
| `json` | Machine Result | compact command-specific JSON | Machine Problem |
| `body` | exact successful Provider Body bytes | exit 2 | Text Problem |

Successful output is written only to stdout. Failures write no stdout data. Output mode is presentation state, never part of provider request semantics or cache identity. There is no pretty-print flag: JSON is compact and callers may use `jq`; Text owns readability.

Cobra command-discovery, flag-parsing, and positional-argument diagnostics always use Cobra Text, even when `--output json` was present. Output mode applies after Cobra has selected and parsed a command; the CLI does not parse or wrap Cobra's diagnostic strings.

### Text View

Each of the 28 Current Capabilities selects one embedded, manually maintained entry template by Capability ID. Templates may share partials but cannot be loaded from configuration, a user path, the network, or OpenAPI at runtime. Text uses fixed English labels, does not inspect TTY state or terminal width, and contains no colour or adaptive layout. It is deterministic for one CLI version but is not a stable machine-parsing surface.

A Capability template controls the primary reading order. Every provider field that it does not consume is rendered once under `Additional fields`. Object keys sort lexically; arrays preserve provider order and are never truncated. The renderer preserves provider strings, dates, timestamps, numeric precision, JSON value types, empty values, and array contents. It performs no time conversion, numeric normalization, provider-content sanitization, or value conversion. It may append only units that are known from a provider value or the invocation's effective unit; when a unit cannot be determined reliably, it does not guess. `--lang` affects provider-returned content only, not template labels.

The common Text renderer displays Capability, Resolved Place when present, Cache, logical Operations, and complete Attribution. For a `no_data` outcome it displays No Data as a common exit-0 Text result and does not execute an empty Capability layout. Provider `refer.sources`, `refer.license`, and `metadata.attributions` are marked consumed when Attribution is rendered, so they do not repeat under `Additional fields`.

Templates are compiled and checked with the command tree. Invalid template syntax or a missing entry template for a Current Capability is a broken internal invariant. If a valid template cannot consume an unexpected runtime provider shape, the renderer writes the complete data through its generic Text tree and keeps the successful outcome; a secret-free fallback diagnostic is written only under `--debug`. A renderer or stdout write failure is an `OUTPUT_ERROR`.

Text Problems begin with the problem message and include the symbolic code, Capability, retryable state, and complete safe details when present. Detail objects use deterministic ordering and arrays remain complete. Text Problem layout is readable presentation, not a machine schema.

### Machine Result

`--output json` writes one compact `qweather.result/v1` object followed by a newline:

```json
{
  "schema": "qweather.result/v1",
  "outcome": "ok",
  "capability": "weather.city.current",
  "resolvedPlace": {
    "id": "101010100",
    "name": "北京",
    "country": "中国"
  },
  "operations": [
    "geo.city.lookup",
    "weather.city.current"
  ],
  "policy": {
    "billingGroup": "basic"
  },
  "cache": {
    "status": "miss",
    "storedAt": "2026-07-23T00:00:00Z",
    "expiresAt": "2026-07-23T00:10:00Z",
    "ageSeconds": 0,
    "upstreamRequested": true
  },
  "upstream": {
    "httpStatus": 200,
    "responseFamily": "code-refer-v1"
  },
  "attribution": [],
  "data": {}
}
```

`data` contains the complete decoded provider JSON object without projecting its domain records into a CLI-owned weather schema. Unknown fields and provider `refer` or `metadata.attributions` content remain intact. `attribution` is a convenient extracted view and does not replace the provider fields in `data`.

`resolvedPlace` is omitted or partial when no Geo resolution occurred. `operations` lists the logical capabilities used to produce the result; the cache object records which data operation actually reached the provider.

`outcome` is `ok` or `no_data`. Provider-defined No Data is exit 0 and is never retried automatically.

Response Family names describe provider-envelope structure, not lifecycle:

- `code-refer-v1` uses the provider's string `code` and optional `refer` object;
- `metadata-v1` relies on HTTP status and uses provider `metadata`; and
- `console-v1` covers the Account console response shape.

Current Capabilities may use `code-refer-v1`; the name never implies deprecation. Only explicit Tombstones are non-executable.

### Provider Body

`--output body` is valid only for a successful provider query. It writes the exact Provider Body bytes without parsing, re-encoding, changing field order, or adding a trailing newline. It does not expose provider error bodies and does not expand the executable Capability surface. On any failure stdout remains empty and stderr receives a Text Problem or Cobra diagnostic.

## Machine Problem and exit codes

For a QWeather-owned failure under `--output json`, stderr receives one compact `qweather.problem/v1` object followed by a newline unless explicit debug mode adds preceding secret-free diagnostic events. Cobra invocation errors are the Text exception described above. Under `--output text` or `--output body`, QWeather-owned failures use the Text Problem presentation.

```json
{
  "schema": "qweather.problem/v1",
  "code": "AMBIGUOUS_PLACE",
  "message": "place matches multiple locations",
  "capability": "weather.city.current",
  "retryable": false,
  "details": {
    "candidates": []
  }
}
```

| Exit | Meaning |
| ---: | --- |
| `0` | `ok` or valid `no_data`, including cache hits |
| `2` | invocation syntax, typed input, mutual exclusion, or local validation |
| `3` | configuration, private key, or authentication configuration |
| `4` | required Product Gate was not acknowledged |
| `5` | place not found or ambiguous |
| `6` | permanent upstream rejection, permission, debt, credit, or invalid request |
| `7` | rate or monthly limit |
| `8` | network, timeout, or upstream 5xx |
| `9` | malformed JSON, oversized body, or unrecognized protocol |
| `10` | broken internal invariant |

The symbolic `code` is the primary Machine Problem decision field. Exit codes provide stable coarse categories for shells and Agents in every output mode.

## Language and units

- `--lang` overrides the profile language; `auto` means omit the provider parameter.
- The Skill normally passes the current conversation language when the capability supports it.
- Indices and minutely precipitation are limited to Chinese and English according to the [language documentation](https://dev.qweather.com/docs/resource/language/).
- The default unit is metric. `--unit metric|imperial` maps to provider `m|i` only on capabilities whose current endpoint schema documents the option.
- The CLI does not convert provider values.
- Effective language and unit are part of the cache key.

## Compatibility

Go binary, npm adapter, and generated Skill references share one semantic version. Within a released major version:

- adding a capability or optional flag is additive;
- command removal or rename, a new required input, Machine Result or Machine Problem meaning changes, or exit-code meaning changes require a major release;
- Text labels, spacing, field grouping, and ordering may improve without a major release, but Text must remain deterministic, complete, and consistent with the selected Capability;
- Provider Body content is owned by QWeather; the CLI promises only successful byte-exact passthrough; and
- patch releases may fix implementation defects without changing the stable machine interface.
