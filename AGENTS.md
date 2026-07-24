# Repository instructions

These instructions apply to the entire repository. Keep this file focused on
development discipline; product and architecture decisions live in the linked
design documents.

## General principles

- Prefer simple, direct, unsurprising solutions.
- Do not build custom versions of solved infrastructure. Use the Go standard
  library when it is sufficient; otherwise prefer a mature, actively maintained
  community package over a home-grown framework.
- Do not add a dependency for a trivial helper. A dependency should remove
  meaningful implementation or maintenance risk.
- Before adding a package, check its maintenance status, license, security
  history, Go-version compatibility, API stability, and transitive dependency
  cost.
- Keep behaviour explicit. Avoid hidden global state, reflection-heavy magic,
  implicit fallbacks, and configuration that is difficult to trace.
- Apply YAGNI. Implement the issue in scope without speculative features or
  unrelated refactoring.
- Preserve existing user changes and keep unrelated work out of the diff.

## UNIX-friendly CLI design

- Follow the UNIX philosophy: each command should do one clear job and compose
  cleanly with other tools.
- Design output so users can naturally append tools such as `grep`, `jq`,
  `sort`, `xargs`, or another pipeline stage.
- Write successful data to stdout. Write diagnostics and errors to stderr.
- Keep default machine output deterministic, compact, and free of decoration.
- Never mix progress messages, logs, banners, or update notices into stdout.
- Use stable, meaningful exit codes. A failing command must exit non-zero.
- Avoid interactive prompts in normal command execution. Require explicit flags
  for acknowledgements that must work in automation.
- Do not assume a TTY. Colour and human-oriented formatting must be opt-in or
  safely TTY-aware and must never corrupt redirected output.
- Preserve a raw/provider-body mode when a structured wrapper would make common
  shell pipelines harder.
- Do not perform implicit retries, telemetry, update checks, background network
  requests, or runtime software installation.
- Honour standard proxy environment variables for network operations.

Before changing a command path, flag, stdout schema, stderr schema, or exit-code
meaning, review the public contract in `docs/design/cli-contract.md`.

## Go development

- Use the Go version declared by the module and keep the project compatible with
  its supported release matrix.
- Format all Go code with `gofmt`.
- Keep packages cohesive and interfaces small. Define an interface where
  behaviour genuinely varies or where an external dependency needs a test
  substitute, not merely to wrap a concrete type.
- Pass `context.Context` through network and other cancellable I/O paths.
- Return errors for expected failures; do not panic for invalid input, provider
  errors, configuration problems, or filesystem failures.
- Wrap errors with useful context using `%w` when callers need the cause.
- Avoid mutable package globals. Construct dependencies explicitly so tests can
  replace external I/O.
- Keep exported identifiers documented when their contract is not obvious.
- All source-code comments, doc comments, and `TODO`/`FIXME` annotations must be
  written in English.
- Comments should explain why a constraint or trade-off exists, not restate
  obvious code.
- Never log or format credentials, tokens, private keys, authorization headers,
  or reversible secret material.

## Dependencies and generated code

- Pin dependencies through `go.mod` and `go.sum`; do not vendor by default.
- Prefer typed library interfaces over stringly typed configuration and generic
  maps at module seams.
- Keep generated files deterministic and clearly marked.
- Edit the source of truth rather than hand-editing generated output.
- When generated contracts or Skill references change, regenerate them and
  verify that a second generation produces no diff.

## Testing

- Keep testing proportional to this small CLI.
- Add focused unit tests for new behaviour and bug regressions.
- Prefer table-driven tests for capability mappings, validation rules, and error
  classification.
- Replace true external I/O with a small scripted or local test adapter.
- Do not call the live QWeather API from the normal unit-test suite.
- Use only a few manual or explicitly approved smoke tests for real credentials,
  release installation, and representative provider response families.
- Do not introduce arbitrary coverage thresholds or large fixture suites without
  a concrete need.
- At minimum, run the relevant tests plus `go test ./...` and `go vet ./...`
  before handing off Go changes.
- For npm Adapter changes, run its focused tests and an install/version smoke
  check when practical.
- Follow `docs/agents/testing.md` for deterministic CI gates and the strictly
  opt-in, quota-consuming release smoke boundary.

## Issue-driven workflow

- GitHub Issues are the source of work and PRDs for `Nativu5/qweather-cli`.
- Before implementation, identify the relevant issue and read its body and
  comments. Do not infer requirements from a PR alone.
- Before every commit, inspect the staged diff and proposed subject. Commit
  subjects must use Conventional Commits syntax (for example,
  `type(scope): description` or `type: description`) and accurately describe
  the staged change. A squash merge uses the PR title as the resulting commit
  subject, so the PR title must pass the same check before the PR becomes Ready.
- Keep non-trivial changes scoped to an issue. If no suitable issue exists,
  create or request one before expanding the work.
- Use `gh` for issue operations and follow
  `docs/agents/issue-tracker.md`.
- Follow `docs/agents/development-sop.md` for non-trivial implementation work,
  including the required independent Standards and Spec reviews on a Draft pull
  request before it becomes Ready.
- Follow `docs/agents/release-sop.md` for release branches, approved live smoke,
  and the handoff to the independently scoped publication work.
- Use the canonical triage labels documented in
  `docs/agents/triage-labels.md`.
- Claim an actionable issue before implementation when the workflow requires
  ownership.
- Record newly discovered scope, blockers, and follow-up work on the issue rather
  than hiding them in code comments.
- Do not close an issue until its acceptance criteria are met and the relevant
  checks pass.

## Documentation

- Read `CONTEXT.md`, relevant ADRs under `docs/adr/`, and the applicable design
  contract before changing public behaviour.
- Use the canonical domain terms from `CONTEXT.md`.
- Update CLI help, generated Skill references, and design contracts when their
  public behaviour changes.
- Prefer primary official sources. Link to the relevant QWeather page rather
  than copying large sections of external documentation.
- Follow `docs/agents/upstream-change-sop.md` when an upstream document, API
  operation, schema, example, policy, or license changes explicitly.
- Keep local upstream checkouts and detailed research out of distributed
  artifacts. Redistribute only upstream files approved by the distribution
  contract, with their required source and license attribution.
- Create an ADR only for a hard-to-reverse decision with a real trade-off.

## Definition of done

- The change matches the issue and does not include unrelated work.
- Code is formatted and focused tests pass.
- Go changes pass `go test ./...` and `go vet ./...`.
- Public CLI behaviour and generated references are consistent.
- Error paths preserve stdout/stderr discipline and do not leak secrets.
- Relevant documentation and issue status are updated.

## Repository references

- Issue workflow: `docs/agents/issue-tracker.md`
- Triage labels: `docs/agents/triage-labels.md`
- Domain-doc workflow: `docs/agents/domain.md`
- Development SOP: `docs/agents/development-sop.md`
- Upstream-change SOP: `docs/agents/upstream-change-sop.md`
- Testing: `docs/agents/testing.md`
- Release branch SOP: `docs/agents/release-sop.md`
- Architecture: `docs/design/architecture.md`
- CLI contract: `docs/design/cli-contract.md`
- Runtime and distribution: `docs/design/runtime-and-distribution.md`
