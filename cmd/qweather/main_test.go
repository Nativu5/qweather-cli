package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/app"
	"github.com/Nativu5/qweather-cli/internal/buildinfo"
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/cli"
)

func TestConfigCheckIsOfflineAndSecretFree(t *testing.T) {
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
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	root, err := cli.NewRoot(registry, app.NewDefault(), buildinfo.Current("test"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := cli.Execute(context.Background(), root, []string{"config", "check", "--config", path}, &stdout, &stderr)
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
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	root, err := cli.NewRoot(registry, app.NewDefault(), buildinfo.Current("test"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := cli.Execute(context.Background(), root, []string{"config", "check", "--config", path}, &stdout, &stderr)
	if exit != 3 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "must-not-appear") || strings.Contains(stderr.String(), "must-not-appear") {
		t.Fatal("config check failure leaked an API key")
	}
	if !strings.Contains(stderr.String(), `"code":"CONFIG_INVALID"`) {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
