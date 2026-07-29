package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Nativu5/qweather-cli/internal/catalog"
)

func networkBranchSummary(path string) (string, error) {
	switch path {
	case "account":
		return "Account finance and request-usage data", nil
	case "air":
		return "Current and forecast air quality", nil
	case "alert":
		return "Current weather alerts", nil
	case "astronomy":
		return "Sun, moon, and solar-position data", nil
	case "geo":
		return "Place and point-of-interest lookup", nil
	case "geo city":
		return "City and place lookup", nil
	case "geo poi":
		return "Point-of-interest lookup", nil
	case "marine":
		return "Marine forecasts", nil
	case "solar":
		return "Solar-radiation forecasts", nil
	case "storm":
		return "Tropical-storm lists, tracks, and forecasts", nil
	case "weather":
		return "Weather observations, forecasts, precipitation, indices, and history", nil
	case "weather city":
		return "Weather for cities and named places", nil
	case "weather grid":
		return "Coordinate-based gridded weather", nil
	default:
		return "", fmt.Errorf("missing help summary for command branch %q", path)
	}
}

func capabilityLongHelp(capability catalog.Capability) string {
	sections := []string{capability.Summary}
	if constraints := capabilityHelpConstraints(capability); len(constraints) > 0 {
		sections = append(sections, "Constraints:\n  "+strings.Join(constraints, "\n  "))
	}
	if safety := capabilitySafetyHelp(capability); safety != "" {
		sections = append(sections, "Safety:\n  "+safety)
	}
	return strings.Join(sections, "\n\n")
}

func capabilityHelpConstraints(capability catalog.Capability) []string {
	constraints := make([]string, 0, 4)
	if exposesPlaceSpec(capability) {
		constraints = append(constraints, "Exactly one of --place, --place-id, or --coordinate is required.")
		if hasCapabilityFlag(capability, "country") || hasCapabilityFlag(capability, "adm") {
			constraints = append(constraints, "--country and --adm are valid only with --place.")
		}
		switch capability.Target {
		case catalog.TargetCoordinate:
			constraints = append(constraints, "The selected target must resolve to coordinates.")
		case catalog.TargetLocationID:
			constraints = append(constraints, "The selected target must resolve to a QWeather Location ID.")
		}
	}
	if capability.Target == catalog.TargetGeoLookup {
		constraints = append(constraints, "Exactly one of --query, --place-id, or --coordinate is required.")
		if hasCapabilityFlag(capability, "country") || hasCapabilityFlag(capability, "adm") {
			constraints = append(constraints, "--country and --adm are valid only with --query.")
		}
	}

	switch capability.ID {
	case "weather.indices.forecast":
		constraints = append(constraints, "Exactly one of --index or --all-indices is required; --index values must be between 1 and 16 and unique.")
	case "solar.radiation.forecast":
		constraints = append(constraints, "--include poa requires --tilt-deg and --azimuth-deg.")
	case "account.requests.stats":
		constraints = append(constraints, "--project-id and --credential-id are mutually exclusive.")
	}
	return constraints
}

func capabilitySafetyHelp(capability catalog.Capability) string {
	switch capability.ProductGate {
	case catalog.GateMarine:
		return "This capability uses the Marine Billing Group; pass --yes to confirm this invocation before network I/O."
	case catalog.GateSolar:
		return "This capability uses the Solar Billing Group; pass --yes to confirm this invocation before network I/O."
	case catalog.GateSensitiveAccount:
		return "This capability returns Sensitive Account Data; pass --yes to confirm this invocation before network I/O."
	default:
		return ""
	}
}

func capabilityExample(capability catalog.Capability) string {
	switch capability.ID {
	case "weather.city.forecast.daily":
		return "  qweather weather city daily --place Beijing --days 7 --output text"
	case "weather.indices.forecast":
		return "  qweather weather indices --place Beijing --days 1 --all-indices --output text"
	case "marine.tide":
		return "  qweather marine tide --tide-station-id P66981 --date \"$(date -u +%F)\" --yes --output text"
	case "solar.radiation.forecast":
		return "  qweather solar forecast --coordinate geo:39.9042,116.4074 --yes --output text"
	default:
		return ""
	}
}

func capabilityFlagUsage(flag catalog.Flag) string {
	details := make([]string, 0, 3)
	if flag.Required {
		details = append(details, "required")
	}
	if len(flag.Enum) > 0 {
		details = append(details, "one of "+strings.Join(flag.Enum, ", "))
	}
	if len(flag.IntEnum) > 0 {
		values := make([]string, len(flag.IntEnum))
		for index, value := range flag.IntEnum {
			values[index] = strconv.Itoa(value)
		}
		details = append(details, "one of "+strings.Join(values, ", "))
	}
	if flag.Min != nil || flag.Max != nil {
		details = append(details, "range "+flagRange(flag.Min, flag.Max))
	}
	if len(details) == 0 {
		return flag.Usage
	}
	return flag.Usage + " (" + strings.Join(details, "; ") + ")"
}

func flagRange(minimum, maximum *float64) string {
	lower, upper := "unbounded", "unbounded"
	if minimum != nil {
		lower = strconv.FormatFloat(*minimum, 'f', -1, 64)
	}
	if maximum != nil {
		upper = strconv.FormatFloat(*maximum, 'f', -1, 64)
	}
	return lower + ".." + upper
}

func exposesPlaceSpec(capability catalog.Capability) bool {
	return hasCapabilityFlag(capability, "place") &&
		hasCapabilityFlag(capability, "place-id") &&
		hasCapabilityFlag(capability, "coordinate")
}

func hasCapabilityFlag(capability catalog.Capability, name string) bool {
	for _, flag := range capability.Flags {
		if flag.Name == name {
			return true
		}
	}
	return false
}
