package config

import (
	"os"
	"time"

	"github.com/Nativu5/qweather-cli/internal/auth"
)

type Options struct {
	ConfigPath       string
	Profile          string
	LanguageOverride *string
	UnitOverride     *string
	LookupEnv        func(string) (string, bool)
	UserConfigDir    func() (string, error)
	UserCacheDir     func() (string, error)
	ReadFile         func(string) ([]byte, error)
	Stat             func(string) (os.FileInfo, error)
}

type CacheSettings struct {
	Enabled   bool   `json:"enabled"`
	Sensitive bool   `json:"sensitive"`
	Directory string `json:"directory"`
}

type Effective struct {
	ConfigPath   string           `json:"configPath"`
	ConfigLoaded bool             `json:"configLoaded"`
	Profile      string           `json:"profile"`
	APIHost      string           `json:"apiHost"`
	Language     string           `json:"language"`
	Unit         string           `json:"unit"`
	JWTLifetime  time.Duration    `json:"jwtLifetime,omitempty"`
	Cache        CacheSettings    `json:"cache"`
	AuthMethod   auth.Method      `json:"authMethod"`
	Credentials  auth.Credentials `json:"-"`
}

type Diagnostics struct {
	ConfigSource  string            `json:"configSource"`
	ProfileSource string            `json:"profileSource"`
	AuthSource    string            `json:"authSource"`
	SecretPresent bool              `json:"secretPresent"`
	ValueSources  map[string]string `json:"valueSources"`
}

type CheckResult struct {
	Valid       bool        `json:"valid"`
	Effective   Effective   `json:"effective"`
	Diagnostics Diagnostics `json:"diagnostics"`
}

type fileConfig struct {
	Profiles map[string]profileFile `toml:"profiles"`
	Cache    cacheFile              `toml:"cache"`
}

type profileFile struct {
	APIHost        string `toml:"api_host"`
	Auth           string `toml:"auth"`
	ProjectID      string `toml:"project_id"`
	CredentialID   string `toml:"credential_id"`
	PrivateKeyFile string `toml:"private_key_file"`
	JWTTTL         string `toml:"jwt_ttl"`
	APIKey         string `toml:"api_key"`
	APIKeyEnv      string `toml:"api_key_env"`
	Language       string `toml:"language"`
	Unit           string `toml:"unit"`
}

type cacheFile struct {
	Enabled   *bool `toml:"enabled"`
	Sensitive *bool `toml:"sensitive"`
	Stale     *bool `toml:"stale"`
}
