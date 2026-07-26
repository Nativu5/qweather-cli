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

Use a full commit from an ignored upstream checkout when a focused investigation
needs fixed evidence. Never compare only against a moving upstream branch, and
never treat an upstream document as an executable registry.

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
record the conflict rather than choosing the more convenient source. The Skill
links to current official pages and does not distribute either source as a
snapshot.

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
prove compatibility with this CLI or authorize a change to the curated
registry, generated references, or official links.

## 3. Classify every observed delta

Classify changes separately for English OpenAPI, Chinese OpenAPI, examples, and
rendered prose. One upstream commit may contain several classes.

| Change class | Typical evidence | Primary project impact |
| --- | --- | --- |
| Operation or lifecycle | added/removed path, method, `operationId`, deprecation, replacement, or sunset | `internal/catalog`, Tombstones, `internal/app/compiler.go`, CLI/help, compatibility policy |
| Parameter | requiredness, name, type, enum, range, format, default, path/query placement, coordinate order | catalog flags/targets, compiler and validation, cache-key semantics, focused mapping tests |
| Schema or example | response status/content type, field type, required field, enum, No Data shape, Response Family, example body | response classification, Capability Text templates, generic remainder/fallback, output/error handling, fixtures, generated field/reference material |
| Prose or policy | authentication, API Host, geography, pricing, quota, cache, Attribution, Product Gate, security guidance | config/network/cache policy, Billing Group, Product Gate, design contracts and Skill guidance |
| License or terms | content license, API EULA, Geo storage restriction, trademark or redistribution term | links-only documentation scope, cache/privacy rules; pause distribution when authority is unclear |

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
- `internal/qweather/` and `internal/output/` for Response Family, No Data,
  Text entry-template assumptions, generic remainder/fallback, Text/Machine
  Problem, Provider Body, Attribution, and unknown-field preservation;
- `internal/cache/` for eligibility, TTL/boundary, key inputs, and privacy;
- `internal/cli/` plus `docs/design/cli-contract.md` for command paths, flags,
  output-mode selection, Machine Result/Machine Problem schemas, Text
  presentation, stdout/stderr discipline, and exit meanings;
- generated Skill command/schema references and their generator source;
- official documentation links, focused research evidence, and the links-only distribution boundary; and
- focused unit tests for registry validation, request compilation, response
  classification, cache behaviour, and public process behaviour.

Increment a Capability's `RequestRevision` whenever an accepted upstream change
can make an old persistent cache entry represent different request semantics or
an incompatible provider response contract. Examples include a changed provider
path, normalized target, effective parameter/default, structural Response
Family semantics, or cache-relevant policy. A project-only Response Family label
rename, presentation-only template change, prose edit, help-only clarification,
or equivalent example correction does not require a revision. A cache-record
schema change may invalidate old records independently without changing cache-key
identity. Add or update a cache-key or cache-record test that demonstrates the
decision.

## 5. Decide compatibility before repair

Classify the project-facing effect before editing:

- **No public change:** upstream evidence changed but the curated mapping and
  public process contract remain correct.
- **Compatible repair:** implementation catches up without changing a promised
  command, required input, Machine Result/Machine Problem meaning, or exit
  meaning. A deterministic and complete Text layout may improve without being a
  machine-contract break.
- **Additive change:** a new optional flag or Current Capability is introduced;
  update the supported surface and generated references deliberately.
- **Breaking change:** removal/rename, new required input, Machine Result/Machine
  Problem meaning change, or exit meaning change; follow the major-version rule
  in the CLI contract and provide migration guidance.
- **Upstream ambiguity or legal risk:** keep the current CLI behavior and links,
  mark the Issue blocked, and seek authoritative clarification. Do not guess or
  distribute upstream content.

A newer upstream commit is not by itself a reason to change the CLI or its
official documentation links.

## 6. Repair and verify

Implement the smallest coherent repair under one Issue. Prefer table-driven
tests that pin the changed contract over broad copied fixtures. If an official
example or schema changes, execute the affected Capability entry template,
verify that all fields appear in either its primary layout or `Additional
fields`, and exercise generic fallback when the structural assumption changed.
Add a full Text golden only when the Capability represents one of the three
maintained golden families or the common layout itself changed. When official
documentation changes without a CLI contract change, update only the affected
links or verification dates and record the fixed research evidence in the
Issue. Do not copy an upstream snapshot into the Skill. Run generation twice
and require the second run to produce no diff.

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

After merge, update any recorded research commit only when the investigation
actually used and accepted that fixed evidence. Never advance a research pin
merely to make a documentation link or generated check pass.
