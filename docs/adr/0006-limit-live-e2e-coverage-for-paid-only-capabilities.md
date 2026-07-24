---
status: accepted
---

# Limit live E2E coverage to capabilities with a free allowance

Capabilities whose QWeather Billing Group has no zero-price allowance are excluded from complete live end-to-end testing because routine verification would create unavoidable provider charges. This currently applies to `storm.list`, `storm.track`, `storm.forecast`, `marine.tide`, and `solar.radiation.forecast`; the classification must be re-evaluated against [QWeather pricing](https://dev.qweather.com/docs/finance/pricing/) when the upstream contract changes.

These capabilities still require deterministic unit and contract tests for registry metadata, validation, request construction, response handling, and error behaviour. However, because the project does not verify their full authentication-to-provider-response path, it does not guarantee their end-to-end functional completeness. Release smoke coverage remains limited to representative capabilities whose Billing Group provides a free request allowance.
