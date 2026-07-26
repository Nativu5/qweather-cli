package buildinfo

import (
	"runtime"
	"time"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version      string `json:"version"`
	GoVersion    string `json:"goVersion"`
	Commit       string `json:"commit"`
	BuildTime    string `json:"buildTime"`
	RegistryHash string `json:"registryHash"`
}

func Current(registryHash string) Info {
	return Info{
		Version:      Version,
		GoVersion:    runtime.Version(),
		Commit:       Commit,
		BuildTime:    normalizedBuildTime(BuildTime),
		RegistryHash: registryHash,
	}
}

func normalizedBuildTime(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format(time.RFC3339)
}
