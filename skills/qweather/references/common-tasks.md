# Common qweather tasks

Last verified: 2026-07-27

Pass `--output text` for reading and `--output json` for automation. These
examples never embed credentials; configuration and authentication remain owned
by the CLI.

## Validate local setup

Before the first weather command, validate the same effective configuration
that the query will use:

```sh
qweather config check --output json
```

Require exit status 0 and `valid: true`. Inspect `effective.configPath` and the
secret-free `diagnostics.configSource`, `profileSource`, and `authSource`
fields instead of assuming which configuration won precedence. This preflight
is offline and consumes no QWeather API quota.

For a non-default file, either set `QWEATHER_CONFIG` for the session:

```sh
export QWEATHER_CONFIG=/path/to/config.toml
qweather config check --output json
qweather weather city current --place-id 101010100 --output text
```

or pass the same path to the preflight and every weather command:

```sh
qweather config check --config /path/to/config.toml --output json
qweather weather city current --place-id 101010100 --config /path/to/config.toml --output text
```

Do not validate one configuration source and then query with another. Continue
with offline version and Capability discovery as needed:

```sh
qweather version --output json
qweather capability list --lifecycle current --output json
```

Use `capability show` when a task needs the exact flags or Product Gate for one
Capability:

```sh
qweather capability show weather.city.current --output json
```

## Routine weather and air queries

```sh
qweather weather city current --place Beijing --country CN --output text
qweather weather city daily --place-id 101010100 --days 7 --output text
qweather weather grid hourly --coordinate geo:39.90,116.40 --hours 24 --output json
qweather alert current --coordinate geo:39.90,116.40 --output text
qweather air current --coordinate geo:39.90,116.40 --output text
```

Use the current conversation language only on commands that expose `--lang`.
Do not add a language or unit flag that is absent from the generated command
reference.

## Mountain and scenic-area forecasts

Prefer grid weather anchored to verified scenic-area coordinates for mountains
and scenic areas. Validate any POI candidate using `places-and-errors.md`
before using its coordinates. A grid point represents the selected coordinate
area; do not imply trail-, slope-, summit-, or microclimate-level precision.

For a target calendar date, calculate the inclusive coverage requirement:

```text
requiredDays = target date - current date + 1
```

Choose the smallest available forecast tier that covers `requiredDays`:

| `requiredDays` | Forecast selection |
| ---: | --- |
| 1–3 | grid daily, `--days 3` |
| 4–7 | grid daily, `--days 7` |
| 8–10 | city daily, `--days 10` |
| 11–15 | city daily, `--days 15` |
| 16–30 | city daily, `--days 30` |

The city tiers are an explicit degradation because grid daily weather currently
offers only 3- and 7-day tiers. If `requiredDays` is outside 1–30, report that
the current CLI cannot provide it; do not substitute a shorter forecast.

Treat a city forecast beyond grid coverage only as the regional trend for the
verified nearby county or city, never as an exact scenic-area forecast. If the
result has no location name, or its `fxLink` points to another county or city,
do not claim that it describes the scenic area precisely.

Every mountain or scenic-area answer must state:

- the forecast anchor, such as the verified coordinate or nearby county/city;
- the semantic difference between that anchor and the requested scenic area;
  and
- whether the forecast is grid weather or nearest-city regional weather.

## UNIX pipelines

Machine Result data is compact and deterministic. Keep complete Attribution in
every presented or stored transformation; a bare scalar cannot satisfy that
requirement by itself. Prefer a small structured record:

```sh
qweather weather city current --place-id 101010100 --output json |
  jq '{observedAt: .data.now.obsTime, condition: .data.now.text, attribution}'

qweather air current --coordinate geo:39.90,116.40 --output json |
  jq '{pollutants: [.data.pollutants[] | {code, concentration}], attribution}'
```

If an external interface requires a scalar, preserve Attribution in its
adjacent record, artifact, or presentation rather than silently discarding it.

Use Provider Body only when the caller explicitly needs QWeather's exact
successful bytes:

```sh
qweather weather city current --place-id 101010100 --output body >weather.json
```

Do not parse Text View with `grep` when a stable JSON field is required. Text is
complete and deterministic for one CLI version but is not a machine schema.

## Freshness and cache control

```sh
qweather weather city current --place-id 101010100 --refresh --output text
qweather weather city current --place-id 101010100 --no-cache --output json
qweather cache status --output json
qweather cache clear --capability weather.city.current --output text
```

`--refresh` skips the cache read and replaces an eligible entry after success.
`--no-cache` skips both read and write. Do not use either by default, and never
add an automatic retry after a failed request.
