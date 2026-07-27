# npm publication checklist

The first real package, `qweather-cli@0.1.0`, was published through the
protected workflow. Its one-time bootstrap token was revoked and Trusted
Publishing is now the required path. Complete this checklist in order for each
later version.

## Before a version PR

1. Confirm the repository is public and `main` protection and Actions policy
   are deliberately configured.
2. Confirm the protected `qweather-release-smoke` Environment still has exactly
   `QWEATHER_API_HOST` and `QWEATHER_API_KEY`, plus a required reviewer and the
   `release/v*` branch policy. Never put either value in a workflow input,
   repository variable, Issue, PR, summary, or artifact.
3. Confirm the generated Skill and version-synchronization checks are green.
4. Confirm the protected `qweather-release-publish` Environment still has a
   required reviewer, the `release/v*` branch policy, the configured npm
   Trusted Publisher, and no stored `NPM_TOKEN`.
5. Recheck `npm view qweather-cli name repository.url`. The existing package
   must belong to this repository, and the requested version must be absent.
6. Dispatch `prepare-release.yml` with stable `X.Y.Z`. Review and merge the
   generated `chore(release): prepare vX.Y.Z` PR; do not edit VERSION manually.

## Exact-SHA release gate

1. The merged version PR creates `release/vX.Y.Z` and dispatches
   `release-gate.yml` for that exact branch.
2. The gate runs deterministic Go/npm checks, double-builds all six targets,
   verifies archive layout and checksums, and retains the first artifact set
   for 14 days.
3. A protected reviewer approves the live smoke Environment. Smoke extracts
   and executes the Linux amd64 binary from the retained artifact; it never
   builds a second binary.
4. Record the successful gate run ID, version, branch, full source SHA, and
   artifact name. Any source change invalidates the handoff and requires a new
   gate run.

## Publication handoff

1. Immediately repeat the npm name and ownership check. The existing package
   must point to this repository and the requested version must be absent. Stop
   on any conflict or ambiguous registry response.
2. Dispatch `publish.yml` with the version, gate run ID, and exact source SHA.
3. The workflow revalidates the gate, verifies all six artifacts without
   rebuilding, refuses an existing tag, creates the immutable tag and Draft
   GitHub Release, uploads/read-backs the assets, publishes the Release, and
   checks anonymous downloads.
4. Only after public asset verification does it publish the staged npm package
   with provenance through npm Trusted Publishing/OIDC. Do not add or restore a
   stored npm token.
5. Verify global install, project-local install, `npx`, `qweather version
   --output json`, checksum selection on all six platforms, and shim stdout,
   stderr, exit-code, and signal forwarding.

## Stop conditions

- the requested npm version is already published, package ownership is
  unexpected, or registry availability is ambiguous;
- the generated Skill checks are not green, either protected Environment is
  missing its required reviewer/branch policy, or Trusted Publishing is not
  configured without a stored npm token;
- source SHA, release branch, gate run, artifact checksums, or VERSION disagree;
- any archive contains an unexpected entry or fails anonymous read-back;
- npm publish fails after a tag or public Release exists.

Never rewrite a tag or replace a published asset. Record the incident and
prepare a new patch version.
