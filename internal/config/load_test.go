package config

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
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
