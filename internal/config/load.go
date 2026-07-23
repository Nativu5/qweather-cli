package config

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Nativu5/qweather-cli/internal/auth"
	"github.com/pelletier/go-toml/v2"
)

const defaultProfile = "default"

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func Load(ctx context.Context, options Options) (Effective, Diagnostics, error) {
	options = withDefaults(options)
	if err := ctx.Err(); err != nil {
		return Effective{}, Diagnostics{}, err
	}
	diagnostics := Diagnostics{ValueSources: make(map[string]string)}
	configPath, configSource, err := selectConfigPath(options)
	if err != nil {
		return Effective{}, diagnostics, err
	}
	diagnostics.ConfigSource = configSource
	profileName, profileSource, err := selectProfile(options)
	if err != nil {
		return Effective{}, diagnostics, err
	}
	diagnostics.ProfileSource = profileSource
	if !profileNamePattern.MatchString(profileName) {
		return Effective{}, diagnostics, errors.New("profile name contains unsupported characters")
	}

	configuration, loaded, err := readConfiguration(options, configPath, configSource)
	if err != nil {
		return Effective{}, diagnostics, err
	}
	profile := profileFile{}
	if loaded {
		var exists bool
		profile, exists = configuration.Profiles[profileName]
		if !exists {
			return Effective{}, diagnostics, fmt.Errorf("profile %q does not exist", profileName)
		}
	}

	cacheDirectory, err := defaultCacheDirectory(options)
	if err != nil {
		return Effective{}, diagnostics, err
	}
	effective := Effective{
		ConfigPath:   configPath,
		ConfigLoaded: loaded,
		Profile:      profileName,
		APIHost:      strings.TrimSpace(profile.APIHost),
		Language:     valueOrDefault(strings.TrimSpace(profile.Language), "auto"),
		Unit:         valueOrDefault(strings.TrimSpace(profile.Unit), "metric"),
		JWTLifetime:  15 * time.Minute,
		Cache: CacheSettings{
			Enabled:   true,
			Sensitive: false,
			Directory: cacheDirectory,
		},
	}
	for _, key := range []string{"api_host", "language", "unit", "project_id", "credential_id", "private_key_file", "jwt_ttl"} {
		diagnostics.ValueSources[key] = "profile"
	}
	if configuration.Cache.Enabled != nil {
		effective.Cache.Enabled = *configuration.Cache.Enabled
	}
	if configuration.Cache.Sensitive != nil {
		effective.Cache.Sensitive = *configuration.Cache.Sensitive
	}
	if configuration.Cache.Stale != nil && *configuration.Cache.Stale {
		return Effective{}, diagnostics, errors.New("cache stale mode is not supported")
	}

	projectID := strings.TrimSpace(profile.ProjectID)
	credentialID := strings.TrimSpace(profile.CredentialID)
	privateKeyFile := strings.TrimSpace(profile.PrivateKeyFile)
	jwtTTLText := strings.TrimSpace(profile.JWTTTL)
	authMode := strings.TrimSpace(profile.Auth)
	apiKeyEnv := strings.TrimSpace(profile.APIKeyEnv)

	stringOverrides := []struct {
		name   string
		target *string
		key    string
	}{
		{"QWEATHER_API_HOST", &effective.APIHost, "api_host"},
		{"QWEATHER_PROJECT_ID", &projectID, "project_id"},
		{"QWEATHER_CREDENTIAL_ID", &credentialID, "credential_id"},
		{"QWEATHER_PRIVATE_KEY_FILE", &privateKeyFile, "private_key_file"},
		{"QWEATHER_JWT_TTL", &jwtTTLText, "jwt_ttl"},
		{"QWEATHER_LANGUAGE", &effective.Language, "language"},
		{"QWEATHER_UNIT", &effective.Unit, "unit"},
	}
	for _, override := range stringOverrides {
		if err := applyStringEnv(options.LookupEnv, override.name, override.target); err != nil {
			return Effective{}, diagnostics, err
		} else if _, present := options.LookupEnv(override.name); present {
			diagnostics.ValueSources[override.key] = "environment"
		}
	}
	if err := applyBoolEnv(options.LookupEnv, "QWEATHER_CACHE_ENABLED", &effective.Cache.Enabled); err != nil {
		return Effective{}, diagnostics, err
	}
	if err := applyBoolEnv(options.LookupEnv, "QWEATHER_CACHE_SENSITIVE", &effective.Cache.Sensitive); err != nil {
		return Effective{}, diagnostics, err
	}
	if options.LanguageOverride != nil {
		effective.Language = strings.TrimSpace(*options.LanguageOverride)
		diagnostics.ValueSources["language"] = "flag"
	}
	if options.UnitOverride != nil {
		effective.Unit = strings.TrimSpace(*options.UnitOverride)
		diagnostics.ValueSources["unit"] = "flag"
	}

	if err := validateAPIHost(effective.APIHost); err != nil {
		return Effective{}, diagnostics, err
	}
	if effective.Language == "" {
		return Effective{}, diagnostics, errors.New("language must not be empty")
	}
	if effective.Unit != "metric" && effective.Unit != "imperial" {
		return Effective{}, diagnostics, errors.New("unit must be metric or imperial")
	}
	if jwtTTLText != "" {
		effective.JWTLifetime, err = time.ParseDuration(jwtTTLText)
		if err != nil {
			return Effective{}, diagnostics, errors.New("JWT TTL is not a valid duration")
		}
	}
	if effective.JWTLifetime <= 0 || effective.JWTLifetime > auth.MaxJWTTTL {
		return Effective{}, diagnostics, fmt.Errorf("JWT TTL must be positive and no greater than %s", auth.MaxJWTTTL)
	}

	externalJWT, externalJWTPresent := options.LookupEnv("QWEATHER_JWT")
	if externalJWTPresent && strings.TrimSpace(externalJWT) == "" {
		return Effective{}, diagnostics, errors.New("QWEATHER_JWT is set but empty")
	}
	if apiKey, present := options.LookupEnv("QWEATHER_API_KEY"); present && strings.TrimSpace(apiKey) == "" {
		return Effective{}, diagnostics, errors.New("QWEATHER_API_KEY is set but empty")
	}
	if authMode == "" {
		if externalJWTPresent {
			authMode = "jwt"
		} else if _, present := options.LookupEnv("QWEATHER_API_KEY"); present {
			authMode = "api_key"
		}
	}
	switch authMode {
	case "jwt":
		if apiKeyEnv != "" {
			return Effective{}, diagnostics, errors.New("JWT profile must not configure api_key_env")
		}
		if externalJWTPresent {
			effective.Credentials, err = auth.NewExternalJWT(externalJWT)
			effective.AuthMethod = auth.MethodExternalJWT
			diagnostics.AuthSource = "QWEATHER_JWT"
			diagnostics.SecretPresent = true
		} else {
			keyPath, expandErr := expandHome(privateKeyFile)
			if expandErr != nil {
				return Effective{}, diagnostics, expandErr
			}
			key, keyErr := readPrivateKey(options, keyPath)
			if keyErr != nil {
				return Effective{}, diagnostics, keyErr
			}
			effective.Credentials, err = auth.NewGeneratedJWT(projectID, credentialID, key, effective.JWTLifetime)
			effective.AuthMethod = auth.MethodGeneratedJWT
			diagnostics.AuthSource = "private_key_file"
			diagnostics.SecretPresent = true
		}
	case "api_key":
		if externalJWTPresent {
			return Effective{}, diagnostics, errors.New("QWEATHER_JWT conflicts with api_key authentication")
		}
		if projectID != "" || credentialID != "" || privateKeyFile != "" {
			return Effective{}, diagnostics, errors.New("api_key profile must not configure JWT identity or private key fields")
		}
		if apiKeyEnv == "" {
			apiKeyEnv = "QWEATHER_API_KEY"
		}
		if !environmentNamePattern.MatchString(apiKeyEnv) {
			return Effective{}, diagnostics, errors.New("api_key_env contains unsupported characters")
		}
		apiKey, present := options.LookupEnv(apiKeyEnv)
		if !present {
			return Effective{}, diagnostics, fmt.Errorf("referenced API key environment variable %s is not set", apiKeyEnv)
		}
		if strings.TrimSpace(apiKey) == "" {
			return Effective{}, diagnostics, fmt.Errorf("referenced API key environment variable %s is empty", apiKeyEnv)
		}
		effective.Credentials, err = auth.NewAPIKey(apiKey)
		effective.AuthMethod = auth.MethodAPIKey
		diagnostics.AuthSource = apiKeyEnv
		diagnostics.SecretPresent = true
	default:
		return Effective{}, diagnostics, errors.New("auth must be jwt or api_key")
	}
	if err != nil {
		return Effective{}, diagnostics, fmt.Errorf("validate authentication configuration: %w", err)
	}
	if err := validateCacheDirectory(options, effective.Cache.Directory); err != nil {
		return Effective{}, diagnostics, err
	}
	return effective, diagnostics, nil
}

func withDefaults(options Options) Options {
	if options.LookupEnv == nil {
		options.LookupEnv = os.LookupEnv
	}
	if options.UserConfigDir == nil {
		options.UserConfigDir = os.UserConfigDir
	}
	if options.UserCacheDir == nil {
		options.UserCacheDir = os.UserCacheDir
	}
	if options.ReadFile == nil {
		options.ReadFile = os.ReadFile
	}
	if options.Stat == nil {
		options.Stat = os.Stat
	}
	return options
}

func selectConfigPath(options Options) (string, string, error) {
	if options.ConfigPath != "" {
		path, err := expandHome(options.ConfigPath)
		return path, "flag", err
	}
	if value, present := options.LookupEnv("QWEATHER_CONFIG"); present {
		if strings.TrimSpace(value) == "" {
			return "", "", errors.New("QWEATHER_CONFIG is set but empty")
		}
		path, err := expandHome(value)
		return path, "environment", err
	}
	directory, err := options.UserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("locate user configuration directory: %w", err)
	}
	return filepath.Join(directory, "qweather", "config.toml"), "default", nil
}

func selectProfile(options Options) (string, string, error) {
	if options.Profile != "" {
		return options.Profile, "flag", nil
	}
	if value, present := options.LookupEnv("QWEATHER_PROFILE"); present {
		if strings.TrimSpace(value) == "" {
			return "", "", errors.New("QWEATHER_PROFILE is set but empty")
		}
		return value, "environment", nil
	}
	return defaultProfile, "default", nil
}

func readConfiguration(options Options, path, source string) (fileConfig, bool, error) {
	contents, err := options.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && source == "default" {
			return fileConfig{}, false, nil
		}
		return fileConfig{}, false, fmt.Errorf("read configuration file %s: %w", path, err)
	}
	configuration := fileConfig{}
	decoder := toml.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configuration); err != nil {
		return fileConfig{}, false, fmt.Errorf("decode configuration file %s: invalid TOML or unknown field", path)
	}
	if configuration.Profiles == nil {
		configuration.Profiles = map[string]profileFile{}
	}
	return configuration, true, nil
}

func defaultCacheDirectory(options Options) (string, error) {
	directory, err := options.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache directory: %w", err)
	}
	return filepath.Join(directory, "qweather"), nil
}

func applyStringEnv(lookup func(string) (string, bool), name string, target *string) error {
	value, present := lookup(name)
	if !present {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is set but empty", name)
	}
	*target = strings.TrimSpace(value)
	return nil
}

func applyBoolEnv(lookup func(string) (string, bool), name string, target *bool) error {
	value, present := lookup(name)
	if !present {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is set but empty", name)
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("%s must be true or false", name)
	}
	*target = parsed
	return nil
}

func validateAPIHost(value string) error {
	if value == "" || strings.Contains(value, "://") || strings.ContainsAny(value, "/?#@ \t\r\n") {
		return errors.New("api_host must be an account-specific hostname without a scheme or path")
	}
	parsed, err := url.Parse("https://" + value)
	if err != nil || parsed.Hostname() == "" || parsed.Port() != "" {
		return errors.New("api_host must be an account-specific hostname without a port")
	}
	return nil
}

func readPrivateKey(options Options, path string) (ed25519.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("private_key_file is required for generated JWT authentication")
	}
	info, err := options.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect private key file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("private key path is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("private key file permissions must not allow group or other access")
	}
	contents, err := options.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}
	return auth.ParsePrivateKeyPEM(contents)
}

func validateCacheDirectory(options Options, path string) error {
	info, err := options.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect cache directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("cache path exists but is not a directory")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return errors.New("cache directory permissions must not allow group or other access")
	}
	return nil
}

func expandHome(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "~" {
		if path == "" {
			return "", nil
		}
		home, err := os.UserHomeDir()
		return home, err
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
