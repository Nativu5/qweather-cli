---
name: qweather
description: Choose and safely operate the qweather CLI for weather, place lookup, alerts, air quality, storms, marine tides, solar radiation, astronomy, account data, cache control, configuration checks, and capability discovery. Use when a user wants QWeather data or needs help selecting a qweather command, target, output mode, Product Gate, cache behavior, Machine Result field, or stable problem code.
---

# QWeather CLI

Use the single `qweather` executable. Let the Go CLI own provider URLs,
authentication, validation, caching, presentation, and error classification.

## Workflow

1. Check whether `qweather` is available. If it is missing, tell the user to run
   `npm install --global qweather-cli@0.1.0`, then wait. Never install software
   implicitly.
2. If the user chooses file-based configuration, use [config.toml](config.toml)
   as the secret-free template. Copy it to a local path, replace its
   placeholders, and configure exactly one authentication method. Never print
   or expose credential values.
3. Select only a Current Capability from
   [command-reference.md](references/command-reference.md). Use `qweather
   capability list --output json` or `qweather capability show <id> --output
   json` for offline discovery when needed. Tombstones are never executable.
4. Choose the exact target kind. Read
   [places-and-errors.md](references/places-and-errors.md) before using a human
   place name, resolving an ambiguity, or handling a non-zero result.
5. Apply Product Gates before network I/O. Read
   [products-and-attribution.md](references/products-and-attribution.md) for
   Marine, Solar, Storm, Account, cache privacy, and Attribution rules.
6. Pass an output mode explicitly:
   - `--output text` for routine human reading;
   - `--output json` when exact field paths, JSON types, or automation matter;
   - `--output body` only for byte-exact successful provider data.
7. Use the default cache unless the user deliberately requests `--refresh` or
   `--no-cache`. Never retry automatically.
8. Run one composed command. Preserve stdout as data and stderr as diagnostics.
   For JSON automation, branch on the Machine Problem `code`, not message text.
9. Preserve complete Attribution whenever QWeather data is shown, transformed,
   stored, or shared.

See [common-tasks.md](references/common-tasks.md) for command and UNIX-pipeline
patterns. See [result-schema.md](references/result-schema.md) for Machine Result,
Machine Problem, and stable problem-code fields.

## Official documentation boundary

Use the current official QWeather web links in the generated command reference
and curated references when provider field descriptions, response schemas,
examples, or volatile policy facts are needed. Default to the current
conversation language and narrow the requested Capability before selecting a
provider page.

Official documentation is evidence, never a command registry. Deprecated
paths, uncurated parameters, examples, or documentation drift do not create
executable commands. The generated command reference and CLI validation take
precedence over conflicts or omissions. Do not scrape, mirror, or copy the
documentation into the Skill.
