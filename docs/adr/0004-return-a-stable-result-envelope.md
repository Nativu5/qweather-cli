---
status: superseded by ADR-0007
---

# Return a stable envelope around provider data

Default success output is a versioned `qweather.result/v1` envelope containing capability, resolved-place, policy, cache, upstream, Attribution, and operation metadata, with the complete provider JSON preserved under `data`. Raw-provider-only output remains available through `--output body`. This avoids leaking three incompatible provider response families into every agent workflow without committing the project to a lossy unified weather schema.
