---
status: accepted
---

# Use a Go CLI with Skill and npm adapters

The provider client is implemented as a Go CLI, while one concise `qweather` Skill teaches agent workflows and a small JavaScript npm adapter installs and launches release binaries. A TypeScript-only client would simplify npm distribution but give up the static, portable CLI binary; a Go-only distribution would not integrate cleanly with the Skills ecosystem. The adapters may describe, install, and invoke the Go binary, but they do not duplicate provider transport or query behaviour.
