package buildinfo

import "runtime"

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
		BuildTime:    BuildTime,
		RegistryHash: registryHash,
	}
}
