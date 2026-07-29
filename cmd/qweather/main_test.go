package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCheckIsOfflineAndSecretFree(t *testing.T) {
	isolateQWeatherEnvironment(t)
	configuration := `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
api_key = "inline-must-not-appear"
api_key_env = "QWEATHER_API_KEY"
language = "auto"
unit = "metric"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QWEATHER_API_KEY", "must-not-appear")
	var stdout, stderr bytes.Buffer
	exit := run([]string{"config", "check", "--config", path, "--output", "json"}, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "must-not-appear") || strings.Contains(stderr.String(), "must-not-appear") {
		t.Fatal("config check leaked the API key")
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["valid"] != true {
		t.Fatalf("result = %#v", result)
	}
}

func TestConfigCheckFailureIsSecretFree(t *testing.T) {
	isolateQWeatherEnvironment(t)
	configuration := `
[profiles.default]
api_host = "example.qweatherapi.com"
auth = "api_key"
api_key = "inline-must-not-appear"
api_key_env = "QWEATHER_API_KEY"
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QWEATHER_API_KEY", "environment-must-not-appear\nsuffix")
	var stdout, stderr bytes.Buffer
	exit := run([]string{"config", "check", "--config", path}, &stdout, &stderr)
	if exit != 3 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "must-not-appear") || strings.Contains(stderr.String(), "must-not-appear") {
		t.Fatal("config check failure leaked an API key")
	}
	if !strings.Contains(stderr.String(), "QWeather configuration is invalid\nCode: CONFIG_INVALID\n") || strings.Contains(stderr.String(), `"schema"`) {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func isolateQWeatherEnvironment(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "QWEATHER_") {
			continue
		}
		value, existed := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}
