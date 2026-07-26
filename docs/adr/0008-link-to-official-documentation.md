---
status: accepted
---

# Link to official documentation instead of vendoring upstream snapshots

The qweather Skill needs provider documentation only for occasional field,
schema, example, and policy questions. Its executable surface already comes
from the compiled Capability registry, while command, result-schema, and stable
problem-code references are generated from project-owned Go sources.

An earlier design distributed a fixed English and Chinese OpenAPI snapshot plus
all locally referenced JSON examples. That made offline lookup reproducible,
but also created a second synchronization, integrity, locale-consistency, and
licensing surface inside a user-facing Skill. The copied examples were evidence,
not part of normal command selection or the public CLI contract.

The Skill therefore links to current official QWeather web documentation and
does not distribute upstream OpenAPI files, response examples, manifests, or
source-content notices. Official documentation is evidence only: it cannot add
a command, revive a Tombstone, override project validation, or become a runtime
scraping dependency. The curated registry and generated project references
remain authoritative for executable behavior.

Maintainers may inspect a fixed official source revision under an ignored
`.cache/` checkout and record focused research under `docs/research/` when an
upstream change requires review. Those materials are not packaged with the
Skill.

This keeps the distributed Skill small and removes snapshot drift, at the cost
of requiring network access when an Agent needs provider documentation beyond
the curated references. Official pages may change independently, so volatile
facts retain direct links and verification dates, and behavior changes still
follow the upstream-change review procedure.
