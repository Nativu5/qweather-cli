# Agent development SOP

This procedure is the default path for non-trivial changes to QWeather CLI. It
is written for an Agent that can inspect the repository, operate GitHub through
`gh`, run local checks, and delegate independent reviews to SubAgents.

Small typo-only documentation fixes may use a proportionate subset. Any change
to executable behaviour, a public contract, dependencies, generated output,
CI, packaging, or release policy uses the complete procedure.

## 1. Establish the requirement

1. Read `AGENTS.md`, `CONTEXT.md`, the relevant ADRs, and the applicable design
   contract.
2. Read the proposed Issue and all comments. The Issue, not a draft PR, is the
   implementation PRD.
3. For a new feature, roadmap, or unresolved product decision, grill the
   requirement before decomposing it. Challenge its user outcome, scope,
   failure behaviour, compatibility, security, quota cost, and explicit
   non-goals. Do not invent answers the product owner must choose.
4. Research uncertain facts from primary sources. For QWeather behaviour, start
   with the official documentation and the pinned `qwd/dev-site` source rather
   than blogs or copied examples.
5. Capture durable decisions in the Issue or an accepted design document. Use
   an ADR only for a hard-to-reverse decision with a real trade-off.

Stop and request direction when a missing decision would materially change the
public contract, security posture, cost, or distribution scope.

## 2. Build a roadmap and Issues

Decompose a multi-part outcome into a map Issue and focused child Issues. Each
child should state:

- one observable outcome;
- scope and non-goals;
- testable acceptance criteria;
- dependencies or blockers; and
- relevant design or upstream evidence.

Order children by dependency. Do not combine unrelated refactoring, policy,
feature, and release work merely to reduce PR count. A non-trivial child Issue
normally maps to exactly one branch and one PR.

Use the labels and map conventions in [issue-tracker.md](./issue-tracker.md).
Update the map when scope, ordering, fog, or blockers change.

## 3. Claim and prepare one Issue

1. Select the first actionable, unblocked child.
2. Read its body and comments immediately before implementation.
3. Claim it with `gh issue edit <number> --add-assignee @me`.
4. Start from an up-to-date intended base branch and create a focused branch.
   Use `codex/issue-<number>-<slug>` for Agent-created branches unless the
   repository or release policy requires another name.
5. Confirm the worktree is clean or identify and preserve unrelated user work.

Do not implement directly on `main`. Do not start a second child merely because
the first becomes difficult; record the blocker and exhaust safe in-scope
alternatives first.

## 4. Implement the smallest complete change

- Keep the diff aligned with the Issue and accepted contracts.
- Prefer established packages and existing repository seams over new
  infrastructure.
- Add focused unit or deterministic integration tests with the behaviour.
- Update help, contracts, generated references, and operator documentation when
  their source behaviour changes.
- Record discovered out-of-scope work as a follow-up Issue or Issue comment, not
  as a hidden `TODO`.
- Never use live QWeather credentials during normal implementation or tests.

Commit boundaries should make the change inspectable, but a reviewer evaluates
the complete branch diff, not commit style.

## 5. Run local gates

Run focused tests during implementation. Before review, run every deterministic
gate required by the repository and Issue. For Go changes, the minimum is:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
git diff --check
git diff --check <base>...HEAD
```

Also run generation drift checks, race tests, adapter checks, workflow linting,
or build commands required by the affected surface. A live smoke test is never
an implied local gate; run one only when the release process explicitly
authorizes its credentials and quota.

## 6. Obtain two independent pre-PR reviews

Before creating the PR, delegate the complete branch diff to at least two
independent SubAgents. Use separate review contexts and do not let either review
stand in for the other:

- **Standards review:** compare the diff with `AGENTS.md`, repository design
  contracts, UNIX process discipline, security rules, test policy, and
  documentation/generation rules.
- **Spec review:** compare the diff with the Issue body and comments, checking
  every acceptance criterion, non-goal, edge case, and expected user outcome.

Each reviewer reports actionable findings with severity, file and line when
possible, evidence, and a pass/fail conclusion. The implementer records both
review results on the Issue before PR creation; a concise Issue comment may
link to durable review artifacts when available.

Fix all accepted blocking findings and explain any rejected finding with
evidence. Material fixes must be re-reviewed on the affected axis. A material
fix changes behaviour, contract, security, workflow control flow, or substantial
test logic; typo and formatting corrections do not require a full rerun.

The same Agent may coordinate the work but must not impersonate either
independent reviewer. If two independent SubAgents are unavailable, stop before
PR creation and record the review blocker on the Issue.

## 7. Open and land the PR

1. Re-run checks affected by review fixes.
2. Push the branch and open one PR using the repository template. Link the Issue
   with `Closes #<number>` and link both pre-PR review records.
3. Target `main` for ordinary integration. Release work follows the dedicated
   release policy and never converts a normal `main` merge into a publication.
4. Wait for required CI. Diagnose failures from evidence; do not bypass or
   weaken a gate to make the PR green.
5. Review the final PR diff for accidental files, secrets, generated drift, and
   unrelated changes.
6. Merge only when the Issue acceptance criteria and required checks pass.

After merge, comment on and close the Issue if GitHub did not close it, update
the parent map, remove obsolete branches as appropriate, and start the next
unblocked Issue from the newly integrated base.

## Handoff evidence

A completed Issue should make these facts discoverable:

- requirement and decision record;
- branch and PR;
- Standards review result;
- Spec review result;
- commands and results for deterministic gates;
- explicit live-smoke status;
- contract, documentation, and generation impact; and
- follow-up work or remaining risk.
