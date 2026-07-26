# Places, targets, and errors

Last verified: 2026-07-26

## Place Specs

For a place-aware command, pass exactly one:

| Form | Use |
| --- | --- |
| `--place <name>` | Resolve human text during this invocation. |
| `--place-id <id>` | Use a known QWeather Location ID. |
| `--coordinate geo:<lat>,<lon>` | Use RFC 5870 order: latitude, then longitude. |

Use `--country <ISO-3166>` or `--adm <text>` only with `--place`. Prefer an
exact Location ID or coordinate for automation because it avoids an additional
Geo request and ambiguity.

Geo lookup commands have their own inputs. For example, `geo city lookup` uses
exactly one of `--query`, `--place-id`, or `--coordinate`; consult the generated
command reference instead of projecting Place Spec flags onto every Geo command.

## Safe ambiguity handling

The CLI continues only for one provider candidate or one normalized exact-name
candidate. It never chooses by provider order or rank.

On `AMBIGUOUS_PLACE`:

1. inspect the safe candidate details;
2. ask the user to choose, or refine with `--country`/`--adm`;
3. rerun with the selected `--place-id` or coordinate.

On `PLACE_NOT_FOUND`, ask for a spelling, administrative area, Location ID, or
coordinate. Do not silently substitute a nearby or similarly named place.
Geo Data is invocation-scoped and must not be persisted or indexed.

## Domain-specific targets

Do not interchange opaque identifiers:

- `--air-station-id` selects an air-quality monitoring station;
- `--tide-station-id` selects a tide station;
- `--storm-id` selects a tropical storm.

These values are not Location IDs and must not be inferred from string shape.

## Failures

QWeather-owned failures write no stdout data. Under `--output json`, stderr
contains one compact `qweather.problem/v1` object. Cobra parsing and command
discovery errors remain Text diagnostics.

Use this decision order:

1. inspect the process exit status;
2. for a Machine Problem, branch on `code` and read complete safe `details`;
3. retry only when the emitted `retryable` value is true and the user wants a
   deliberate retry;
4. never expose debug output containing credentials or provider error bodies.

Read `result-schema.md` directly from the Skill workflow for the generated code
table. Messages are human presentation and are not stable decision fields.
