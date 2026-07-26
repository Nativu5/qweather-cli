# Products, caching, and Attribution

Last verified: 2026-07-26

## Product Gates

Product Gates are acknowledgements enforced by the binary before any network
or optional Geo request. They are not interactive prompts.

| Surface | Required acknowledgement |
| --- | --- |
| Basic weather, Geo, alerts, air, and astronomy | none |
| Storm and Marine tide | `--allow-product marine` |
| Solar radiation | `--allow-product solar` |
| Account finance or usage | `--allow-sensitive-output account` |

Add a gate only when the user's requested operation clearly selects that
surface. Never use Account as a credential probe; it returns Sensitive Account
Data. Treat `--allow-sensitive-output account` as the user's explicit
acknowledgement, not as provider authorization. When consent is conditional or
unclear, ask before adding it or making the request. Confirm current commercial
terms through the official [pricing](https://dev.qweather.com/docs/finance/pricing/)
documentation rather than describing Basic as permanently free.

## Cache and privacy boundaries

- Geo responses, candidate lists, queries, POIs, and place mappings are never
  persistently cached.
- Ordinary eligible data uses the Capability-specific cache policy by default.
- Account data is not persistently cached unless the user has explicitly opted
  into sensitive caching.
- `--refresh` skips a read and replaces an eligible success; `--no-cache` skips
  both read and write.
- Errors are not cached, and expired data is not returned automatically after
  a network failure.

See the current official [caching guidance](https://dev.qweather.com/docs/best-practices/cache/)
and [GeoAPI restriction](https://dev.qweather.com/docs/terms/restriction/#cache-or-index-geoapi-data)
for volatile provider policy.

## Attribution

Preserve complete Attribution whenever QWeather data is presented, reused,
stored, transformed, or shared. Text View renders Attribution; Machine Result
also retains provider attribution fields inside `data` and exposes a convenient
`attribution` array. Do not discard either merely because the output is being
reformatted.

Follow the current official [Attribution requirements](https://dev.qweather.com/docs/terms/attribution/).
The bundled OpenAPI snapshot has its own pinned source and CC BY 4.0 notice at
`upstream/openapi/NOTICE.md`. API access, returned data, credentials, and service
use remain separately governed by the QWeather Developers EULA and current
QWeather terms.
