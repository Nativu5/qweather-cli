# Upstream QWeather change SOP

Use this procedure when QWeather documentation, an API contract, an example,
product policy, terms, or licensing changes explicitly. It supplements the
[development SOP](./development-sop.md); it does not permit an automatic sync
from upstream into executable behaviour.

The curated QWeather CLI Capability registry and accepted design contracts are
the project source of truth. The official `qwd/dev-site` repository is primary
upstream evidence. Its English and Chinese OpenAPI files are independent
evidence and must not be silently merged, averaged, or assumed equivalent.

## 1. Open a focused investigation

Create or claim an Issue before changing a contract. Record:

- the currently pinned full upstream commit;
- the candidate full upstream commit;
- the signal that prompted investigation;
- the affected Capability IDs or policy areas; and
- whether the change might be breaking, quota-sensitive, security-sensitive,
  or distribution-sensitive.

The distributed OpenAPI manifest is the preferred source for the current pin
once it exists. Until then, use the pin recorded in the accepted design
documents. Never compare only against a moving upstream branch.

## 2. Prepare fixed source revisions

Keep upstream material under ignored `.cache/` paths. For example:

```sh
git clone https://github.com/qwd/dev-site.git .cache/qweather-dev-site-source
git -C .cache/qweather-dev-site-source fetch --prune origin
git -C .cache/qweather-dev-site-source rev-parse <old-revision>
git -C .cache/qweather-dev-site-source rev-parse origin/master
git -C .cache/qweather-dev-site-source log --oneline <old-sha>..<new-sha>
git -C .cache/qweather-dev-site-source diff --stat <old-sha> <new-sha>
```

Record the resolved 40-character SHAs in the Issue before analysis. Inspect
both OpenAPI locales and their local examples with a fixed-SHA diff:

```sh
git -C .cache/qweather-dev-site-source diff <old-sha> <new-sha> -- \
  assets/openapi/qweather-apis-en.yml \
  assets/openapi/qweather-apis-zh.yml \
  assets/openapi/examples
git -C .cache/qweather-dev-site-source grep -n 'operationId\|deprecated' \
  <new-sha> -- assets/openapi
```

Inspect related English and Chinese prose under `content/en/docs/` and
`content/zh/docs/`. When the rendered official site disagrees with OpenAPI,
record the conflict rather than choosing the more convenient source.

Validate the candidate with the upstream repository's own pinned dependencies
in a detached review worktree:

```sh
git -C .cache/qweather-dev-site-source worktree add --detach \
  ../qweather-dev-site-review <new-sha>
npm --prefix .cache/qweather-dev-site-review ci
npm --prefix .cache/qweather-dev-site-review run openapi:test
git -C .cache/qweather-dev-site-source worktree remove \
  ../qweather-dev-site-review
```

An upstream test pass establishes upstream self-consistency only. It does not
prove compatibility with this CLI or approve a new distributed snapshot.

## 3. Classify every observed delta

Classify changes separately for English OpenAPI, Chinese OpenAPI, examples, and
rendered prose. One upstream commit may contain several classes.

| Change class | Typical evidence | Primary project impact |
| --- | --- | --- |
| Operation or lifecycle | added/removed path, method, `operationId`, deprecation, replacement, or sunset | `internal/catalog`, Tombstones, `internal/app/compiler.go`, CLI/help, compatibility policy |
| Parameter | requiredness, name, type, enum, range, format, default, path/query placement, coordinate order | catalog flags/targets, compiler and validation, cache-key semantics, focused mapping tests |
| Schema or example | response status/content type, field type, required field, enum, No Data shape, response family, example body | response classification, output/error handling, fixtures, generated field/reference material |
| Prose or policy | authentication, API Host, geography, pricing, quota, cache, Attribution, Product Gate, security guidance | config/network/cache policy, Billing Group, Product Gate, design contracts and Skill guidance |
| License or terms | content license, API EULA, Geo storage restriction, trademark or redistribution term | `NOTICE`, manifest/distribution scope, cache/privacy rules; pause distribution when authority is unclear |

Do not infer a stable project Capability ID from upstream `operationId`. Do not
make a deprecated operation executable merely because it remains in OpenAPI.
Do not assume an example is normative when schema and prose disagree.

## 4. Build the impact map

For every accepted delta, inspect these project surfaces and record either the
required repair or “no impact” with a reason:

- `internal/catalog/records.go` and catalog validation for Capability metadata,
  lifecycle, target, flags, response family, Billing Group, Product Gate, cache
  policy, and documentation URL;
- `internal/app/compiler.go` for the exact provider path, query, normalization,
  validation, and coordinate order;
- `internal/qweather/` and `internal/output/` for response-family, No Data,
  problem, Attribution, and unknown-field preservation;
- `internal/cache/` for eligibility, TTL/boundary, key inputs, and privacy;
- `internal/cli/` plus `docs/design/cli-contract.md` for command paths, flags,
  stdout/stderr schemas, and exit meanings;
- generated Skill command/schema references and their generator source;
- the reviewed OpenAPI snapshot, manifest, source links, and `NOTICE`; and
- focused unit tests for registry validation, request compilation, response
  classification, cache behaviour, and public process behaviour.

Increment a Capability's `RequestRevision` whenever an accepted change can make
an old persistent cache entry represent different request semantics or an
incompatible provider response contract. Examples include a changed provider
path, normalized target, effective parameter/default, response family, or
cache-relevant policy. A prose edit, help-only clarification, or equivalent
example correction does not require a revision. Add or update a cache-key test
that demonstrates the decision.

## 5. Decide compatibility before repair

Classify the project-facing effect before editing:

- **No public change:** upstream evidence changed but the curated mapping and
  public process contract remain correct.
- **Compatible repair:** implementation catches up without changing a promised
  command, required input, schema meaning, or exit meaning.
- **Additive change:** a new optional flag or Current Capability is introduced;
  update the supported surface and generated references deliberately.
- **Breaking change:** removal/rename, new required input, result/problem meaning
  change, or exit meaning change; follow the major-version rule in the CLI
  contract and provide migration guidance.
- **Upstream ambiguity or legal risk:** keep the existing pin, mark the Issue
  blocked, and seek authoritative clarification. Do not guess or distribute.

A newer upstream commit is not by itself a reason to change the CLI or snapshot.

## 6. Repair and verify

Implement the smallest coherent repair under one Issue. Prefer table-driven
tests that pin the changed contract over broad copied fixtures. When the
distributed OpenAPI snapshot changes, verify every manifest hash, referenced
local example, locale comparison, attribution file, and pinned source URL; run
generation twice and require the second run to produce no diff.

Run the deterministic local/CI gates from the development SOP. Live QWeather
smoke is reserved for the explicit pre-release workflow and remains limited to
its approved Basic sample; an upstream investigation never authorizes a live
call by itself.

Then open a Draft PR and obtain independent Standards and Spec reviews on that
PR. Give the Spec reviewer the old/new upstream SHAs, classification table,
impact map, compatibility decision, exact PR base/head SHAs, and Issue
acceptance criteria. Post both reports and any material-fix re-review on the PR.
Record unresolved locale or prose/spec conflicts in the Issue and in user-facing
documentation where they affect reliable use.

After merge, update the current upstream pin only if the reviewed snapshot was
actually accepted. Never advance a pin merely to make drift checks pass.
