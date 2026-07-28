package cli

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/Nativu5/qweather-cli/internal/place"
	"github.com/spf13/cobra"
)

var providerPathID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func bindCapabilityFlags(command *cobra.Command, input *catalog.Input, flags []catalog.Flag) error {
	for _, flag := range flags {
		integerDefault := 0
		if flag.Kind == catalog.FlagInt && flag.Default != "" {
			value, err := strconv.Atoi(flag.Default)
			if err != nil {
				return fmt.Errorf("registry flag --%s has an invalid integer default", flag.Name)
			}
			integerDefault = value
		}
		usage := capabilityFlagUsage(flag)
		switch flag.Name {
		case "place":
			command.Flags().StringVar(&input.Place, flag.Name, "", usage)
		case "place-id":
			command.Flags().StringVar(&input.PlaceID, flag.Name, "", usage)
		case "coordinate":
			command.Flags().StringVar(&input.Coordinate, flag.Name, "", usage)
		case "country":
			command.Flags().StringVar(&input.Country, flag.Name, "", usage)
		case "adm":
			command.Flags().StringVar(&input.Adm, flag.Name, "", usage)
		case "query":
			command.Flags().StringVar(&input.Query, flag.Name, "", usage)
		case "poi-type":
			command.Flags().StringVar(&input.POIType, flag.Name, "", usage)
		case "limit":
			command.Flags().IntVar(&input.Limit, flag.Name, integerDefault, usage)
		case "lang":
			command.Flags().StringVar(&input.Language, flag.Name, "", usage)
		case "unit":
			command.Flags().StringVar(&input.Unit, flag.Name, "", usage)
		case "days":
			command.Flags().IntVar(&input.Days, flag.Name, integerDefault, usage)
		case "hours":
			command.Flags().IntVar(&input.Hours, flag.Name, integerDefault, usage)
		case "index":
			command.Flags().IntSliceVar(&input.Indices, flag.Name, nil, usage)
		case "all-indices":
			command.Flags().BoolVar(&input.AllIndices, flag.Name, false, usage)
		case "date":
			command.Flags().StringVar(&input.Date, flag.Name, "", usage)
		case "local-time":
			command.Flags().BoolVar(&input.LocalTime, flag.Name, false, usage)
		case "air-station-id":
			command.Flags().StringVar(&input.AirStationID, flag.Name, "", usage)
		case "storm-id":
			command.Flags().StringVar(&input.StormID, flag.Name, "", usage)
		case "year":
			command.Flags().IntVar(&input.Year, flag.Name, integerDefault, usage)
		case "tide-station-id":
			command.Flags().StringVar(&input.TideStationID, flag.Name, "", usage)
		case "interval-min":
			command.Flags().IntVar(&input.IntervalMinutes, flag.Name, integerDefault, usage)
		case "include":
			command.Flags().StringSliceVar(&input.Includes, flag.Name, nil, usage)
		case "tilt-deg":
			command.Flags().IntVar(&input.TiltDegrees, flag.Name, integerDefault, usage)
		case "azimuth-deg":
			command.Flags().IntVar(&input.AzimuthDegrees, flag.Name, integerDefault, usage)
		case "at":
			command.Flags().StringVar(&input.At, flag.Name, "", usage)
		case "altitude-m":
			command.Flags().Float64Var(&input.AltitudeMeters, flag.Name, 0, usage)
		case "project-id":
			command.Flags().StringVar(&input.ProjectID, flag.Name, "", usage)
		case "credential-id":
			command.Flags().StringVar(&input.CredentialID, flag.Name, "", usage)
		case "allow-product":
			command.Flags().StringVar(&input.AllowProduct, flag.Name, "", usage)
		case "allow-sensitive-output":
			command.Flags().StringVar(&input.AllowSensitive, flag.Name, "", usage)
		default:
			return fmt.Errorf("unsupported registry flag %q", flag.Name)
		}
		if flag.Required {
			if err := command.MarkFlagRequired(flag.Name); err != nil {
				return fmt.Errorf("mark --%s required: %w", flag.Name, err)
			}
		}
	}
	return nil
}

func validateInvocation(capability catalog.Capability, input catalog.Input, common CommonOptions, changed map[string]bool) *output.Problem {
	if common.Timeout <= 0 {
		return invalid(capability.ID, "--timeout must be positive")
	}
	if !output.Mode(common.Output).Valid() {
		return invalid(capability.ID, "--output must be text, json, or body")
	}
	if common.Refresh && common.NoCache {
		return invalid(capability.ID, "--refresh and --no-cache are mutually exclusive")
	}
	for _, flag := range capability.Flags {
		if problem := validateFlag(capability.ID, flag, input, changed[flag.Name]); problem != nil {
			return problem
		}
	}
	if capability.Target == catalog.TargetPlace || capability.Target == catalog.TargetCoordinate || capability.Target == catalog.TargetLocationID {
		count := nonEmpty(input.Place, input.PlaceID, input.Coordinate)
		if count != 1 {
			return invalid(capability.ID, "exactly one of --place, --place-id, or --coordinate is required")
		}
		if (input.Country != "" || input.Adm != "") && input.Place == "" {
			return invalid(capability.ID, "--country and --adm are valid only with --place")
		}
	}
	if capability.ID == "geo.city.lookup" && nonEmpty(input.Query, input.PlaceID, input.Coordinate) != 1 {
		return invalid(capability.ID, "exactly one of --query, --place-id, or --coordinate is required")
	}
	if input.Coordinate != "" {
		if _, err := place.ParseCoordinate(input.Coordinate); err != nil {
			return invalid(capability.ID, err.Error())
		}
	}
	if capability.ID == "geo.city.lookup" && (input.Country != "" || input.Adm != "") && strings.TrimSpace(input.Query) == "" {
		return invalid(capability.ID, "--country and --adm require --query")
	}
	if input.Date != "" {
		parsed, err := time.Parse("2006-01-02", input.Date)
		if err != nil || parsed.Format("2006-01-02") != input.Date {
			return invalid(capability.ID, "--date must use a real YYYY-MM-DD date")
		}
		var days int
		switch capability.ID {
		case "marine.tide":
			days = catalog.TideDateWindowDays
		case "astronomy.sun.events", "astronomy.moon.events":
			days = catalog.AstronomyDateWindowDays
		}
		if days > 0 {
			first, last, _ := catalog.UTCDateWindow(time.Now(), days)
			if parsed.Before(first) || parsed.After(last) {
				return invalid(capability.ID, fmt.Sprintf("--date must be between %s and %s inclusive", first.Format("2006-01-02"), last.Format("2006-01-02")))
			}
		}
	}
	if capability.ID == "storm.list" && !catalog.SupportsStormYear(time.Now(), input.Year) {
		return invalid(capability.ID, "--year must be the current or previous UTC calendar year")
	}
	if capability.Target == catalog.TargetAirStation && !providerPathID.MatchString(input.AirStationID) {
		return invalid(capability.ID, "--air-station-id contains unsupported characters")
	}
	if (capability.ID == "weather.indices.forecast" || capability.ID == "weather.precipitation.minutely") && changed["lang"] {
		if input.Language != "auto" && input.Language != "zh" && input.Language != "en" {
			return invalid(capability.ID, "this capability supports only zh or en")
		}
	}
	if capability.ID == "weather.indices.forecast" {
		if input.AllIndices == (len(input.Indices) > 0) {
			return invalid(capability.ID, "exactly one of --index or --all-indices is required")
		}
		seen := make(map[int]struct{}, len(input.Indices))
		for _, index := range input.Indices {
			if index < 1 || index > 16 {
				return invalid(capability.ID, "--index must be between 1 and 16")
			}
			if _, exists := seen[index]; exists {
				return invalid(capability.ID, "--index values must be unique")
			}
			seen[index] = struct{}{}
		}
	}
	if capability.ID == "account.requests.stats" && input.ProjectID != "" && input.CredentialID != "" {
		return invalid(capability.ID, "--project-id and --credential-id are mutually exclusive")
	}
	if capability.ID == "astronomy.solar.position" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(input.At)); err != nil {
			return invalid(capability.ID, "--at must use RFC3339 form")
		}
	}
	if capability.ID == "solar.radiation.forecast" && slices.Contains(input.Includes, "poa") {
		if !changed["tilt-deg"] || !changed["azimuth-deg"] {
			return invalid(capability.ID, "--include poa requires --tilt-deg and --azimuth-deg")
		}
	}
	return nil
}

func validateFlag(capabilityID string, flag catalog.Flag, input catalog.Input, changed bool) *output.Problem {
	if !changed && !flag.Required {
		return nil
	}
	switch flag.Kind {
	case catalog.FlagString:
		value := stringValue(flag.Name, input)
		if flag.Required && strings.TrimSpace(value) == "" {
			return invalid(capabilityID, "--"+flag.Name+" must not be empty")
		}
		if value != "" && len(flag.Enum) > 0 && !slices.Contains(flag.Enum, value) {
			return invalid(capabilityID, fmt.Sprintf("--%s must be one of %s", flag.Name, strings.Join(flag.Enum, ", ")))
		}
	case catalog.FlagInt:
		value := intValue(flag.Name, input)
		if len(flag.IntEnum) > 0 && !slices.Contains(flag.IntEnum, value) {
			return invalid(capabilityID, fmt.Sprintf("--%s has an unsupported value", flag.Name))
		}
		if flag.Min != nil && float64(value) < *flag.Min || flag.Max != nil && float64(value) > *flag.Max {
			return invalid(capabilityID, fmt.Sprintf("--%s is outside the supported range", flag.Name))
		}
	case catalog.FlagFloat:
		value := floatValue(flag.Name, input)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return invalid(capabilityID, fmt.Sprintf("--%s must be a finite number", flag.Name))
		}
		if flag.Min != nil && value < *flag.Min || flag.Max != nil && value > *flag.Max {
			return invalid(capabilityID, fmt.Sprintf("--%s is outside the supported range", flag.Name))
		}
	case catalog.FlagStringSlice:
		for _, value := range stringSliceValue(flag.Name, input) {
			if len(flag.Enum) > 0 && !slices.Contains(flag.Enum, value) {
				return invalid(capabilityID, fmt.Sprintf("--%s contains unsupported value %q", flag.Name, value))
			}
		}
	}
	return nil
}

func invalid(capabilityID, message string) *output.Problem {
	problem := output.NewProblem(2, output.CodeInvalidInvocation, message)
	problem.Capability = capabilityID
	return problem
}

func nonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func stringValue(name string, input catalog.Input) string {
	switch name {
	case "place":
		return input.Place
	case "place-id":
		return input.PlaceID
	case "coordinate":
		return input.Coordinate
	case "country":
		return input.Country
	case "adm":
		return input.Adm
	case "query":
		return input.Query
	case "poi-type":
		return input.POIType
	case "lang":
		return input.Language
	case "unit":
		return input.Unit
	case "date":
		return input.Date
	case "air-station-id":
		return input.AirStationID
	case "storm-id":
		return input.StormID
	case "tide-station-id":
		return input.TideStationID
	case "at":
		return input.At
	case "project-id":
		return input.ProjectID
	case "credential-id":
		return input.CredentialID
	case "allow-product":
		return input.AllowProduct
	case "allow-sensitive-output":
		return input.AllowSensitive
	default:
		return ""
	}
}

func intValue(name string, input catalog.Input) int {
	switch name {
	case "limit":
		return input.Limit
	case "days":
		return input.Days
	case "hours":
		return input.Hours
	case "year":
		return input.Year
	case "interval-min":
		return input.IntervalMinutes
	case "tilt-deg":
		return input.TiltDegrees
	case "azimuth-deg":
		return input.AzimuthDegrees
	default:
		return 0
	}
}

func floatValue(name string, input catalog.Input) float64 {
	switch name {
	case "altitude-m":
		return input.AltitudeMeters
	default:
		return 0
	}
}

func stringSliceValue(name string, input catalog.Input) []string {
	if name == "include" {
		return input.Includes
	}
	return nil
}
