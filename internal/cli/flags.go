package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

func bindCapabilityFlags(command *cobra.Command, input *catalog.Input, flags []catalog.Flag) error {
	for _, flag := range flags {
		switch flag.Name {
		case "place":
			command.Flags().StringVar(&input.Place, flag.Name, "", flag.Usage)
		case "place-id":
			command.Flags().StringVar(&input.PlaceID, flag.Name, "", flag.Usage)
		case "coordinate":
			command.Flags().StringVar(&input.Coordinate, flag.Name, "", flag.Usage)
		case "country":
			command.Flags().StringVar(&input.Country, flag.Name, "", flag.Usage)
		case "adm":
			command.Flags().StringVar(&input.Adm, flag.Name, "", flag.Usage)
		case "query":
			command.Flags().StringVar(&input.Query, flag.Name, "", flag.Usage)
		case "poi-type":
			command.Flags().StringVar(&input.POIType, flag.Name, "", flag.Usage)
		case "limit":
			command.Flags().IntVar(&input.Limit, flag.Name, 0, flag.Usage)
		case "lang":
			command.Flags().StringVar(&input.Language, flag.Name, "", flag.Usage)
		case "unit":
			command.Flags().StringVar(&input.Unit, flag.Name, "", flag.Usage)
		case "days":
			command.Flags().IntVar(&input.Days, flag.Name, 0, flag.Usage)
		case "hours":
			command.Flags().IntVar(&input.Hours, flag.Name, 0, flag.Usage)
		case "index":
			command.Flags().IntSliceVar(&input.Indices, flag.Name, nil, flag.Usage)
		case "all-indices":
			command.Flags().BoolVar(&input.AllIndices, flag.Name, false, flag.Usage)
		case "date":
			command.Flags().StringVar(&input.Date, flag.Name, "", flag.Usage)
		case "local-time":
			command.Flags().BoolVar(&input.LocalTime, flag.Name, false, flag.Usage)
		case "air-station-id":
			command.Flags().StringVar(&input.AirStationID, flag.Name, "", flag.Usage)
		case "storm-id":
			command.Flags().StringVar(&input.StormID, flag.Name, "", flag.Usage)
		case "year":
			command.Flags().IntVar(&input.Year, flag.Name, 0, flag.Usage)
		case "tide-station-id":
			command.Flags().StringVar(&input.TideStationID, flag.Name, "", flag.Usage)
		case "interval-min":
			command.Flags().IntVar(&input.IntervalMinutes, flag.Name, 0, flag.Usage)
		case "include":
			command.Flags().StringSliceVar(&input.Includes, flag.Name, nil, flag.Usage)
		case "tilt-deg":
			command.Flags().Float64Var(&input.TiltDegrees, flag.Name, 0, flag.Usage)
		case "azimuth-deg":
			command.Flags().Float64Var(&input.AzimuthDegrees, flag.Name, 0, flag.Usage)
		case "at":
			command.Flags().StringVar(&input.At, flag.Name, "", flag.Usage)
		case "altitude-m":
			command.Flags().Float64Var(&input.AltitudeMeters, flag.Name, 0, flag.Usage)
		case "project-id":
			command.Flags().StringVar(&input.ProjectID, flag.Name, "", flag.Usage)
		case "credential-id":
			command.Flags().StringVar(&input.CredentialID, flag.Name, "", flag.Usage)
		case "allow-product":
			command.Flags().StringVar(&input.AllowProduct, flag.Name, "", flag.Usage)
		case "allow-sensitive-output":
			command.Flags().StringVar(&input.AllowSensitive, flag.Name, "", flag.Usage)
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
	if common.Output != "json" && common.Output != "body" {
		return invalid(capability.ID, "--output must be json or body")
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
	problem := output.NewProblem(2, "INVALID_INVOCATION", message)
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
	default:
		return 0
	}
}

func floatValue(name string, input catalog.Input) float64 {
	switch name {
	case "tilt-deg":
		return input.TiltDegrees
	case "azimuth-deg":
		return input.AzimuthDegrees
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
