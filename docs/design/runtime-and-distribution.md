# QWeather CLI runtime and distribution

Status: accepted design

Last verified against QWeather documentation: 2026-07-22

This document defines configuration, authentication, networking, caching, Skill packaging, npm installation, release, and verification behaviour.

## Configuration

### File and profile model

The default file is `os.UserConfigDir()/qweather/config.toml`. The CLI uses [`pelletier/go-toml/v2`](https://github.com/pelletier/go-toml) with strict unknown-field decoding and a project-owned typed loader. It does not use Viper, dynamic key lookup, remote configuration, file watching, dotenv loading, or runtime write-back.

```toml
[profiles.default]
api_host = "abc1234xyz.def.qweatherapi.com"
auth = "jwt"
project_id = "ABCDE23456"
credential_id = "ABCDE12345"
private_key_file = "~/.config/qweather/ed25519-private.pem"
jwt_ttl = "15m"
language = "auto"
unit = "metric"

[profiles.legacy]
api_host = "abc1234xyz.def.qweatherapi.com"
auth = "api_key"
api_key_env = "QWEATHER_API_KEY"
language = "auto"
unit = "metric"

[cache]
enabled = true
sensitive = false
stale = false
```

The file schema and effective runtime configuration are distinct types. The loader selects one profile and returns an immutable effective configuration with secret-free provenance.

### Precedence

Selector precedence:

```text
config path:  --config  > QWEATHER_CONFIG  > default path
profile:      --profile > QWEATHER_PROFILE > "default"
```

Effective-value precedence:

```text
explicitly changed non-secret flag
    > fixed QWEATHER_* environment override
    > selected profile and file cache settings
    > compiled default
```

An environment variable that exists with an empty value is an explicit invalid override; it does not silently fall back to the file. Only a fixed, documented list of environment variables is recognized. Environment variables override the selected profile, not arbitrary unselected profiles.

### Supported environment sources

Non-secret overrides include:

- `QWEATHER_API_HOST`
- `QWEATHER_PROJECT_ID`
- `QWEATHER_CREDENTIAL_ID`
- `QWEATHER_PRIVATE_KEY_FILE`
- `QWEATHER_JWT_TTL`
- `QWEATHER_LANGUAGE`
- `QWEATHER_UNIT`
- `QWEATHER_CACHE_ENABLED`
- `QWEATHER_CACHE_SENSITIVE`

Secret sources include:

- `QWEATHER_API_KEY` for API-key compatibility mode;
- `QWEATHER_JWT` for an externally issued short-lived token.

The loader records that a secret source is present and where it came from, but never includes its value in diagnostics or serializable effective configuration.

### Offline validation

`qweather config check` invokes the same loader and validator used by data commands without invoking the HTTP adapter. It checks:

- TOML syntax, types, and unknown fields;
- profile naming and existence;
- API Host form and HTTPS-only request eligibility;
- authentication-method required and mutually exclusive fields;
- JWT duration and the official 24-hour maximum;
- private-key readability, PEM/PKCS#8 Ed25519 parsing, and local file permissions;
- referenced environment-secret presence; and
- cache settings and directory permissions.

It does not test credentials by consuming a weather request.

## Authentication

QWeather supports API KEY and Ed25519 JWT and [recommends JWT](https://dev.qweather.com/docs/configuration/authentication/). The CLI supports exactly one of three effective authentication sources:

1. generate a JWT from project ID, credential ID, and an Ed25519 private-key file;
2. use a complete short-lived JWT supplied by `QWEATHER_JWT`; or
3. use an API KEY supplied through its referenced environment variable.

For locally generated JWTs:

- header `alg` is `EdDSA` and `kid` is the credential ID;
- payload `sub` is the project ID;
- `iat` is current time minus 30 seconds;
- default lifetime is 15 minutes and never exceeds 24 hours;
- only the official required or reserved fields are emitted; and
- the private key never leaves memory except for its source file.

The implementation uses Go's standard `crypto/ed25519`, `crypto/x509`, `encoding/pem`, and Base64URL support. It does not require a JWT framework.

There are no secret flags. The configuration module never writes an effective configuration back to TOML. A future `config init`, if added, may create only a secret-free template.

## Network behaviour

- All provider operations are HTTPS GET requests to the selected account-specific [API Host](https://dev.qweather.com/docs/configuration/api-host/).
- URL paths and query values are built with Go URL types and percent encoding, never string concatenation.
- The HTTP adapter accepts gzip and enforces a bounded decompressed response size; the initial limit is 16 MiB.
- The default deadline is 10 seconds and may be changed per invocation.
- The Go transport honours `HTTPS_PROXY`, `HTTP_PROXY`, and `NO_PROXY`.
- Redirects to a different host are rejected before credentials can be forwarded.
- There is no automatic retry. The problem envelope reports whether a failure is retryable.
- 400/401/403 and other permanent errors stop immediately. A caller may deliberately retry a 429 or transient failure after observing the structured problem.
- Legacy HTTP-200 bodies are also checked for provider `code`; modern problem responses are classified by HTTP status and body.

## Persistent cache

QWeather [recommends elastic caching](https://dev.qweather.com/docs/best-practices/cache/) to reduce request volume and latency. Cache policy is part of every capability record; there is no implicit global TTL fallback.

### Default policy

| Capability group | Default hard TTL | Evidence |
| --- | ---: | --- |
| `alert.current`; `weather.precipitation.minutely` | 5m | official recommendation lower bound |
| city/grid current weather | 10m | official current-weather lower bound; conservative grid derivation |
| active / inactive storm data | 20m / 60m | official recommendation |
| city/grid hourly weather; current/hourly/station air | 30m | official lower bound or conservative derivation |
| city/grid daily weather | 1h | official lower bound |
| weather indices; solar radiation | 6h | official recommendation |
| daily air; tide | 8h | official lower bound |
| sun, moon, solar position, and historical weather | 24h | documented update cycle or conservative project derivation |
| four Geo capabilities | disabled | license restriction |
| two Account capabilities | disabled by default; 10m after opt-in | project security policy |

The [GeoAPI restriction](https://dev.qweather.com/docs/terms/restriction/#cache-or-index-geoapi-data) is a hard policy. No flag or configuration value may enable persistent or cross-process caching of Geo responses, candidate lists, search text, negative results, place mappings, or POIs.

Automatic place resolution may use Geo Data during one invocation. The subsequent non-Geo data request may use persistent cache, but only the eligible provider body is stored. The result envelope and `resolvedPlace` object are rebuilt for the current invocation and are never written wholesale.

### Key and storage

A cache key is a SHA-256 digest of a canonical structure containing:

- cache schema and request-semantics revision;
- stable capability ID;
- normalized API origin and non-secret profile scope;
- typed, normalized downstream target; and
- every effective parameter that can affect the provider body, including defaults.

It never contains an Authorization header, token, API KEY, private key, reversible secret hash, raw request URL, output formatting option, or Geo query text.

The file adapter uses `os.UserCacheDir()`, directory mode `0700`, file mode `0600`, same-directory temporary files, and atomic rename. Each record stores the provider body, outcome, `storedAt`, `expiresAt`, TTL, and a small set of provider diagnostic timestamps. Expired entries are removed opportunistically under a bounded disk budget.

### Read and refresh

- `hit` returns an unexpired body without a provider data request.
- `miss` invokes the provider and writes an eligible success.
- `--refresh` skips the read and replaces the entry after a successful request.
- `--no-cache` skips both read and write.
- `disabled` applies to Geo and non-opted-in Account data.
- Errors are not cached.
- Valid No Data may be cached at the capability TTL or a stricter endpoint-specific TTL.
- Expired bodies are not returned automatically when the network fails.

For rolling hourly/daily arrays, expiration is the earlier of the hard TTL and the relevant target-timezone hour or day boundary. Grid data uses UTC boundaries. Explicit-date tide, astronomy, and history queries do not require boundary truncation.

QWeather currently documents no ETag, `Cache-Control`, `Last-Modified`, or 304 contract. The CLI does not invent conditional requests from `metadata.tag` or provider timestamps.

## Skill package

The repository contains one Skill:

```text
skills/qweather/
├── SKILL.md
├── agents/
│   └── openai.yaml
└── references/
    ├── common-tasks.md
    ├── places-and-errors.md
    ├── command-reference.md
    ├── result-schema.md
    └── products-and-attribution.md
```

`SKILL.md` is concise and contains only triggering metadata, command-selection workflow, installation check, place-error handling, Product Gate rules, cache-refresh guidance, and Attribution requirements. Detailed material is loaded from one-level references only when relevant.

`command-reference.md`, schema tables, and stable problem-code tables are generated from the Go registry and contract definitions. Human workflow guidance remains hand-written. CI regenerates these files and fails when tracked output differs.

The complete crawler output and research reports are not shipped. Curated references include official hyperlinks and `last_verified` dates. For volatile pricing, geography, alert, or lifecycle facts, the Skill checks an official QWeather link when the local catalog cannot answer safely.

If `qweather` is absent, the Skill prints a fixed-version npm installation command and waits for the user. It never installs software itself.

## npm adapter

The adapter lives outside the Skill under `packages/npm/`. Its package version always matches the Go CLI version.

During an explicit npm installation:

1. detect `process.platform` and `process.arch`;
2. select the exact same-version GitHub Release asset;
3. download through the configured HTTP(S) proxy when applicable;
4. stream into a temporary file;
5. compare SHA256 with the manifest embedded in the npm package;
6. extract and atomically place the binary; and
7. set executable permissions where required.

During normal execution, the JavaScript shim never downloads or updates anything. It launches the installed binary and preserves arguments, signals, stdout, stderr, and exit status.

When npm lifecycle scripts are disabled, the shim reports the missing binary and an explicit `install` or `repair` command. It does not download on first weather use and does not fall back to compiling Go source.

Private development releases may use `GH_TOKEN`. Public distribution uses unauthenticated public Release assets.

## Release

### Platform matrix

| GOOS | GOARCH | Archive |
| --- | --- | --- |
| `darwin` | `arm64` | `tar.gz` |
| `darwin` | `amd64` | `tar.gz` |
| `linux` | `arm64` | `tar.gz` |
| `linux` | `amd64` | `tar.gz` |
| `windows` | `arm64` | `zip` |
| `windows` | `amd64` | `zip` |

Release builds use `CGO_ENABLED=0`. Unsupported platforms fail clearly; the adapter does not compile from source. Every asset appears in the SHA256 manifest embedded in the matching npm package.

### Versioning

One semantic version identifies the Git tag, binary, npm package, and generated Skill catalog. `qweather version --json` also exposes Go version, source commit, build time, and registry hash.

The npm adapter never resolves `latest` at runtime. A package version downloads only its matching release version.

### Visibility and license

Development remains private. Before the first general distribution, source and Release assets become public under Apache License 2.0. The project is an unofficial QWeather client and is not affiliated with QWeather. The project license covers project code only; use of QWeather credentials, data, Attribution, and trademarks remains subject to QWeather's terms.

## Implicit network policy

The only runtime network operations are:

- QWeather capabilities explicitly requested by the caller;
- Geo resolution explicitly implied by a `--place` or required target conversion; and
- binary download during an explicit npm install or repair.

There is no telemetry, crash reporting, usage analytics, automatic update check, remote configuration, or background documentation request.

Maintainer documentation checks may run manually or in isolated CI. They do not alter the installed registry or final-user runtime.

## Verification scope

The project uses focused unit tests and a small approved smoke surface:

- table-driven registry validation and provider request mapping;
- place uniqueness and ambiguity;
- cache hit, expiry, Geo prohibition, and Account opt-in;
- TOML precedence and strict decoding;
- JWT signing with a fixed clock;
- representative result/problem and exit-code behaviour;
- npm platform selection, SHA256 success/failure, and missing-binary guidance;
- cross-compilation of the six release targets; and
- manual or approved smoke checks for Geo plus current weather, one modern response family, and npm install/version.

There is no coverage percentage target, scheduled live workflow, exhaustive platform runtime suite, or large golden-fixture corpus. Solar, Marine, and Account are smoke-tested only when a relevant change warrants an explicitly approved call.

## Primary references

- [Authentication](https://dev.qweather.com/docs/configuration/authentication/)
- [API request configuration](https://dev.qweather.com/docs/configuration/api-config/)
- [Security guidelines](https://dev.qweather.com/docs/best-practices/security-guidelines/)
- [Caching best practices](https://dev.qweather.com/docs/best-practices/cache/)
- [GeoAPI restrictions](https://dev.qweather.com/docs/terms/restriction/#cache-or-index-geoapi-data)
- [Language behaviour](https://dev.qweather.com/docs/resource/language/)
- [Units](https://dev.qweather.com/docs/resource/unit/)
- [Pricing](https://dev.qweather.com/docs/finance/pricing/)
- [Attribution](https://dev.qweather.com/docs/terms/attribution/)
