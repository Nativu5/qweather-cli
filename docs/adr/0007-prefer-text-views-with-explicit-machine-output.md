---
status: accepted
---

# Prefer Text Views with explicit machine output

QWeather CLI defaults to a deterministic Text View selected by Capability, while `--output json` exposes the versioned Machine Result or Machine Problem and `--output body` writes an exact successful Provider Body. Text is readable by humans and Agents but is not a stable parsing contract; each Current Capability has an embedded, manually maintained entry template, and a generic remainder and fallback preserve fields that the primary layout does not consume. This supersedes ADR 0004 because a JSON envelope no longer needs to be the default merely to remain the stable machine interface.

The single global output choice replaces pretty-print and command-local format flags. Templates are compiled into the binary, cannot be overridden, never truncate arrays or convert provider values, and share common handling for No Data, cache and operation metadata, Resolved Place, units, and Attribution. Cache records continue to store only Provider Bodies and response metadata, so presentation is rebuilt on every invocation and output mode never affects cache identity.

JSON-default, lossy summaries, TTY-dependent formatting, runtime OpenAPI templates, and user-defined templates were rejected. JSON-default made routine diagnostics and weather answers harder to read; lossy summaries could hide data already paid for; environment-dependent or user-defined layouts would make Agent behaviour irreproducible; and OpenAPI describes possible shapes but cannot choose a useful presentation.
