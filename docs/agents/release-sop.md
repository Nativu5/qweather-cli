# Release branch SOP

This procedure separates daily integration from release validation. Merges to
`main` never publish and never consume live-smoke quota. A stable release is
prepared only on a branch named exactly `release/vX.Y.Z` and validated by the
independent manual Release gate workflow.

This SOP stops at a release-ready commit. Issue #8 owns packaging, tags, GitHub
Releases, npm publication, and public distribution. A passing gate is not a
published release; publication remains a separate protected handoff.

The read-only workflow may be merged before release credentials are provisioned.
Issue #20 must be completed before the first real release: it adds a supported
required-reviewer rule and the two Environment-scoped secrets. Until then, do
not dispatch the live gate and do not treat the repository as release ready.
Issue #8's package job now produces and retains the exact six-platform artifact
set for 14 days; its publish workflow consumes that set without rebuilding.

## Roles and durable records

- A release Issue names one release owner, stable SemVer, source `main` SHA,
  release-branch head SHA, scope, and go/no-go decision.
- Once #20 is complete, protected Environment reviewers authorize the
  quota-consuming smoke job.
- The Release gate workflow proves deterministic gates and three approved live
  Basic calls for one exact release-branch commit.
- Issue #8 consumes the passing workflow URL, version, and exact commit SHA for
  later packaging/publication work.

Do not reuse one release Issue or branch for multiple versions.

## 1. Prepare the reviewed version

Choose a stable version in `X.Y.Z` form. Numeric identifiers must not contain
leading zeros; prerelease/build suffixes need a separate policy and are not
accepted by the current gate.

Dispatch the maintainer-only preparation workflow from `main`:

```sh
gh workflow run prepare-release.yml -f version=X.Y.Z
```

Review and merge the generated `chore(release): prepare vX.Y.Z` PR. It updates
the sole version source and synchronized npm metadata; for the initial version,
those files may already match and the workflow records an explicit no-op
version commit for review. After merge, the workflow creates the exact
`release/vX.Y.Z` branch and dispatches the gate. Record the full source and
branch-head SHAs on the release Issue. Branch creation does not run smoke,
create a tag, or publish anything.

## 2. Stabilize without mixing daily development

Freeze the release branch to release blockers. Features and unrelated cleanup
continue through ordinary PRs to `main`.

- Every release-branch change uses a focused Issue, a branch created from the
  release branch, a Draft PR targeting `release/vX.Y.Z`, both independent review
  axes, and required CI.
- Prefer fixing on `main` first, then cherry-pick the reviewed fix into a focused
  release-branch PR.
- If an urgent fix must land on the release branch first, create a follow-up PR
  to forward-port it to `main`. Do not merge the release branch wholesale back
  into `main`.
- Never force-push, rewrite, or directly patch a validated release head.

Any change after a passing Release gate makes the old result stale and requires
another approved run for the new exact head.

## 3. Run the independent release gate

The workflow exists on the default branch but must be dispatched with the exact
release branch selected and a matching version input:

```sh
gh workflow run release-gate.yml \
  --ref release/vX.Y.Z \
  -f version=X.Y.Z
```

Before dispatch, verify #20 is closed and the Environment still has its required
reviewer rule, `release/v*` branch policy, and exactly the two expected secret
names. Missing credentials are a safe pre-release stop, not a reason to add
repository-level fallbacks or placeholder values.

The workflow hard-fails unless the version is stable SemVer and the selected ref
is exactly `refs/heads/release/v<version>`. It then:

1. reruns the same deterministic gates used by PR CI;
2. waits for approval on the protected `qweather-release-smoke` Environment;
3. double-builds all six release targets and proves byte-for-byte equality;
4. uploads the first artifact set with a 14-day retention;
5. extracts the Linux amd64 binary from that exact artifact and runs the three
   sequential Basic smoke calls with `--no-cache`; and
6. records a `release ready` status for that exact SHA.

The Environment supplies `QWEATHER_API_HOST` and `QWEATHER_API_KEY`. Do not put
either value in workflow inputs, repository variables, Issue/PR text, command
output, or artifacts. Each approved run can consume three calls; do not rerun it
without recording why the prior result is unusable.

## 4. Hand off without publishing

After every job passes, record the workflow URL, `X.Y.Z`, release branch, and
full `GITHUB_SHA` on the release Issue and #8. Confirm the branch still points to
that SHA, and record the retained artifact name. The gate has no permission to
create tags, Releases, packages, or npm publications.

Publication automation added by #8 must remain independently triggered, accept
only `release/vX.Y.Z`, and verify a passing Release gate for the same version and
exact source SHA. A `main` push or merge is never a publication trigger.

## 5. Failure and rollback

Before publication, a failed gate means “not release ready.” Diagnose it through
a focused Issue and PR; revert or repair on the release branch, forward-port the
change where necessary, and rerun all gates. Never skip smoke, weaken assertions,
or reuse a pass from an older SHA.

If credentials or secret-bearing output may have escaped, stop the release,
rotate the credential, and remove the exposed material from every durable
surface before retrying.

After publication, do not rewrite an existing tag or silently replace assets.
Record the incident under #8 and prepare a new patch version. Package withdrawal
or downstream advisory actions belong to the publication procedure implemented
by #8.

## 6. Retire the branch

For a successful release, wait until #8 records publication completion, then
delete the release branch and close the release Issue with links to the gate and
publication evidence. For a cancelled release, record the reason and delete the
branch after confirming it contains no fix that still needs forward-porting.

Never recreate or reuse a retired `release/vX.Y.Z` branch. A later release uses
a new version, Issue, branch, approved gate run, and publication handoff.
