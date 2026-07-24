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

Before creating every commit:

1. Inspect `git status --short`, the staged diff, and run
   `git diff --cached --check`; unstage or remove anything outside the Issue.
2. Write down the proposed subject and verify that it uses Conventional
   Commits syntax (`type(scope): description` or `type: description`) and
   accurately describes the staged change.
3. Create the commit only after those checks pass, then verify the recorded
   subject with `git log -1 --format='%s'`.

Commit boundaries should make the change inspectable, but a reviewer evaluates
the complete branch diff as well as the validity of each commit subject.

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

## 6. Open a Draft PR

1. Commit the complete focused change and re-run the checks affected by the
   final commit.
2. Push the branch and open one Draft PR using the repository template. Link the
   Issue with `Closes #<number>`.
3. Record the PR URL, full 40-character base commit SHA, full 40-character head
   commit SHA, and exact diff command. Reviewers must evaluate that fixed range,
   not a moving branch name alone.
4. Before marking the PR Ready, inspect `git log <base>..HEAD --format='%h %s'`
   and verify every subject. Verify that the PR title is also a valid
   Conventional Commit subject and accurately summarizes the complete squash
   diff; GitHub uses that title for the commit on `main`.
5. Target `main` for ordinary integration. Release work follows the dedicated
   release policy and never converts a normal `main` merge into a publication.

Keep the PR Draft while either review axis is missing, failing, or stale after a
material change.

## 7. Obtain two independent PR reviews

Delegate the Draft PR's complete fixed-SHA diff to at least two independent
SubAgents. Every review prompt identifies the PR URL/number, the exact base SHA,
the exact head SHA, the diff command, and the applicable Issue. Use separate
review contexts and do not let either review stand in for the other:

- **Standards review:** compare the diff with `AGENTS.md`, repository design
  contracts, UNIX process discipline, security rules, test policy, and
  documentation/generation rules.
- **Spec review:** compare the diff with the Issue body and comments, checking
  every acceptance criterion, non-goal, edge case, and expected user outcome.

Each reviewer reports actionable findings with severity, file and line when
possible, evidence, and a PASS/FAIL conclusion. Post the Standards report and
the Spec report as distinct comments or formal reviews on the Draft PR. When
SubAgents do not have separate GitHub identities, the coordinator posts each
report with its reviewer context, full reviewed base/head SHAs, and exact diff
command intact.

Fix all accepted blocking findings and explain any rejected finding with
evidence. Material fixes must be re-reviewed on the affected axis. A material
fix changes behaviour, contract, security, workflow control flow, or substantial
test logic; typo and formatting corrections do not require a full rerun. Record
the re-review on the same PR against its new exact head SHA.

The same Agent may coordinate the work but must not impersonate either
independent reviewer. If two independent SubAgents are unavailable, stop before
marking the PR Ready and record the blocker on the PR and Issue.

## 8. Mark Ready and land the PR

1. Re-run checks affected by review fixes and push the final head.
2. Update the PR body with links to both passing review reports and any required
   material-fix re-review.
3. Mark the PR Ready only when both axes pass for the current material diff and
   required local checks are current.
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
- Standards review result on the PR;
- Spec review result on the PR;
- commands and results for deterministic gates;
- explicit live-smoke status;
- contract, documentation, and generation impact; and
- follow-up work or remaining risk.
