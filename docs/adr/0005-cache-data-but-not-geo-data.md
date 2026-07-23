---
status: accepted
---

# Cache data but not Geo Data

Eligible data capabilities use persistent, capability-specific hard TTLs by default to reduce repeated requests, while all Geo capabilities are permanently excluded because QWeather's license does not grant GeoAPI storage or caching rights. Sensitive Account data is excluded unless explicitly enabled, errors and expired bodies are not returned as cache fallbacks, and users can refresh, bypass, inspect, or clear the cache. A single global TTL was rejected because provider update rates range from minutes to a day.
