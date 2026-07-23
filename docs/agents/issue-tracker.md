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

## Pull requests as a triage surface

**PRs as a request surface: no.**

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
