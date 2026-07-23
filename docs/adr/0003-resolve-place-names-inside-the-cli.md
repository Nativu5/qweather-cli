---
status: accepted
---

# Resolve place names inside the CLI

Place-aware commands accept a Place Spec and resolve human names inside the Go application module. Resolution continues only for an unambiguous candidate; otherwise it returns structured candidates and requires an exact retry, never choosing by provider order or rank. This can add one visible Geo request, but it centralizes a fragile workflow that would otherwise be repeated in Skill prompts and callers; Geo Data is consumed only for the current invocation.
