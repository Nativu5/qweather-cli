package catalog

func stringFlag(name, usage string, required bool, enum ...string) Flag {
	return Flag{Name: name, Usage: usage, Kind: FlagString, Required: required, Enum: enum}
}

func boolFlag(name, usage string) Flag {
	return Flag{Name: name, Usage: usage, Kind: FlagBool}
}

func intFlag(name, usage string, required bool, values ...int) Flag {
	return Flag{Name: name, Usage: usage, Kind: FlagInt, Required: required, IntEnum: values}
}

func rangedIntFlag(name, usage string, required bool, min, max float64) Flag {
	return Flag{Name: name, Usage: usage, Kind: FlagInt, Required: required, Min: &min, Max: &max}
}

func rangedIntSliceFlag(name, usage string, min, max float64) Flag {
	return Flag{Name: name, Usage: usage, Kind: FlagIntSlice, Min: &min, Max: &max}
}

func stringSliceFlag(name, usage string, values ...string) Flag {
	return Flag{Name: name, Usage: usage, Kind: FlagStringSlice, Enum: values}
}

func floatFlag(name, usage string, required bool, min, max float64) Flag {
	return Flag{Name: name, Usage: usage, Kind: FlagFloat, Required: required, Min: &min, Max: &max}
}

func placeFlags(language, unit bool) []Flag {
	flags := []Flag{
		stringFlag("place", "human place name to resolve", false),
		stringFlag("place-id", "QWeather Location ID", false),
		stringFlag("coordinate", "coordinate in geo:<lat>,<lon> form", false),
		stringFlag("country", "ISO-3166 country or region filter for --place", false),
		stringFlag("adm", "administrative-area filter for --place", false),
	}
	if language {
		flags = append(flags, stringFlag("lang", "response language", false))
	}
	if unit {
		flags = append(flags, stringFlag("unit", "measurement unit", false, "metric", "imperial"))
	}
	return flags
}

func appendFlags(base []Flag, extra ...Flag) []Flag {
	result := make([]Flag, 0, len(base)+len(extra))
	result = append(result, base...)
	return append(result, extra...)
}
