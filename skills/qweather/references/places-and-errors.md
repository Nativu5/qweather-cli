# Places, targets, and errors

Last verified: 2026-07-27

## Place Specs

For a place-aware command, pass exactly one:

| Form | Use |
| --- | --- |
| `--place <name>` | Resolve human text during this invocation. |
| `--place-id <id>` | Use a known QWeather Location ID. |
| `--coordinate geo:<latitude>,<longitude>` | Use CLI order: latitude, then longitude. |

Use `--country <ISO-3166>` or `--adm <text>` only with `--place`. Prefer an
exact Location ID or coordinate for automation because it avoids an additional
Geo request and ambiguity.

The `geo:` prefix is required. The current CLI accepts decimal coordinates
with at most two fractional digits. Round coordinates from another source to
that limit before passing them, and do not imply greater location precision in
the answer.

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

For `geo poi lookup` results, inspect candidate names and administrative areas.
Never select the first provider result merely because it is first. Continue
only when a candidate's normalized name and administrative area match the
place the user intended; ask the user when several candidates remain
plausible. If every candidate is unrelated, treat resolution as failed and do
not issue a weather query for any of them.

On `PLACE_NOT_FOUND`, an unrelated POI result, or another unresolved place,
ask the user for a Location ID or coordinate. Alternatively, use an available
map capability such as AMap to locate the intended place, then disclose that
source with the latitude-first coordinate. Do not silently substitute a nearby
or similarly named place.

The [generated command reference](command-reference.md) marks Geo capabilities
`cache: disabled`; each provider-bound lookup is an API request under
[QWeather pricing](https://dev.qweather.com/docs/finance/pricing/). Reuse an
exact Location ID or coordinate already supplied or resolved in the current
task instead of repeating a Geo lookup. Geo Data remains invocation-scoped and
must not be persisted or indexed.

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

For a Geo failure with `retryable: false`, never repeat the same request. Ask
for a coordinate or Location ID, refine the place input, or use a disclosed
external map source instead.

Read `result-schema.md` directly from the Skill workflow for the generated code
table. Messages are human presentation and are not stable decision fields.
