# Testing QWeather CLI

The test harness has two layers with different authority and cost. Never turn a
live QWeather call into a convenient substitute for a deterministic test.

## Layer 1: deterministic gates

Run focused tests while implementing. Before handing off any Go change, run:

```sh
test -z "$(gofmt -l .)"
go mod verify
make skill-check
go test ./...
go test -race ./...
go vet ./...
go build -o "${TMPDIR:-/tmp}/qweather" ./cmd/qweather
go test -tags=e2e ./tests/e2e -run '^$'
git diff --check
```

The Makefile is the canonical local and CI command entry point. `make check`
runs the deterministic sequence above and writes its build artifact to the
ignored `bin/` directory. Use `make test` for the normal non-live Go suite,
`make build` for `bin/qweather`, and `make help` for the small target list. CI
invokes the corresponding granular Make targets so each gate remains an
individually visible workflow step. The explicit commands above document what
those targets execute.

`make skill-check` regenerates the expected command and result-schema content
in memory, checks binary/npm/Skill version synchronization, and validates the
curated one-level Skill layout and metadata. It performs no network request and
does not modify tracked files.

The deterministic CI job also runs `packages/npm` tests on Node.js 22.11.0 and
24.18.0 with npm 11.16.0, checks the allowlisted reproducible npm tarball, and
runs the local-fixture install/version smoke. The npm tests cover standard and
npm-lifecycle proxy configuration resolution, HTTPS redirect handling, and
bounded streaming through the `undici@7` dispatcher. The smoke injects only a
local archive into the tested installer seam; it never changes the production
fixed GitHub download URL and never calls QWeather.

The tagged command compiles the E2E package with a regular expression that
matches no tests. It validates build compatibility without reading live-test
configuration or making a provider request.

Pull-request CI and `main` push CI run the same gates. They receive no QWeather
credentials. `go test ./...` does not discover the build-tagged E2E package.
Adding a normal test that uses the public network is prohibited.

Text presentation remains deterministic without turning every Capability into a
large golden fixture. Unit tests execute each of the 28 embedded entry templates
against its reviewed official example and verify complete rendering, including
the generic `Additional fields` remainder, without `<no value>`, duplicate
Attribution, data loss, or unexpected fallback. Three full Text goldens
represent a current object, an array forecast, and a deeply nested
`metadata-v1` response. Focused tests separately cover No Data, generic fallback,
sorted object keys, provider-ordered untruncated arrays, Attribution, units, Text
Problems, compact Machine Result/Machine Problem output, and byte-exact Provider
Body output. These tests use local fixtures and never expand the live-call
budget.

## Layer 2: live release smoke

`tests/e2e/e2e_test.go` is process-level verification of the released CLI
contract. It is guarded by the `e2e` build tag and requires all of:

- `QWEATHER_E2E_BINARY`: absolute path to the already-built binary;
- `QWEATHER_API_HOST`: the dedicated account API Host; and
- `QWEATHER_API_KEY`: a release-smoke credential.

The suite executes sequentially and makes at most three Basic provider calls:

1. Geo city lookup;
2. current city weather using the `code-refer-v1` Response Family; and
3. current air quality using the `metadata-v1` Response Family.

Every provider command passes `--output json --no-cache`; the version/install
smoke also selects `--output json` when it inspects build metadata. Inputs are
exact Location IDs or coordinates where needed, so no implicit Geo resolution
adds a fourth request. Assertions cover only the stable Machine Result,
Capability identity, Billing Group, Response Family, cache bypass, and the
expected logical operation. The suite does not assert Text layouts, request
Provider Body mode, or log successful provider bodies or secrets.

The normal execution path is the manual `.github/workflows/release-gate.yml`
workflow on an exact `release/vX.Y.Z` branch and its protected
`qweather-release-smoke` Environment. Follow the
[release branch SOP](./release-sop.md) and verify the Environment's reviewer,
branch policy, and secret names before dispatch. Do not run the suite from a
PR, a `main` workflow, a schedule, or an unapproved local shell. When an
approved diagnostic run is necessary, inject the host and key through a
protected secret source, build the binary first, and run only:

```sh
export QWEATHER_E2E_BINARY="$PWD/qweather"
go test -tags=e2e ./tests/e2e -run '^TestReleaseSmoke$' -count=1
```

The paid-only Storm, Marine tide, and Solar forecast capabilities are excluded
from complete live E2E and release smoke by
[ADR 0006](../adr/0006-limit-live-e2e-coverage-for-paid-only-capabilities.md).
Account is also excluded from this suite because its output is sensitive. Do
not add Account, retries, parallelism, or broader sampling without a focused
Issue and an explicit quota, product-policy, and security review.
