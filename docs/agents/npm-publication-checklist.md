# npm publication checklist

Complete this checklist in order for each stable release.

## Before a version PR

1. Confirm the generated Skill and version-synchronization checks are green.
2. Recheck `npm view qweather-cli name repository.url`. The existing package
   must belong to this repository. A new requested version must be absent; a
   retry may accept it only after the workflow proves exact tarball integrity.
3. Dispatch `prepare-release.yml` with stable `X.Y.Z`. Review and merge the
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
   must point to this repository. Stop on any identity conflict or ambiguous
   registry response.
2. Dispatch `publish.yml` on the exact release branch with the version, gate run ID, and source SHA:

   ```sh
   gh workflow run publish.yml \
     --ref "release/vX.Y.Z" \
     --field version=X.Y.Z \
     --field gate_run_id=GATE_RUN_ID \
     --field source_sha=FULL_SOURCE_SHA
   ```
3. The workflow revalidates the gate and verifies all six artifacts without
   rebuilding. It creates absent tag/Release state and accepts exact existing
   state on retry. An existing annotated tag must peel to the gated source SHA;
   every existing Release asset must match the retained artifact digest. Only
   an exact draft may receive missing assets, and a public Release is never
   modified.
4. The workflow packs one exact npm `.tgz`. Only after public asset verification
   does it publish that tarball with provenance through npm Trusted
   Publishing/OIDC. If the version is already visible after an ambiguous prior
   result, the workflow succeeds only when repository identity and
   `dist.integrity` match. Do not add or restore a stored npm token.
5. Verify global install, project-local install, `npx`, `qweather version
   --output json`, checksum selection on all six platforms, and shim stdout,
   stderr, exit-code, and signal forwarding.

## Retry

Rerun a failed publication on the same `release/vX.Y.Z` ref with the same three
inputs. Exact durable state is accepted; absent state is completed. Never
change the gate run or source SHA merely to make a retry pass.

## Stop conditions

- package ownership is unexpected or registry availability is ambiguous;
- the generated Skill checks are not green;
- source SHA, release branch, gate run, artifact checksums, or VERSION disagree;
- any archive contains an unexpected entry or fails anonymous read-back;
- an existing tag does not peel to the gated SHA, a Release asset differs or a
  public Release is incomplete, or an existing npm version has different
  repository identity or tarball integrity;
- an npm failure remains observable after reconciliation; rerun only after the
  publisher or transient service problem is corrected.

Never rewrite a tag or replace a published asset. Exact state may be retried;
mismatched state requires an incident record and a new patch version.
