# QWeather CLI

QWeather CLI exposes QWeather data as a stable, agent-friendly query model. The model keeps upstream transport versions, credential mechanics, response families, and data-product policy out of the caller's vocabulary.

## Query model

**Capability**:
A project-owned, stable read operation that represents one supported QWeather query. A current capability maps to exactly one current upstream endpoint.
_Avoid_: API, endpoint, action

**Query Target**:
The typed subject of a capability, such as a place, coordinate, air station, tide station, or storm. Target kinds are not interchangeable even when their identifiers look alike.
_Avoid_: Location, location string

**Place Spec**:
A caller-supplied place expressed as a human name, QWeather Location ID, or geographic coordinate. A Place Spec may need resolution before it is a valid Query Target.
_Avoid_: Location, address

**Resolved Place**:
An unambiguous place with the exact Location ID or coordinate required by a capability. A Resolved Place is scoped to the current invocation and is not a persistent geographic index.
_Avoid_: Selected city, cached location

**Location ID**:
A QWeather identifier for a city or geographic place. It is distinct from air-station IDs, tide-station IDs, and storm IDs.
_Avoid_: City code, station ID

**No Data**:
A successful query outcome for which QWeather has no matching records. No Data is not a transport, authentication, or validation failure.
_Avoid_: Error, empty failure

## Presentation

**Text View**:
A deterministic, readable presentation of one successful query or QWeather-owned Problem. It is not a machine compatibility contract, but it preserves every provider field through a Capability-specific primary layout and a generic remainder.
_Avoid_: Text schema, pretty JSON

**Machine Result**:
The versioned `qweather.result/v1` JSON representation of a successful query. It preserves the complete decoded Provider Body under `data` and is the stable field-level interface for automation.
_Avoid_: Default output, formatted body

**Machine Problem**:
The versioned `qweather.problem/v1` JSON representation of a QWeather-owned failure. Cobra invocation diagnostics are not Machine Problems.
_Avoid_: Cobra error, debug event

**Provider Body**:
The exact successful response bytes received from QWeather. Eligible Provider Bodies may be cached and may be written without a project envelope, but they are never a project-owned schema.
_Avoid_: Result, Text View

**Response Family**:
A project-owned structural classification of QWeather response envelopes: `code-refer-v1`, `metadata-v1`, or `console-v1`. A Response Family describes parsing shape, not whether a Capability is current or deprecated.
_Avoid_: Legacy API, modern API

## Lifecycle and policy

**Current Capability**:
A capability backed by a non-deprecated upstream endpoint and executable by the current CLI release.
_Avoid_: Active API

**Tombstone**:
A non-executable lifecycle record for a deprecated or removed upstream operation, including its replacement and sunset information when known.
_Avoid_: Legacy command, disabled capability

**Billing Group**:
QWeather's pricing category for a capability: Basic, Marine, or Solar. Basic identifies a price group and does not mean permanently free.
_Avoid_: Plan, subscription tier

**Product Gate**:
An explicit, non-interactive acknowledgement required before a Marine, Solar, or sensitive Account query may perform network I/O.
_Avoid_: Confirmation prompt

**Geo Data**:
Place and POI information returned by GeoAPI. Geo Data may be consumed during an invocation but is never persisted or indexed by this project.
_Avoid_: Location cache, city database

**Sensitive Account Data**:
Finance or request-usage data associated with a QWeather account. It requires a Product Gate and is not persistently cached by default.
_Avoid_: Diagnostics, health data

**Attribution**:
Provider, source, and license information that must remain associated with QWeather data when it is presented or reused.
_Avoid_: Metadata, credits
