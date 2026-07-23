# Issue tracker: GitHub

Issues and PRDs for this repo live as GitHub issues in the private
`Nativu5/qweather-cli` repository. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`
- **Read an issue**: `gh issue view <number> --comments`
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments`
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply/remove labels**: `gh issue edit <number> --add-label "..."` or `--remove-label "..."`
- **Close an issue**: `gh issue close <number> --comment "..."`

Infer the repository from `git remote -v`; `gh` does this automatically inside the repository.

## Implementation lifecycle

For non-trivial work, follow the complete
[development SOP](./development-sop.md). In particular:

1. Read and claim one actionable Issue before implementation.
2. Create one focused branch and keep the diff within that Issue.
3. Run the required local checks.
4. Open a Draft PR against its intended integration branch.
5. Obtain independent Standards and Spec reviews from at least two SubAgents and
   post each report on the PR. Keep it Draft until both axes pass.
6. Mark the PR Ready, wait for required CI, and close the Issue only after its
   acceptance criteria are satisfied.

Issues remain the source for requirements, scope, blockers, decisions, and
closure. Code-review findings and re-review evidence belong on the PR whose diff
they evaluate.

Explicit upstream QWeather changes use the separate
[upstream-change SOP](./upstream-change-sop.md) in addition to this lifecycle.

## Pull requests as a triage surface

**PRs as a request surface: no.**

**PRs as a code-review surface: yes.** Standards, Spec, and material-fix
re-review reports belong on the Draft PR whose exact diff they evaluate.

GitHub shares one number space across issues and pull requests. Resolve an
ambiguous `#42` with `gh pr view 42`, then fall back to `gh issue view 42`.

## When a skill says “publish to the issue tracker”

Create a GitHub issue.

## When a skill says “fetch the relevant ticket”

Run `gh issue view <number> --comments`.

## Wayfinding operations

The map is a single issue labelled `wayfinder:map`; child issues are tickets.

- **Map**: stores Notes, Decisions-so-far, and Fog.
- **Child ticket**: use a GitHub sub-issue when available; otherwise use a task list and add `Part of #<map>`.
- **Blocking**: prefer GitHub’s native issue dependencies; otherwise add a `Blocked by: #<n>` line.
- **Frontier query**: select the first unassigned open child without open blockers.
- **Claim**: `gh issue edit <n> --add-assignee @me`.
- **Resolve**: comment with the answer, close the issue, then add its context pointer to the map.
