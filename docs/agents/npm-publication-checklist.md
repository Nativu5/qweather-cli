# npm publication checklist

Issue #8 implements the package and publication machinery but does not permit
an unattended first release. Complete this checklist in order for each version.

## Before a version PR

1. Confirm the repository is public and `main` protection and Actions policy
   are deliberately configured.
2. Confirm Issue #20 has provisioned the protected `qweather-release-smoke`
   Environment with exactly `QWEATHER_API_HOST` and `QWEATHER_API_KEY`, plus a
   required reviewer. Never put either value in a workflow input, repository
   variable, Issue, PR, summary, or artifact.
3. Confirm Issue #31 has completed the npm Trusted Publisher/bootstrap handoff
   and the `qweather-release-publish` Environment review path.
4. Recheck `npm view qweather-cli name version`. A successful lookup is a hard
   stop; an E404 is only a point-in-time availability result and must be
   repeated immediately before publication.
5. Dispatch `prepare-release.yml` with stable `X.Y.Z`. Review and merge the
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

1. Immediately repeat the npm name check and stop on any result other than
   E404/unclaimed.
2. Dispatch `publish.yml` with the version, gate run ID, and exact source SHA.
3. The workflow revalidates the gate, verifies all six artifacts without
   rebuilding, refuses an existing tag, creates the immutable tag and Draft
   GitHub Release, uploads/read-backs the assets, publishes the Release, and
   checks anonymous downloads.
4. Only after public asset verification does it publish the staged npm package
   with provenance. The first package may use the one-day bootstrap token from
   #31; later runs must use npm Trusted Publishing/OIDC and no stored npm token.
5. Verify global install, project-local install, `npx`, `qweather version
   --output json`, checksum selection on all six platforms, and shim stdout,
   stderr, exit-code, and signal forwarding.

## Stop conditions

- npm name is already published or registry availability is ambiguous;
- #20 or #31 is not complete;
- source SHA, release branch, gate run, artifact checksums, or VERSION disagree;
- any archive contains an unexpected entry or fails anonymous read-back;
- npm publish fails after a tag or public Release exists.

Never rewrite a tag or replace a published asset. Record the incident and
prepare a new patch version.
