# Common qweather tasks

Last verified: 2026-07-26

Pass `--output text` for reading and `--output json` for automation. These
examples never embed credentials; configuration and authentication remain owned
by the CLI.

## Validate local setup

```sh
qweather config check --output text
qweather version --output json
qweather capability list --lifecycle current --output json
```

All three commands are offline. Use `capability show` when a task needs the
exact flags or Product Gate for one Capability:

```sh
qweather capability show weather.city.current --output json
```

## Routine weather and air queries

```sh
qweather weather city current --place Beijing --country CN --output text
qweather weather city daily --place-id 101010100 --days 7 --output text
qweather weather grid hourly --coordinate geo:39.9042,116.4074 --hours 24 --output json
qweather alert current --coordinate geo:39.9042,116.4074 --output text
qweather air current --coordinate geo:39.9042,116.4074 --output text
```

Use the current conversation language only on commands that expose `--lang`.
Do not add a language or unit flag that is absent from the generated command
reference.

## UNIX pipelines

Machine Result data is compact and deterministic. Keep complete Attribution in
every presented or stored transformation; a bare scalar cannot satisfy that
requirement by itself. Prefer a small structured record:

```sh
qweather weather city current --place-id 101010100 --output json |
  jq '{observedAt: .data.now.obsTime, condition: .data.now.text, attribution}'

qweather air current --coordinate geo:39.9042,116.4074 --output json |
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
