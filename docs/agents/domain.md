# Domain Docs

This repository uses a single-context domain-documentation layout.

## Before exploring, read these

- `CONTEXT.md` at the repository root.
- Relevant ADRs under `docs/adr/`.
- [`docs/design/architecture.md`](../design/architecture.md) and the linked contract relevant to the task.

If these files do not exist, proceed silently. Domain-modeling workflows create
them lazily when terminology or architectural decisions are resolved.

## Layout

```
/
├── CONTEXT.md
├── docs/
│   └── adr/
└── internal/
```

## Use the glossary’s vocabulary

When naming domain concepts in issues, proposals, tests, or code, use the terms
defined in `CONTEXT.md`. Avoid synonyms the glossary explicitly rejects.

If a needed concept is absent, reconsider whether new terminology is necessary
or record the gap for the domain-modeling workflow.

## Flag ADR conflicts

If proposed work contradicts an existing ADR, surface the conflict explicitly
instead of silently overriding the decision.
