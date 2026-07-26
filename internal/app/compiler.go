package app

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/Nativu5/qweather-cli/internal/place"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

var providerPathID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// CompileRequest maps the curated public contract to one provider GET request.
func CompileRequest(capability catalog.Capability, parameters RequestParameters) (qweather.Request, *output.Problem) {
	query := make(url.Values)
	request := qweather.Request{CapabilityID: capability.ID}
	input := parameters.Input

	switch capability.ID {
	case "geo.city.lookup":
		location, problem := geoLookupLocation(input)
		if problem != nil {
			return qweather.Request{}, problemForCapability(problem, capability.ID)
		}
		request.Path = "/geo/v2/city/lookup"
		query.Set("location", location)
		if strings.TrimSpace(input.Query) != "" {
			if value := providerCountry(input.Country); value != "" {
				query.Set("range", value)
			}
			if value := strings.TrimSpace(input.Adm); value != "" {
				query.Set("adm", value)
			}
		} else if strings.TrimSpace(input.Country) != "" || strings.TrimSpace(input.Adm) != "" {
			return qweather.Request{}, invalidRequest(capability.ID, "--country and --adm require --query")
		}
		if problem := setLimit(query, input.Limit, capability.ID); problem != nil {
			return qweather.Request{}, problem
		}
		setLanguage(query, parameters.Language)

	case "geo.city.top":
		request.Path = "/geo/v2/city/top"
		if value := providerCountry(input.Country); value != "" {
			query.Set("range", value)
		}
		if problem := setLimit(query, input.Limit, capability.ID); problem != nil {
			return qweather.Request{}, problem
		}
		setLanguage(query, parameters.Language)

	case "geo.poi.lookup":
		location := strings.TrimSpace(input.Query)
		if location == "" {
			return qweather.Request{}, invalidRequest(capability.ID, "--query is required")
		}
		poiType, problem := providerPOIType(input.POIType, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = "/geo/v2/poi/lookup"
		query.Set("location", location)
		query.Set("type", poiType)
		if value := strings.TrimSpace(input.Adm); value != "" {
			query.Set("city", value)
		}
		if problem := setLimit(query, input.Limit, capability.ID); problem != nil {
			return qweather.Request{}, problem
		}
		setLanguage(query, parameters.Language)

	case "geo.poi.nearby":
		coordinate, problem := requireCoordinate(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		poiType, problem := providerPOIType(input.POIType, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = "/geo/v2/poi/range"
		query.Set("location", coordinate.ProviderQuery())
		query.Set("type", poiType)
		if problem := setLimit(query, input.Limit, capability.ID); problem != nil {
			return qweather.Request{}, problem
		}
		setLanguage(query, parameters.Language)

	case "weather.city.current":
		location, problem := requirePlace(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = "/v7/weather/now"
		query.Set("location", location)
		setLanguage(query, parameters.Language)

	case "weather.city.forecast.daily":
		if !slices.Contains([]int{3, 7, 10, 15, 30}, input.Days) {
			return qweather.Request{}, invalidRequest(capability.ID, "--days has an unsupported value")
		}
		location, problem := requirePlace(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = fmt.Sprintf("/v7/weather/%dd", input.Days)
		query.Set("location", location)
		setLanguage(query, parameters.Language)

	case "weather.city.forecast.hourly":
		if !slices.Contains([]int{24, 72, 168}, input.Hours) {
			return qweather.Request{}, invalidRequest(capability.ID, "--hours has an unsupported value")
		}
		location, problem := requirePlace(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = fmt.Sprintf("/v7/weather/%dh", input.Hours)
		query.Set("location", location)
		setLanguage(query, parameters.Language)

	case "weather.grid.current", "weather.grid.forecast.daily", "weather.grid.forecast.hourly":
		coordinate, problem := requireCoordinate(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		switch capability.ID {
		case "weather.grid.current":
			request.Path = "/v7/grid-weather/now"
		case "weather.grid.forecast.daily":
			if !slices.Contains([]int{3, 7}, input.Days) {
				return qweather.Request{}, invalidRequest(capability.ID, "--days has an unsupported value")
			}
			request.Path = fmt.Sprintf("/v7/grid-weather/%dd", input.Days)
		case "weather.grid.forecast.hourly":
			if !slices.Contains([]int{24, 72}, input.Hours) {
				return qweather.Request{}, invalidRequest(capability.ID, "--hours has an unsupported value")
			}
			request.Path = fmt.Sprintf("/v7/grid-weather/%dh", input.Hours)
		}
		query.Set("location", coordinate.ProviderQuery())
		setLanguage(query, parameters.Language)
		unit, problem := providerUnit(parameters.Unit, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		query.Set("unit", unit)

	case "weather.precipitation.minutely":
		coordinate, problem := requireCoordinate(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		if problem := validateLimitedLanguage(parameters.Language, capability.ID); problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = "/v7/minutely/5m"
		query.Set("location", coordinate.ProviderQuery())
		setLanguage(query, parameters.Language)

	case "weather.indices.forecast":
		if !slices.Contains([]int{1, 3}, input.Days) {
			return qweather.Request{}, invalidRequest(capability.ID, "--days has an unsupported value")
		}
		location, problem := requirePlace(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		indexTypes, problem := providerIndices(input, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		if problem := validateLimitedLanguage(parameters.Language, capability.ID); problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = fmt.Sprintf("/v7/indices/%dd", input.Days)
		query.Set("type", indexTypes)
		query.Set("location", location)
		setLanguage(query, parameters.Language)

	case "weather.history":
		locationID := strings.TrimSpace(parameters.Resolved.ID)
		if locationID == "" {
			return qweather.Request{}, invalidRequest(capability.ID, "a resolved Location ID is required")
		}
		date, problem := providerDate(input.Date, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		unit, problem := providerUnit(parameters.Unit, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = "/v7/historical/weather"
		query.Set("location", locationID)
		query.Set("date", date)
		query.Set("unit", unit)
		setLanguage(query, parameters.Language)

	case "alert.current":
		coordinate, problem := requireCoordinate(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = latLonCoordinatePath("/weatheralert/v1/current", coordinate)
		if input.LocalTime {
			query.Set("localTime", "true")
		}
		setLanguage(query, parameters.Language)

	case "air.current", "air.forecast.daily", "air.forecast.hourly":
		coordinate, problem := requireCoordinate(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		base := map[string]string{
			"air.current":         "/airquality/v1/current",
			"air.forecast.daily":  "/airquality/v1/daily",
			"air.forecast.hourly": "/airquality/v1/hourly",
		}[capability.ID]
		request.Path = latLonCoordinatePath(base, coordinate)
		setLanguage(query, parameters.Language)

	case "air.station.current":
		stationID := strings.TrimSpace(input.AirStationID)
		if !providerPathID.MatchString(stationID) {
			return qweather.Request{}, invalidRequest(capability.ID, "--air-station-id contains unsupported characters")
		}
		request.Path = "/airquality/v1/stations/" + stationID
		setLanguage(query, parameters.Language)

	case "storm.list":
		if parameters.Now.IsZero() {
			return qweather.Request{}, internalRequestProblem(capability.ID, "request clock is unavailable")
		}
		if !catalog.SupportsStormYear(parameters.Now, input.Year) {
			return qweather.Request{}, invalidRequest(capability.ID, "--year must be the current or previous UTC calendar year")
		}
		request.Path = "/v7/tropical/storm-list"
		query.Set("basin", "NP")
		query.Set("year", strconv.Itoa(input.Year))

	case "storm.track", "storm.forecast":
		stormID := strings.TrimSpace(input.StormID)
		if stormID == "" {
			return qweather.Request{}, invalidRequest(capability.ID, "--storm-id must not be empty")
		}
		if capability.ID == "storm.track" {
			request.Path = "/v7/tropical/storm-track"
		} else {
			request.Path = "/v7/tropical/storm-forecast"
		}
		query.Set("stormid", stormID)

	case "marine.tide":
		stationID := strings.TrimSpace(input.TideStationID)
		if stationID == "" {
			return qweather.Request{}, invalidRequest(capability.ID, "--tide-station-id must not be empty")
		}
		date, problem := providerDateInWindow(input.Date, capability.ID, parameters.Now, catalog.TideDateWindowDays)
		if problem != nil {
			return qweather.Request{}, problem
		}
		request.Path = "/v7/ocean/tide"
		query.Set("location", stationID)
		query.Set("date", date)

	case "solar.radiation.forecast":
		coordinate, problem := requireCoordinate(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		hours := input.Hours
		if hours == 0 && !parameters.Changed["hours"] {
			hours = 24
		}
		if hours < 1 || hours > 60 {
			return qweather.Request{}, invalidRequest(capability.ID, "--hours is outside the supported range")
		}
		interval := input.IntervalMinutes
		if interval == 0 && !parameters.Changed["interval-min"] {
			interval = 60
		}
		if !slices.Contains([]int{15, 30, 60}, interval) {
			return qweather.Request{}, invalidRequest(capability.ID, "--interval-min has an unsupported value")
		}
		extras := append([]string(nil), input.Includes...)
		for _, extra := range extras {
			if !slices.Contains([]string{"weather", "poa"}, extra) {
				return qweather.Request{}, invalidRequest(capability.ID, "--include contains an unsupported value")
			}
		}
		if slices.Contains(extras, "poa") && (!parameters.Changed["tilt-deg"] || !parameters.Changed["azimuth-deg"]) {
			return qweather.Request{}, invalidRequest(capability.ID, "--include poa requires --tilt-deg and --azimuth-deg")
		}
		if parameters.Changed["tilt-deg"] && (input.TiltDegrees < 0 || input.TiltDegrees > 90) {
			return qweather.Request{}, invalidRequest(capability.ID, "--tilt-deg is outside the supported range")
		}
		if parameters.Changed["azimuth-deg"] && (input.AzimuthDegrees < 0 || input.AzimuthDegrees > 359) {
			return qweather.Request{}, invalidRequest(capability.ID, "--azimuth-deg is outside the supported range")
		}
		request.Path = latLonCoordinatePath("/solarradiation/v1/forecast", coordinate)
		query.Set("hours", strconv.Itoa(hours))
		query.Set("interval", strconv.Itoa(interval))
		if len(extras) > 0 {
			sort.Strings(extras)
			query.Set("extra", strings.Join(extras, ","))
		}
		if parameters.Changed["tilt-deg"] {
			query.Set("tilt", strconv.Itoa(input.TiltDegrees))
		}
		if parameters.Changed["azimuth-deg"] {
			query.Set("azimuth", strconv.Itoa(input.AzimuthDegrees))
		}
		if input.LocalTime {
			query.Set("localTime", "true")
		}

	case "astronomy.sun.events", "astronomy.moon.events":
		location, problem := requirePlace(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		date, problem := providerDateInWindow(input.Date, capability.ID, parameters.Now, catalog.AstronomyDateWindowDays)
		if problem != nil {
			return qweather.Request{}, problem
		}
		if capability.ID == "astronomy.sun.events" {
			request.Path = "/v7/astronomy/sun"
		} else {
			request.Path = "/v7/astronomy/moon"
			setLanguage(query, parameters.Language)
		}
		query.Set("location", location)
		query.Set("date", date)

	case "astronomy.solar.position":
		coordinate, problem := requireCoordinate(parameters.Resolved, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		date, clock, timezone, problem := providerSolarTimestamp(input.At, capability.ID)
		if problem != nil {
			return qweather.Request{}, problem
		}
		if math.IsNaN(input.AltitudeMeters) || math.IsInf(input.AltitudeMeters, 0) || input.AltitudeMeters < -500 || input.AltitudeMeters > 9000 {
			return qweather.Request{}, invalidRequest(capability.ID, "--altitude-m is outside the supported range")
		}
		request.Path = "/v7/astronomy/solar-elevation-angle"
		query.Set("location", coordinate.ProviderQuery())
		query.Set("date", date)
		query.Set("time", clock)
		query.Set("tz", timezone)
		query.Set("alt", strconv.FormatFloat(input.AltitudeMeters, 'f', -1, 64))

	case "account.finance.summary":
		request.Path = "/finance/v1/summary"

	case "account.requests.stats":
		projectID := strings.TrimSpace(input.ProjectID)
		credentialID := strings.TrimSpace(input.CredentialID)
		if projectID != "" && credentialID != "" {
			return qweather.Request{}, invalidRequest(capability.ID, "--project-id and --credential-id are mutually exclusive")
		}
		request.Path = "/metrics/v1/stats"
		if projectID != "" {
			query.Set("project", projectID)
		}
		if credentialID != "" {
			query.Set("cred", credentialID)
		}

	default:
		problem := output.NewProblem(10, output.CodeCapabilityNotImplemented, "capability request mapping is not implemented")
		problem.Capability = capability.ID
		return qweather.Request{}, problem
	}

	request.Query = query
	return request, nil
}

func geoLookupLocation(input catalog.Input) (string, *output.Problem) {
	forms := 0
	var location string
	if value := strings.TrimSpace(input.Query); value != "" {
		forms++
		location = value
	}
	if value := strings.TrimSpace(input.PlaceID); value != "" {
		forms++
		location = value
	}
	if value := strings.TrimSpace(input.Coordinate); value != "" {
		forms++
		coordinate, err := place.ParseCoordinate(value)
		if err != nil {
			return "", invalidRequest("", err.Error())
		}
		location = coordinate.ProviderQuery()
	}
	if forms != 1 {
		return "", invalidRequest("", "exactly one of --query, --place-id, or --coordinate is required")
	}
	return location, nil
}

func requirePlace(resolved place.Resolved, capabilityID string) (string, *output.Problem) {
	if resolved.ID != "" {
		return resolved.ID, nil
	}
	coordinate, problem := requireCoordinate(resolved, capabilityID)
	if problem != nil {
		return "", invalidRequest(capabilityID, "a resolved place target is required")
	}
	return coordinate.ProviderQuery(), nil
}

func requireCoordinate(resolved place.Resolved, capabilityID string) (place.Coordinate, *output.Problem) {
	if resolved.Lat == "" || resolved.Lon == "" {
		return place.Coordinate{}, invalidRequest(capabilityID, "resolved coordinates are required")
	}
	coordinate, err := place.ParseCoordinate("geo:" + resolved.Lat + "," + resolved.Lon)
	if err != nil {
		return place.Coordinate{}, invalidRequest(capabilityID, "resolved coordinates are invalid")
	}
	return coordinate, nil
}

func latLonCoordinatePath(base string, coordinate place.Coordinate) string {
	return base + "/" + coordinate.LatText + "/" + coordinate.LonText
}

func providerCountry(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func providerPOIType(value, capabilityID string) (string, *output.Problem) {
	switch value {
	case "scenic":
		return "scenic", nil
	case "tide-station":
		return "TSTA", nil
	default:
		return "", invalidRequest(capabilityID, "--poi-type has an unsupported value")
	}
}

func providerUnit(value, capabilityID string) (string, *output.Problem) {
	switch strings.TrimSpace(value) {
	case "metric":
		return "m", nil
	case "imperial":
		return "i", nil
	default:
		return "", invalidRequest(capabilityID, "effective unit must be metric or imperial")
	}
}

func providerDate(value, capabilityID string) (string, *output.Problem) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return "", invalidRequest(capabilityID, "--date must use a real YYYY-MM-DD date")
	}
	return parsed.Format("20060102"), nil
}

func providerDateInWindow(value, capabilityID string, now time.Time, days int) (string, *output.Problem) {
	date, problem := providerDate(value, capabilityID)
	if problem != nil {
		return "", problem
	}
	if now.IsZero() {
		return "", internalRequestProblem(capabilityID, "request clock is unavailable")
	}
	parsed, _ := time.Parse("20060102", date)
	today, last, ok := catalog.UTCDateWindow(now, days)
	if !ok {
		return "", internalRequestProblem(capabilityID, "request date window is unavailable")
	}
	if parsed.Before(today) || parsed.After(last) {
		return "", invalidRequest(capabilityID, fmt.Sprintf("--date must be between %s and %s inclusive", today.Format("2006-01-02"), last.Format("2006-01-02")))
	}
	return date, nil
}

func providerSolarTimestamp(value, capabilityID string) (string, string, string, *output.Problem) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "", "", "", invalidRequest(capabilityID, "--at must use RFC3339 form")
	}
	timezone := strings.TrimPrefix(parsed.Format("-0700"), "+")
	return parsed.Format("20060102"), parsed.Format("1504"), timezone, nil
}

func providerIndices(input catalog.Input, capabilityID string) (string, *output.Problem) {
	if input.AllIndices == (len(input.Indices) > 0) {
		return "", invalidRequest(capabilityID, "exactly one of --index or --all-indices is required")
	}
	if input.AllIndices {
		return "0", nil
	}
	values := append([]int(nil), input.Indices...)
	sort.Ints(values)
	seen := make(map[int]struct{}, len(values))
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value < 1 || value > 16 {
			return "", invalidRequest(capabilityID, "--index must be between 1 and 16")
		}
		if _, exists := seen[value]; exists {
			return "", invalidRequest(capabilityID, "--index values must be unique")
		}
		seen[value] = struct{}{}
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ","), nil
}

func validateLimitedLanguage(language, capabilityID string) *output.Problem {
	language = strings.TrimSpace(language)
	if language == "auto" || language == "zh" || language == "en" {
		return nil
	}
	return invalidRequest(capabilityID, "this capability supports only zh or en")
}

func setLanguage(query url.Values, language string) {
	language = strings.TrimSpace(language)
	if language != "" && language != "auto" {
		query.Set("lang", language)
	}
}

func setLimit(query url.Values, limit int, capabilityID string) *output.Problem {
	if limit == 0 {
		return nil
	}
	if limit < 1 || limit > 20 {
		return invalidRequest(capabilityID, "--limit must be between 1 and 20")
	}
	query.Set("number", strconv.Itoa(limit))
	return nil
}

func invalidRequest(capabilityID, message string) *output.Problem {
	problem := output.NewProblem(2, output.CodeInvalidInvocation, message)
	problem.Capability = capabilityID
	return problem
}

func internalRequestProblem(capabilityID, message string) *output.Problem {
	problem := output.NewProblem(10, output.CodeInternalError, message)
	problem.Capability = capabilityID
	return problem
}

func problemForCapability(problem *output.Problem, capabilityID string) *output.Problem {
	if problem == nil {
		return nil
	}
	problem.Capability = capabilityID
	return problem
}
