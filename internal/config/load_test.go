package config

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/auth"
)

func lookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func testOptions(t *testing.T, configPath string, environment map[string]string) Options {
	t.Helper()
	root := t.TempDir()
	return Options{
		ConfigPath: configPath,
		LookupEnv:  lookup(environment),
		UserConfigDir: func() (string, error) {
			return filepath.Join(root, "config"), nil
		},
		UserCacheDir: func() (string, error) {
			return filepath.Join(root, "cache"), nil
		},
	}
}

func writeConfig(t *testing.T, contents string) string {
	return writeConfigMode(t, contents, 0o600)
}

func writeConfigMode(t *testing.T, contents string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePrivateKey(t *testing.T, mode os.FileMode) string {
	t.Helper()
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "private.pem")
	contents := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadReportsAbsentConfigurationSources(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "qweather", "config.toml")
	options := Options{
		LookupEnv: lookup(nil),
		UserConfigDir: func() (string, error) {
			return filepath.Join(root, "config"), nil
		},
		UserCacheDir: func() (string, error) {
			return filepath.Join(root, "cache"), nil
		},
	}

	_, _, err := Load(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), "QWeather is not configured") {
		t.Fatalf("Load() error = %v", err)
	}
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Load() error kind = %v", err)
	}
	if _, statErr := os.Stat(configPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("default configuration file was created: %v", statErr)
	}
}

func TestLoadWithoutFilePreservesEnvironmentConfigurationBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		environment map[string]string
		wantError   string
	}{
		{
			name: "complete API key configuration",
			environment: map[string]string{
				"QWEATHER_API_HOST": "example.qweatherapi.com",
				"QWEATHER_API_KEY":  "environment-key",
			},
		},
		{
			name:        "partial provider configuration",
			environment: map[string]string{"QWEATHER_API_KEY": "environment-key"},
			wantError:   "api_host must be an account-specific hostname",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			effective, diagnostics, err := Load(context.Background(), testOptions(t, "", test.environment))
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("Load() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if effective.ConfigLoaded || effective.AuthMethod != auth.MethodAPIKey {
				t.Fatalf("effective = %#v", effective)
			}
			if diagnostics.ConfigSource != "default" || diagnostics.AuthSource != "QWEATHER_API_KEY" || !diagnostics.SecretPresent {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
		})
	}
}

func TestLoadReportsExplicitlySelectedMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	_, _, err := Load(context.Background(), testOptions(t, path, nil))
	if err == nil || !strings.Contains(err.Error(), "read configuration file "+path) {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsUnknownTOMLFields(t *testing.T) {
	path := writeConfig(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
api_key_env = "TEST_API_KEY"
surprise = true
`)
	_, _, err := Load(context.Background(), testOptions(t, path, map[string]string{"TEST_API_KEY": "secret"}))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadAppliesFlagEnvironmentProfileAndDefaultPrecedence(t *testing.T) {
	path := writeConfig(t, `
[profiles.default]
api_host = "file.qweatherapi.com"
auth = "api_key"
api_key_env = "TEST_API_KEY"
language = "zh"
unit = "metric"

[cache]
enabled = false
sensitive = false
stale = false
`)
	language := "en"
	options := testOptions(t, path, map[string]string{
		"TEST_API_KEY":             "do-not-print",
		"QWEATHER_API_HOST":        "env.qweatherapi.com",
		"QWEATHER_LANGUAGE":        "ja",
		"QWEATHER_UNIT":            "imperial",
		"QWEATHER_CACHE_ENABLED":   "true",
		"QWEATHER_CACHE_SENSITIVE": "true",
	})
	options.LanguageOverride = &language
	effective, diagnostics, err := Load(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if effective.APIHost != "env.qweatherapi.com" || effective.Language != "en" || effective.Unit != "imperial" {
		t.Fatalf("effective = %#v", effective)
	}
	if !effective.Cache.Enabled || !effective.Cache.Sensitive || effective.AuthMethod != auth.MethodAPIKey {
		t.Fatalf("effective cache/auth = %#v", effective)
	}
	if diagnostics.ValueSources["api_host"] != "environment" || diagnostics.ValueSources["language"] != "flag" || diagnostics.AuthSource != "TEST_API_KEY" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	encoded, err := json.Marshal(CheckResult{Valid: true, Effective: effective, Diagnostics: diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "do-not-print") {
		t.Fatal("serialized configuration leaked API key")
	}
}

func TestLoadSelectsAPIKeySource(t *testing.T) {
	tests := []struct {
		name        string
		fields      string
		environment map[string]string
		wantKey     string
		wantSource  string
		wantError   string
	}{
		{
			name:       "inline only",
			fields:     `api_key = " inline-key "`,
			wantKey:    "inline-key",
			wantSource: "profile.api_key",
		},
		{
			name: "custom environment overrides inline",
			fields: `api_key = "inline-key"
api_key_env = "CUSTOM_API_KEY"`,
			environment: map[string]string{"CUSTOM_API_KEY": "environment-key"},
			wantKey:     "environment-key",
			wantSource:  "CUSTOM_API_KEY",
		},
		{
			name:       "custom environment absence falls back to inline",
			fields:     "api_key = \"inline-key\"\napi_key_env = \"CUSTOM_API_KEY\"",
			wantKey:    "inline-key",
			wantSource: "profile.api_key",
		},
		{
			name:        "default environment overrides inline",
			fields:      `api_key = "inline-key"`,
			environment: map[string]string{"QWEATHER_API_KEY": "default-environment-key"},
			wantKey:     "default-environment-key",
			wantSource:  "QWEATHER_API_KEY",
		},
		{
			name:        "environment only remains supported",
			fields:      `api_key_env = "CUSTOM_API_KEY"`,
			environment: map[string]string{"CUSTOM_API_KEY": "environment-key"},
			wantKey:     "environment-key",
			wantSource:  "CUSTOM_API_KEY",
		},
		{
			name:        "explicit empty environment does not fall back",
			fields:      "api_key = \"inline-key\"\napi_key_env = \"CUSTOM_API_KEY\"",
			environment: map[string]string{"CUSTOM_API_KEY": ""},
			wantError:   "referenced API key environment variable CUSTOM_API_KEY is empty",
		},
		{
			name:      "missing sources",
			fields:    `api_key = "   "`,
			wantError: "API key is not configured",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfig(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
`+test.fields+"\n")
			effective, diagnostics, err := Load(context.Background(), testOptions(t, path, test.environment))
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("Load() error = %v", err)
				}
				if strings.Contains(err.Error(), "inline-key") || strings.Contains(err.Error(), "environment-key") {
					t.Fatal("Load() error leaked an API key")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			header, err := effective.Credentials.Header(time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			if header.Value != test.wantKey || diagnostics.AuthSource != test.wantSource || !diagnostics.SecretPresent {
				t.Fatalf("header source or secret presence did not match expectations")
			}
		})
	}
}

func TestLoadRejectsInlineAPIKeyInJWTProfile(t *testing.T) {
	keyPath := writePrivateKey(t, 0o600)
	path := writeConfig(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "jwt"
project_id = "project"
credential_id = "credential"
private_key_file = "`+keyPath+`"
api_key = "inline-key"
`)
	_, _, err := Load(context.Background(), testOptions(t, path, nil))
	if err == nil || !strings.Contains(err.Error(), "JWT profile must not configure api_key or api_key_env") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadProtectsInlineAPIKeyConfigurationPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	tests := []struct {
		name      string
		fields    string
		mode      os.FileMode
		wantError bool
	}{
		{name: "inline key in private file", fields: `api_key = "inline-key"`, mode: 0o600},
		{name: "inline key in group-readable file", fields: `api_key = "inline-key"`, mode: 0o640, wantError: true},
		{name: "inline key in other-readable file", fields: `api_key = "inline-key"`, mode: 0o604, wantError: true},
		{name: "environment reference in public file", fields: `api_key_env = "TEST_API_KEY"`, mode: 0o644},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfigMode(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
`+test.fields+"\n", test.mode)
			_, _, err := Load(context.Background(), testOptions(t, path, map[string]string{"TEST_API_KEY": "environment-key"}))
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "configuration file permissions") {
					t.Fatalf("Load() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLoadRejectsLoosePermissionsWhenUnselectedProfileHasInlineAPIKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	path := writeConfigMode(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
api_key_env = "TEST_API_KEY"

[profiles.unselected]
api_host = "example.qweatherapi.com"
auth = "api_key"
api_key = "inline-key"
`, 0o644)
	_, _, err := Load(context.Background(), testOptions(t, path, map[string]string{"TEST_API_KEY": "environment-key"}))
	if err == nil || !strings.Contains(err.Error(), "configuration file permissions") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsEmptyEnvironmentOverride(t *testing.T) {
	path := writeConfig(t, `
[profiles.default]
api_host = "file.qweatherapi.com"
auth = "api_key"
api_key_env = "TEST_API_KEY"
`)
	_, _, err := Load(context.Background(), testOptions(t, path, map[string]string{
		"TEST_API_KEY":      "secret",
		"QWEATHER_API_HOST": "",
	}))
	if err == nil || !strings.Contains(err.Error(), "QWEATHER_API_HOST is set but empty") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadGeneratedJWTAndExternalJWT(t *testing.T) {
	keyPath := writePrivateKey(t, 0o600)
	path := writeConfig(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "jwt"
project_id = "project"
credential_id = "credential"
private_key_file = "`+keyPath+`"
jwt_ttl = "20m"
`)
	effective, diagnostics, err := Load(context.Background(), testOptions(t, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	if effective.AuthMethod != auth.MethodGeneratedJWT || effective.JWTLifetime != 20*time.Minute || diagnostics.AuthSource != "private_key_file" {
		t.Fatalf("effective=%#v diagnostics=%#v", effective, diagnostics)
	}
	if _, err := effective.Credentials.Header(time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}

	external, externalDiagnostics, err := Load(context.Background(), testOptions(t, path, map[string]string{"QWEATHER_JWT": "header.payload.signature"}))
	if err != nil {
		t.Fatal(err)
	}
	if external.AuthMethod != auth.MethodExternalJWT || externalDiagnostics.AuthSource != "QWEATHER_JWT" {
		t.Fatalf("external=%#v diagnostics=%#v", external, externalDiagnostics)
	}
}

func TestLoadRejectsLoosePrivateKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	keyPath := writePrivateKey(t, 0o644)
	path := writeConfig(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "jwt"
project_id = "project"
credential_id = "credential"
private_key_file = "`+keyPath+`"
`)
	_, _, err := Load(context.Background(), testOptions(t, path, nil))
	if err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsMissingProfile(t *testing.T) {
	path := writeConfig(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
api_key_env = "TEST_API_KEY"
`)
	options := testOptions(t, path, map[string]string{"TEST_API_KEY": "secret"})
	options.Profile = "missing"
	_, _, err := Load(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), `profile "missing" does not exist`) {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsConflictingAuthenticationUnion(t *testing.T) {
	path := writeConfig(t, `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
project_id = "must-not-be-here"
api_key_env = "TEST_API_KEY"
`)
	_, _, err := Load(context.Background(), testOptions(t, path, map[string]string{"TEST_API_KEY": "secret"}))
	if err == nil || !strings.Contains(err.Error(), "must not configure JWT") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadSupportsEveryDocumentedEnvironmentOverride(t *testing.T) {
	keyPath := writePrivateKey(t, 0o600)
	path := writeConfig(t, `
[profiles.default]
api_host = "default.qweatherapi.com"
auth = "api_key"
api_key_env = "UNUSED_KEY"

[profiles.alt]
api_host = "file.qweatherapi.com"
auth = "jwt"
project_id = "file-project"
credential_id = "file-credential"
private_key_file = "/missing/private-key.pem"
jwt_ttl = "15m"
language = "auto"
unit = "metric"

[cache]
enabled = false
sensitive = false
`)
	options := testOptions(t, "", map[string]string{
		"QWEATHER_CONFIG":           path,
		"QWEATHER_PROFILE":          "alt",
		"QWEATHER_API_HOST":         "env.qweatherapi.com",
		"QWEATHER_PROJECT_ID":       "env-project",
		"QWEATHER_CREDENTIAL_ID":    "env-credential",
		"QWEATHER_PRIVATE_KEY_FILE": keyPath,
		"QWEATHER_JWT_TTL":          "30m",
		"QWEATHER_LANGUAGE":         "en",
		"QWEATHER_UNIT":             "imperial",
		"QWEATHER_CACHE_ENABLED":    "true",
		"QWEATHER_CACHE_SENSITIVE":  "true",
	})
	effective, diagnostics, err := Load(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if effective.ConfigPath != path || effective.Profile != "alt" || effective.APIHost != "env.qweatherapi.com" || effective.JWTLifetime != 30*time.Minute || effective.Language != "en" || effective.Unit != "imperial" || !effective.Cache.Enabled || !effective.Cache.Sensitive {
		t.Fatalf("effective = %#v", effective)
	}
	if diagnostics.ConfigSource != "environment" || diagnostics.ProfileSource != "environment" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	header, err := effective.Credentials.Header(time.Unix(1_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.TrimPrefix(header.Value, "Bearer "), ".")
	if len(parts) != 3 {
		t.Fatalf("JWT = %q", header.Value)
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(headerJSON), `"kid":"env-credential"`) || !strings.Contains(string(payloadJSON), `"sub":"env-project"`) {
		t.Fatalf("header=%s payload=%s", headerJSON, payloadJSON)
	}
}
