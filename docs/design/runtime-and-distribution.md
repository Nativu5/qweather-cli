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

[profiles.api_key]
api_host = "abc1234xyz.def.qweatherapi.com"
auth = "api_key"
api_key = "replace-with-your-api-key"
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

An environment variable that exists with an empty value is an explicit invalid override; it does not silently fall back to the file. Only the documented fixed overrides and the one secret variable explicitly referenced by `api_key_env` are recognized. Environment variables override the selected profile, not arbitrary unselected profiles.

For API-key profiles, `api_key_env` names the environment variable to consult and defaults to `QWEATHER_API_KEY` when omitted. A present, non-empty environment value takes precedence over `api_key`. When that environment variable is absent, the loader falls back to the inline `api_key`; when it is present but empty, validation fails without falling back. The two fields may coexist.

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

- `api_key` in the selected API-key profile;
- the environment variable referenced by `api_key_env`, defaulting to `QWEATHER_API_KEY`;
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
- inline API-key or referenced environment-secret presence and precedence;
- restrictive Unix permissions for any configuration file containing an inline API key; and
- cache settings and directory permissions.

It does not test credentials by consuming a weather request, and it never prints either an inline or environment-supplied secret.

## Authentication

QWeather supports API KEY and Ed25519 JWT and [recommends JWT](https://dev.qweather.com/docs/configuration/authentication/). The CLI supports exactly one of three effective authentication sources:

1. generate a JWT from project ID, credential ID, and an Ed25519 private-key file;
2. use a complete short-lived JWT supplied by `QWEATHER_JWT`; or
3. use an API KEY supplied inline in a private profile or through its referenced environment variable.

For locally generated JWTs:

- header `alg` is `EdDSA` and `kid` is the credential ID;
- payload `sub` is the project ID;
- `iat` is current time minus 30 seconds;
- default lifetime is 15 minutes and never exceeds 24 hours;
- only the official required or reserved fields are emitted; and
- the private key never leaves memory except for its source file.

The implementation uses Go's standard `crypto/ed25519`, `crypto/x509`, `encoding/pem`, and Base64URL support. It does not require a JWT framework.

There are no secret flags. On Unix, a configuration file containing any non-empty inline API key must be a regular file whose permissions do not allow group or other access (for example, mode `0600`). The configuration module never writes an effective configuration back to TOML. The CLI has no `config init` command; users can copy the secret-free template shipped with the Skill. A future `config init`, if added, may create only a secret-free template.

## Network behaviour

- All provider operations are HTTPS GET requests to the selected account-specific [API Host](https://dev.qweather.com/docs/configuration/api-host/).
- URL paths and query values are built with Go URL types and percent encoding, never string concatenation.
- The HTTP adapter accepts gzip and enforces a bounded decompressed response size; the initial limit is 16 MiB.
- The default deadline is 10 seconds and may be changed per invocation.
- The Go transport honours `HTTPS_PROXY`, `HTTP_PROXY`, and `NO_PROXY`.
- Redirects to a different host are rejected before credentials can be forwarded.
- There is no automatic retry. A Text Problem or Machine Problem reports whether a QWeather-owned failure is retryable.
- 400/401/403 and other permanent errors stop immediately. A caller may deliberately retry a 429 or transient failure after observing the selected problem presentation.
- `code-refer-v1` HTTP-200 bodies are also checked for provider `code`; `metadata-v1` failures are classified by HTTP status and body. `console-v1` remains a separate Account response shape.

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

Automatic place resolution may use Geo Data during one invocation. The subsequent non-Geo data request may use persistent cache, but only the eligible Provider Body and minimal response metadata are stored. Text Views, Machine Results, and the `resolvedPlace` object are rebuilt for the current invocation and are never written wholesale.

### Key and storage

A cache key is a SHA-256 digest of a canonical structure containing:

- cache schema and request-semantics revision;
- stable capability ID;
- normalized API origin and non-secret profile scope;
- typed, normalized downstream target; and
- every effective parameter that can affect the provider body, including defaults.

It never contains an Authorization header, token, API KEY, private key, reversible secret hash, raw request URL, output mode, or Geo query text.

The file adapter uses `os.UserCacheDir()`, directory mode `0700`, file mode `0600`, same-directory temporary files, and atomic rename. Each `qweather.cache-record/v3` record stores the Provider Body, outcome, structural Response Family, HTTP status, `storedAt`, `expiresAt`, the policy maximum TTL used for compatibility validation, and only the response metadata required to reconstruct an application Result. It never stores rendered output. Expired entries are removed opportunistically under a bounded disk budget.

The v3 record schema replaces the private-development `qweather.cache-record/v2` shape because the Response Family labels changed from lifecycle-suggesting names to `code-refer-v1`, `metadata-v1`, and `console-v1`. Older records are treated as incompatible and removed through normal bounded cleanup; there is no migration or compatibility mapping before public distribution. This project-only record-label change does not alter request semantics, so it does not increment a Capability's `RequestRevision` or change the `qweather.cache-key/v1` structure.

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
├── config.toml
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

`config.toml` is a secret-free reference template. It uses API KEY authentication
by default and includes a commented alternative for locally generated Ed25519
JWT authentication. The template is copied with the Skill and is never filled
or written automatically.

`command-reference.md`, schema tables, and stable problem-code tables are generated from the Go registry and contract definitions. Human workflow guidance remains hand-written. CI regenerates these files and fails when tracked output differs.

The Skill does not distribute QWeather OpenAPI files, copied response examples,
or another upstream documentation snapshot. Generated and curated references
link to the current official QWeather web documentation when an Agent needs a
provider field description, response schema, example, or volatile policy fact.
This links-only boundary avoids a second synchronization and consistency
contract inside the Skill.

Agents explicitly pass `--output text` for routine reading, select `--output json` when they need exact field paths or JSON value types, and reserve `--output body` for byte-exact successful provider data. They use the generated command reference for the supported CLI surface and may follow its official links for upstream parameters, response schemas, and field descriptions. Deprecated paths or other documentation details never imply an executable capability. For conflicts, prose-only constraints, and volatile pricing, geography, alert, or lifecycle facts, the curated project contract and current official documentation take precedence.

The rest of the official site checkout and detailed research reports are not shipped. Curated references include official hyperlinks and `last_verified` dates.

If `qweather` is absent, the Skill prints a version-neutral npm installation command and waits for the user. It never installs software itself.

## npm adapter

The adapter lives outside the Skill under `packages/npm/`. Its package version always matches the Go CLI version.

The implementation is a CommonJS package for Node.js `>=22.11.0`, the first
Node 22 LTS release. Its reviewed runtime dependencies are pinned in
`packages/npm/npm-shrinkwrap.json`: `tar@7.5.22`, `undici@7.29.0`, and
`yauzl@3.4.0`. The package contains only the shim, installer,
archive/checksum helpers, project README/license, and the release
`checksums.txt` manifest; it never contains Go source or a downloaded platform
binary.

During an explicit npm installation:

1. detect `process.platform` and `process.arch`;
2. select the exact same-version GitHub Release asset;
3. download through the configured HTTP(S) proxy when applicable;
4. stream into a temporary file;
5. compare SHA256 with the manifest embedded in the npm package;
6. extract and atomically place the binary; and
7. set executable permissions where required.

The installer uses the fixed public GitHub Release URL and downloads
anonymously through `undici@7`'s `EnvHttpProxyAgent`. Proxy precedence is:

- HTTP: non-empty `npm_config_proxy`, `http_proxy`, then `HTTP_PROXY`;
- HTTPS: non-empty `npm_config_https_proxy`, `https_proxy`, then `HTTPS_PROXY`,
  falling back to the resolved HTTP proxy; and
- bypass list: non-empty `npm_config_noproxy`, `no_proxy`, then `NO_PROXY`.

The installer does not inspect platform-specific system proxy settings or
invoke `curl`/`wget`. The same dispatcher is used for every manually checked
HTTPS redirect. It follows at most five HTTPS redirects, enforces a 64 MiB
archive and 128 MiB extracted-content limit, rejects HTTP downgrades, validates
the exact three-entry archive layout, verifies SHA256, and installs through a
same-directory temporary path and atomic rename. It does not read `GH_TOKEN`,
accept a mirror or URL override, retry, or compile source.

During normal execution, the JavaScript shim never downloads or updates anything. It launches the installed binary and preserves arguments, signals, stdout, stderr, and exit status.

When npm lifecycle scripts are disabled, the shim reports the missing binary and an explicit `install` or `repair` command. It does not download on first weather use and does not fall back to compiling Go source.

The installer only consumes public Release assets anonymously. Maintainer-only
publication workflows may use GitHub's workflow token for GitHub operations,
but the npm adapter never reads `GH_TOKEN` and cannot install from a private
Release.

## Release

Daily integration on `main` never publishes or runs live smoke. Stable release
validation is manual and valid only on an exact `release/vX.Y.Z` branch. The
[release branch SOP](../agents/release-sop.md) defines branch creation,
stabilization, approval, smoke, publication handoff, failure handling, and
retirement. The Release gate double-builds and retains one exact-SHA artifact
set, then smoke-tests the Linux amd64 binary from that set; packaging and
publication are handled by the independently protected publication workflow.
The protected reviewer rules and secret names must be verified before every
live release run; they are part of the release environment, not a one-time
first-release prerequisite.

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

One semantic version identifies the Git tag, binary, npm package, and generated Skill catalog. `qweather version --output json` also exposes Go version, source commit, build time, and registry hash.

The npm adapter never resolves `latest` at runtime. A package version downloads only its matching release version.

### Visibility and license

The source repository is public under Apache License 2.0. Release assets become
public only through the exact-SHA publication workflow. The project is an
unofficial QWeather client and is not affiliated with QWeather. The project
license covers project code only; use of QWeather credentials, data,
Attribution, and trademarks remains subject to QWeather's terms.

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
- all 28 Capability Text entry templates executing against reviewed official examples without `<no value>`, duplicate Attribution, data loss, or unexpected fallback;
- three full Text goldens representing a current object, an array forecast, and a deeply nested `metadata-v1` response;
- representative Text/Machine Result/Machine Problem, Provider Body, fallback, and exit-code behaviour;
- npm platform selection, SHA256 success/failure, and missing-binary guidance;
- generated Skill reference drift, curated layout, metadata, and version synchronization;
- cross-compilation of the six release targets; and
- manual or approved smoke checks for Geo plus current weather, one `metadata-v1` Capability, and npm install/version.

There is no coverage percentage target, scheduled live workflow, exhaustive platform runtime suite, or large golden-fixture corpus. The three Storm capabilities, Marine tide, and Solar forecast have no free allowance and are excluded from complete live E2E and release smoke under [ADR 0006](../adr/0006-limit-live-e2e-coverage-for-paid-only-capabilities.md). Account is also excluded from the release-smoke suite because its output is sensitive. A narrowly scoped one-off Account diagnostic requires explicit approval and protected credentials; it does not expand the release-smoke contract.

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
