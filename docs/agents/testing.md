# Testing QWeather CLI

The test harness has two layers with different authority and cost. Never turn a
live QWeather call into a convenient substitute for a deterministic test.

## Layer 1: deterministic gates

Run focused tests while implementing. Before handing off any Go change, run:

```sh
test -z "$(gofmt -l .)"
go mod verify
go test ./...
go test -race ./...
go vet ./...
go build -o "${TMPDIR:-/tmp}/qweather" ./cmd/qweather
go test -tags=e2e ./tests/e2e -run '^$'
git diff --check
```

The tagged command compiles the E2E package with a regular expression that
matches no tests. It validates build compatibility without reading live-test
configuration or making a provider request.

Pull-request CI and `main` push CI run the same gates. They receive no QWeather
credentials. `go test ./...` does not discover the build-tagged E2E package.
Adding a normal test that uses the public network is prohibited.

## Layer 2: live release smoke

`tests/e2e/e2e_test.go` is process-level verification of the released CLI
contract. It is guarded by the `e2e` build tag and requires all of:

- `QWEATHER_E2E_BINARY`: absolute path to the already-built binary;
- `QWEATHER_API_HOST`: the dedicated account API Host; and
- `QWEATHER_API_KEY`: a release-smoke credential.

The suite executes sequentially and makes at most three Basic provider calls:

1. Geo city lookup;
2. current city weather using the `legacy-v1` response family; and
3. current air quality using the `modern-v1` response family.

Every command passes `--no-cache`. Inputs are exact Location IDs or coordinates
where needed, so no implicit Geo resolution adds a fourth request. Assertions
cover only the stable result envelope, Capability identity, Billing Group,
response family, cache bypass, and the expected logical operation. The suite
does not log successful provider bodies or secrets.

The normal execution path is the manual `.github/workflows/release-gate.yml`
workflow on an exact `release/vX.Y.Z` branch and its protected
`qweather-release-smoke` Environment. Follow the
[release branch SOP](./release-sop.md). The workflow may exist before its
required-reviewer rule and secrets, but Issue #20 must close before any live
run. Do not run the suite from a PR, a `main` workflow, a schedule, or an
unapproved local shell. When an approved diagnostic run is necessary, inject
the host and key through a protected secret source, build the binary first, and
run only:

```sh
export QWEATHER_E2E_BINARY="$PWD/qweather"
go test -tags=e2e ./tests/e2e -run '^TestReleaseSmoke$' -count=1
```

Do not add Marine, Solar, Account, retries, parallelism, or broader sampling to
this suite without a focused Issue and an explicit quota/product-policy review.
