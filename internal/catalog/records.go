package catalog

import "time"

const (
	evidenceOfficial     = "official QWeather caching recommendation"
	evidenceConservative = "project conservative derivation from provider update cycle"
	evidenceGeoPolicy    = "GeoAPI persistent storage restriction"
	evidenceSensitive    = "project sensitive-account policy"
)

func policy(mode CacheMode, ttl time.Duration, boundary CacheBoundary, evidence string) CachePolicy {
	return CachePolicy{Mode: mode, TTL: ttl, Boundary: boundary, Evidence: evidence}
}

func upstream(path string, family ResponseFamily) Upstream {
	return Upstream{Method: "GET", PathTemplate: path, ResponseFamily: family}
}

func current(
	id, commandPath, domain, summary, docs string,
	target TargetKind,
	flags []Flag,
	transport Upstream,
	billing BillingGroup,
	gate ProductGate,
	cache CachePolicy,
) Capability {
	return Capability{
		ID:              id,
		CommandPath:     commandPath,
		Domain:          domain,
		Summary:         summary,
		DocsURL:         docs,
		Lifecycle:       LifecycleCurrent,
		Target:          target,
		Flags:           flags,
		Upstream:        transport,
		BillingGroup:    billing,
		ProductGate:     gate,
		Cache:           cache,
		RequestRevision: 1,
	}
}

func tombstone(id, domain, summary, docs, path, replacement string, billing BillingGroup) Capability {
	return Capability{
		ID:           id,
		Domain:       domain,
		Summary:      summary,
		DocsURL:      docs,
		Lifecycle:    LifecycleDeprecated,
		Replacement:  replacement,
		Target:       TargetNone,
		Upstream:     upstream(path, ResponseCodeReferV1),
		BillingGroup: billing,
		ProductGate:  GateNone,
		Cache:        policy(CacheDisabled, 0, BoundaryNone, "non-executable Tombstone"),
	}
}

func records() []Capability {
	geoCache := policy(CacheDisabled, 0, BoundaryNone, evidenceGeoPolicy)
	basic10m := policy(CacheEnabled, 10*time.Minute, BoundaryNone, evidenceOfficial)
	basic30m := policy(CacheEnabled, 30*time.Minute, BoundaryNone, evidenceOfficial)

	limitFlag := rangedIntFlag("limit", "maximum result count", false, 1, 20)
	langFlag := stringFlag("lang", "response language", false)
	coordinateFlag := stringFlag("coordinate", "coordinate in geo:<lat>,<lon> form", true)
	poiTypeFlag := stringFlag("poi-type", "POI kind", true, "scenic", "tide-station")
	allowMarine := stringFlag("allow-product", "acknowledge a billed product", false, "marine")
	allowSolar := stringFlag("allow-product", "acknowledge a billed product", false, "solar")
	allowAccount := stringFlag("allow-sensitive-output", "acknowledge sensitive account output", false, "account")
	solarHours := rangedIntFlag("hours", "forecast length in hours", false, 1, 60)
	solarHours.Default = "24"
	solarInterval := intFlag("interval-min", "forecast interval in minutes", false, 15, 30, 60)
	solarInterval.Default = "60"

	return []Capability{
		current(
			"geo.city.lookup", "geo city lookup", "geo", "Look up a city or place",
			"https://dev.qweather.com/docs/api/geoapi/city-lookup/", TargetGeoLookup,
			[]Flag{
				stringFlag("query", "place name or search text", false),
				stringFlag("place-id", "QWeather Location ID", false),
				stringFlag("coordinate", "coordinate in geo:<lat>,<lon> form", false),
				stringFlag("country", "ISO-3166 country or region filter", false),
				stringFlag("adm", "administrative-area filter", false),
				limitFlag,
				langFlag,
			},
			upstream("/geo/v2/city/lookup", ResponseCodeReferV1), BillingBasic, GateNone, geoCache,
		),
		current(
			"geo.city.top", "geo city top", "geo", "List top cities",
			"https://dev.qweather.com/docs/api/geoapi/top-city/", TargetNone,
			[]Flag{stringFlag("country", "ISO-3166 country or region filter", false), limitFlag, langFlag},
			upstream("/geo/v2/city/top", ResponseCodeReferV1), BillingBasic, GateNone, geoCache,
		),
		current(
			"geo.poi.lookup", "geo poi lookup", "geo", "Look up a point of interest",
			"https://dev.qweather.com/docs/api/geoapi/poi-lookup/", TargetNone,
			[]Flag{
				stringFlag("query", "POI name or search text", true), poiTypeFlag,
				stringFlag("adm", "city or administrative-area filter", false), limitFlag, langFlag,
			},
			upstream("/geo/v2/poi/lookup", ResponseCodeReferV1), BillingBasic, GateNone, geoCache,
		),
		current(
			"geo.poi.nearby", "geo poi nearby", "geo", "Find nearby points of interest",
			"https://dev.qweather.com/docs/api/geoapi/poi-range/", TargetCoordinate,
			[]Flag{coordinateFlag, poiTypeFlag, limitFlag, langFlag},
			upstream("/geo/v2/poi/range", ResponseCodeReferV1), BillingBasic, GateNone, geoCache,
		),

		current(
			"weather.city.current", "weather city current", "weather", "Get current city weather",
			"https://dev.qweather.com/docs/api/weather/weather-now/", TargetPlace,
			placeFlags(true, false), upstream("/v7/weather/now", ResponseCodeReferV1),
			BillingBasic, GateNone, basic10m,
		),
		current(
			"weather.city.forecast.daily", "weather city daily", "weather", "Get daily city weather forecast",
			"https://dev.qweather.com/docs/api/weather/weather-daily-forecast/", TargetPlace,
			appendFlags(placeFlags(true, false), intFlag("days", "forecast length in days", true, 3, 7, 10, 15, 30)),
			upstream("/v7/weather/{days}", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, time.Hour, BoundaryLocalDay, evidenceOfficial),
		),
		current(
			"weather.city.forecast.hourly", "weather city hourly", "weather", "Get hourly city weather forecast",
			"https://dev.qweather.com/docs/api/weather/weather-hourly-forecast/", TargetPlace,
			appendFlags(placeFlags(true, false), intFlag("hours", "forecast length in hours", true, 24, 72, 168)),
			upstream("/v7/weather/{hours}", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, 30*time.Minute, BoundaryLocalHour, evidenceOfficial),
		),
		current(
			"weather.grid.current", "weather grid current", "weather", "Get current grid weather",
			"https://dev.qweather.com/docs/api/weather/grid-weather-now/", TargetCoordinate,
			placeFlags(true, true), upstream("/v7/grid-weather/now", ResponseCodeReferV1),
			BillingBasic, GateNone, basic10m,
		),
		current(
			"weather.grid.forecast.daily", "weather grid daily", "weather", "Get daily grid weather forecast",
			"https://dev.qweather.com/docs/api/weather/grid-weather-daily-forecast/", TargetCoordinate,
			appendFlags(placeFlags(true, true), intFlag("days", "forecast length in days", true, 3, 7)),
			upstream("/v7/grid-weather/{days}", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, time.Hour, BoundaryUTCDay, evidenceConservative),
		),
		current(
			"weather.grid.forecast.hourly", "weather grid hourly", "weather", "Get hourly grid weather forecast",
			"https://dev.qweather.com/docs/api/weather/grid-weather-hourly-forecast/", TargetCoordinate,
			appendFlags(placeFlags(true, true), intFlag("hours", "forecast length in hours", true, 24, 72)),
			upstream("/v7/grid-weather/{hours}", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, 30*time.Minute, BoundaryUTCHour, evidenceConservative),
		),
		current(
			"weather.precipitation.minutely", "weather minutely", "weather", "Get minutely precipitation",
			"https://dev.qweather.com/docs/api/minutely/minutely-precipitation/", TargetCoordinate,
			placeFlags(true, false), upstream("/v7/minutely/5m", ResponseCodeReferV1),
			BillingBasic, GateNone, policy(CacheEnabled, 5*time.Minute, BoundaryNone, evidenceOfficial),
		),
		current(
			"weather.indices.forecast", "weather indices", "weather", "Get weather life indices",
			"https://dev.qweather.com/docs/api/indices/indices-forecast/", TargetPlace,
			appendFlags(placeFlags(true, false),
				intFlag("days", "forecast length in days", true, 1, 3),
				rangedIntSliceFlag("index", "weather index type; repeatable", 1, 16),
				boolFlag("all-indices", "request every weather index type"),
			),
			upstream("/v7/indices/{days}", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, 6*time.Hour, BoundaryLocalDay, evidenceOfficial),
		),
		current(
			"weather.history", "weather history", "weather", "Get historical daily weather",
			"https://dev.qweather.com/docs/api/time-machine/time-machine-weather/", TargetLocationID,
			appendFlags(placeFlags(true, true), stringFlag("date", "date in YYYY-MM-DD form", true)),
			upstream("/v7/historical/weather", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, 24*time.Hour, BoundaryNone, evidenceConservative),
		),

		current(
			"alert.current", "alert current", "alert", "Get current weather alerts",
			"https://dev.qweather.com/docs/api/warning/weather-alert/", TargetCoordinate,
			appendFlags(placeFlags(true, false), boolFlag("local-time", "return local timestamps")),
			upstream("/weatheralert/v1/current/{latitude}/{longitude}", ResponseMetadataV1),
			BillingBasic, GateNone, policy(CacheEnabled, 5*time.Minute, BoundaryNone, evidenceOfficial),
		),

		current(
			"air.current", "air current", "air", "Get current air quality",
			"https://dev.qweather.com/docs/api/air-quality/air-current/", TargetCoordinate,
			placeFlags(true, false), upstream("/airquality/v1/current/{latitude}/{longitude}", ResponseMetadataV1),
			BillingBasic, GateNone, basic30m,
		),
		current(
			"air.forecast.daily", "air daily", "air", "Get daily air-quality forecast",
			"https://dev.qweather.com/docs/api/air-quality/air-daily-forecast/", TargetCoordinate,
			placeFlags(true, false), upstream("/airquality/v1/daily/{latitude}/{longitude}", ResponseMetadataV1),
			BillingBasic, GateNone, policy(CacheEnabled, 8*time.Hour, BoundaryLocalDay, evidenceOfficial),
		),
		current(
			"air.forecast.hourly", "air hourly", "air", "Get hourly air-quality forecast",
			"https://dev.qweather.com/docs/api/air-quality/air-hourly-forecast/", TargetCoordinate,
			placeFlags(true, false), upstream("/airquality/v1/hourly/{latitude}/{longitude}", ResponseMetadataV1),
			BillingBasic, GateNone, policy(CacheEnabled, 30*time.Minute, BoundaryLocalHour, evidenceOfficial),
		),
		current(
			"air.station.current", "air station", "air", "Get current air quality at a station",
			"https://dev.qweather.com/docs/api/air-quality/air-station/", TargetAirStation,
			[]Flag{stringFlag("air-station-id", "air-quality monitoring station ID", true), langFlag},
			upstream("/airquality/v1/stations/{locationId}", ResponseMetadataV1), BillingBasic, GateNone, basic30m,
		),

		current(
			"storm.list", "storm list", "storm", "List tropical storms",
			"https://dev.qweather.com/docs/api/tropical-cyclone/storm-list/", TargetNone,
			[]Flag{intFlag("year", "current or previous UTC calendar year", true), allowMarine},
			upstream("/v7/tropical/storm-list", ResponseCodeReferV1), BillingMarine, GateMarine,
			CachePolicy{Mode: CacheEnabled, TTL: 20 * time.Minute, InactiveTTL: time.Hour, Boundary: BoundaryNone, Evidence: evidenceOfficial},
		),
		current(
			"storm.track", "storm track", "storm", "Get a tropical storm track",
			"https://dev.qweather.com/docs/api/tropical-cyclone/storm-track/", TargetStorm,
			[]Flag{stringFlag("storm-id", "QWeather storm ID", true), allowMarine},
			upstream("/v7/tropical/storm-track", ResponseCodeReferV1), BillingMarine, GateMarine,
			CachePolicy{Mode: CacheEnabled, TTL: 20 * time.Minute, InactiveTTL: time.Hour, Boundary: BoundaryNone, Evidence: evidenceOfficial},
		),
		current(
			"storm.forecast", "storm forecast", "storm", "Get a tropical storm forecast",
			"https://dev.qweather.com/docs/api/tropical-cyclone/storm-forecast/", TargetStorm,
			[]Flag{stringFlag("storm-id", "QWeather storm ID", true), allowMarine},
			upstream("/v7/tropical/storm-forecast", ResponseCodeReferV1), BillingMarine, GateMarine,
			CachePolicy{Mode: CacheEnabled, TTL: 20 * time.Minute, InactiveTTL: time.Hour, Boundary: BoundaryNone, Evidence: evidenceOfficial},
		),
		current(
			"marine.tide", "marine tide", "marine", "Get tide forecasts for a station",
			"https://dev.qweather.com/docs/api/ocean/tide/", TargetTideStation,
			[]Flag{stringFlag("tide-station-id", "QWeather tide station ID", true), stringFlag("date", "UTC date from today through 9 days ahead in YYYY-MM-DD form", true), allowMarine},
			upstream("/v7/ocean/tide", ResponseCodeReferV1), BillingMarine, GateMarine,
			policy(CacheEnabled, 8*time.Hour, BoundaryNone, evidenceOfficial),
		),
		current(
			"solar.radiation.forecast", "solar forecast", "solar", "Get solar-radiation forecast",
			"https://dev.qweather.com/docs/api/solar-radiation/solar-radiation-forecast/", TargetCoordinate,
			appendFlags(placeFlags(false, false),
				solarHours,
				solarInterval,
				stringSliceFlag("include", "optional dataset; repeatable", "weather", "poa"),
				rangedIntFlag("tilt-deg", "panel tilt in degrees", false, 0, 90),
				rangedIntFlag("azimuth-deg", "panel azimuth in degrees", false, 0, 359),
				boolFlag("local-time", "return local timestamps"), allowSolar,
			),
			upstream("/solarradiation/v1/forecast/{latitude}/{longitude}", ResponseMetadataV1),
			BillingSolar, GateSolar, policy(CacheEnabled, 6*time.Hour, BoundaryNone, evidenceOfficial),
		),
		current(
			"astronomy.sun.events", "astronomy sun", "astronomy", "Get sunrise and sunset",
			"https://dev.qweather.com/docs/api/astronomy/sunrise-sunset/", TargetPlace,
			appendFlags(placeFlags(false, false), stringFlag("date", "UTC date from today through 59 days ahead in YYYY-MM-DD form", true)),
			upstream("/v7/astronomy/sun", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, 24*time.Hour, BoundaryNone, evidenceConservative),
		),
		current(
			"astronomy.moon.events", "astronomy moon", "astronomy", "Get moonrise, moonset, and phases",
			"https://dev.qweather.com/docs/api/astronomy/moon-and-moon-phase/", TargetPlace,
			appendFlags(placeFlags(true, false), stringFlag("date", "UTC date from today through 59 days ahead in YYYY-MM-DD form", true)),
			upstream("/v7/astronomy/moon", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, 24*time.Hour, BoundaryNone, evidenceConservative),
		),
		current(
			"astronomy.solar.position", "astronomy position", "astronomy", "Get solar position",
			"https://dev.qweather.com/docs/api/astronomy/solar-elevation-angle/", TargetCoordinate,
			appendFlags(placeFlags(false, false),
				stringFlag("at", "timestamp in RFC3339 form", true),
				floatFlag("altitude-m", "altitude in meters", true, -500, 9000),
			),
			upstream("/v7/astronomy/solar-elevation-angle", ResponseCodeReferV1), BillingBasic, GateNone,
			policy(CacheEnabled, 24*time.Hour, BoundaryNone, evidenceConservative),
		),
		current(
			"account.finance.summary", "account finance", "account", "Get account finance summary",
			"https://dev.qweather.com/docs/api/console/finance/", TargetNone,
			[]Flag{allowAccount}, upstream("/finance/v1/summary", ResponseConsoleV1),
			BillingBasic, GateSensitiveAccount, policy(CacheSensitive, 10*time.Minute, BoundaryNone, evidenceSensitive),
		),
		current(
			"account.requests.stats", "account usage", "account", "Get account request statistics",
			"https://dev.qweather.com/docs/api/console/stats/", TargetNone,
			[]Flag{
				stringFlag("project-id", "filter by project ID", false),
				stringFlag("credential-id", "filter by credential ID", false),
				allowAccount,
			},
			upstream("/metrics/v1/stats", ResponseConsoleV1), BillingBasic, GateSensitiveAccount,
			policy(CacheSensitive, 10*time.Minute, BoundaryNone, evidenceSensitive),
		),

		tombstone("legacy.alert.current", "alert", "Deprecated legacy weather alert", "https://dev.qweather.com/docs/deprecated/", "/v7/warning/now", "alert.current", BillingBasic),
		tombstone("legacy.air.current", "air", "Deprecated legacy current air quality", "https://dev.qweather.com/docs/deprecated/", "/v7/air/now", "air.current", BillingBasic),
		tombstone("legacy.air.forecast.daily", "air", "Deprecated legacy daily air forecast", "https://dev.qweather.com/docs/deprecated/", "/v7/air/5d", "air.forecast.daily", BillingBasic),
		tombstone("legacy.solar.radiation.forecast", "solar", "Deprecated legacy solar radiation forecast", "https://dev.qweather.com/docs/deprecated/", "/v7/solar-radiation/{hours}", "solar.radiation.forecast", BillingSolar),
		tombstone("legacy.air.history", "air", "Deprecated historical air quality", "https://dev.qweather.com/docs/deprecated/", "/v7/historical/air", "", BillingBasic),
	}
}
