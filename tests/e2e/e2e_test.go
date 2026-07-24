//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const resultSchema = "qweather.result/v1"

type smokeCase struct {
	name           string
	capability     string
	responseFamily string
	args           []string
}

type resultEnvelope struct {
	Schema     string           `json:"schema"`
	Outcome    string           `json:"outcome"`
	Capability string           `json:"capability"`
	Operations []string         `json:"operations"`
	Policy     policyEnvelope   `json:"policy"`
	Cache      cacheEnvelope    `json:"cache"`
	Upstream   upstreamEnvelope `json:"upstream"`
	Data       json.RawMessage  `json:"data"`
}

type policyEnvelope struct {
	BillingGroup string `json:"billingGroup"`
}

type cacheEnvelope struct {
	Status            string `json:"status"`
	UpstreamRequested bool   `json:"upstreamRequested"`
}

type upstreamEnvelope struct {
	HTTPStatus     int    `json:"httpStatus"`
	ResponseFamily string `json:"responseFamily"`
}

func TestReleaseSmoke(t *testing.T) {
	binary := requireAbsoluteExecutable(t, "QWEATHER_E2E_BINARY")
	apiHost := requireEnvironment(t, "QWEATHER_API_HOST")
	apiKey := requireEnvironment(t, "QWEATHER_API_KEY")
	environment := isolatedEnvironment(t, apiHost, apiKey)

	tests := []smokeCase{
		{
			name:           "Geo city lookup",
			capability:     "geo.city.lookup",
			responseFamily: "code-refer-v1",
			args: []string{
				"geo", "city", "lookup", "--place-id", "101010100", "--limit", "1",
				"--lang", "en", "--output", "json", "--no-cache",
			},
		},
		{
			name:           "current city weather",
			capability:     "weather.city.current",
			responseFamily: "code-refer-v1",
			args: []string{
				"weather", "city", "current", "--place-id", "101010100",
				"--lang", "en", "--output", "json", "--no-cache",
			},
		},
		{
			name:           "current air quality",
			capability:     "air.current",
			responseFamily: "metadata-v1",
			args: []string{
				"air", "current", "--coordinate", "geo:39.90,116.41",
				"--lang", "en", "--output", "json", "--no-cache",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := execute(t, binary, environment, test.args)
			assertResult(t, result, test)
		})
	}
}

func requireAbsoluteExecutable(t *testing.T, name string) string {
	t.Helper()
	value := requireEnvironment(t, name)
	if !filepath.IsAbs(value) {
		t.Fatalf("%s must be an absolute path", name)
	}
	info, err := os.Stat(value)
	if err != nil {
		t.Fatalf("%s must point to an existing binary: %v", name, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s must point to an executable regular file", name)
	}
	return value
}

func requireEnvironment(t *testing.T, name string) string {
	t.Helper()
	value, exists := os.LookupEnv(name)
	if !exists || strings.TrimSpace(value) == "" {
		t.Fatalf("%s is required for the live release smoke", name)
	}
	return strings.TrimSpace(value)
}

func isolatedEnvironment(t *testing.T, apiHost, apiKey string) []string {
	t.Helper()
	home := t.TempDir()
	environment := make([]string, 0, len(os.Environ())+5)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "QWEATHER_") || slices.Contains([]string{
			"HOME", "XDG_CACHE_HOME", "XDG_CONFIG_HOME",
		}, name) {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment,
		"HOME="+home,
		"XDG_CACHE_HOME="+filepath.Join(home, "cache"),
		"XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		"QWEATHER_API_HOST="+apiHost,
		"QWEATHER_API_KEY="+apiKey,
	)
}

func execute(t *testing.T, binary string, environment, args []string) resultEnvelope {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Env = environment
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("qweather process failed without exposing captured output: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatal("qweather wrote diagnostics to stderr on success; captured output suppressed")
	}

	decoder := json.NewDecoder(&stdout)
	var result resultEnvelope
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("qweather stdout is not a valid result envelope; captured output suppressed: %v", err)
	}
	if err := ensureEOF(decoder); err != nil {
		t.Fatalf("qweather stdout contains data after the result envelope; captured output suppressed: %v", err)
	}
	return result
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("multiple JSON values")
}

func assertResult(t *testing.T, result resultEnvelope, test smokeCase) {
	t.Helper()
	if result.Schema != resultSchema {
		t.Errorf("schema = %q, want %q", result.Schema, resultSchema)
	}
	if result.Outcome != "ok" && result.Outcome != "no_data" {
		t.Errorf("outcome = %q, want ok or no_data", result.Outcome)
	}
	if result.Capability != test.capability {
		t.Errorf("capability = %q, want %q", result.Capability, test.capability)
	}
	if !slices.Equal(result.Operations, []string{test.capability}) {
		t.Errorf("operations = %q, want only %q", result.Operations, test.capability)
	}
	if result.Policy.BillingGroup != "basic" {
		t.Errorf("billing group = %q, want basic", result.Policy.BillingGroup)
	}
	if result.Cache.Status != "disabled" || !result.Cache.UpstreamRequested {
		t.Errorf("cache metadata does not confirm --no-cache and an upstream request")
	}
	if result.Upstream.HTTPStatus < 200 || result.Upstream.HTTPStatus >= 300 {
		t.Errorf("upstream HTTP status = %d, want 2xx", result.Upstream.HTTPStatus)
	}
	if result.Upstream.ResponseFamily != test.responseFamily {
		t.Errorf("response family = %q, want %q", result.Upstream.ResponseFamily, test.responseFamily)
	}
	var data map[string]any
	if len(result.Data) == 0 || json.Unmarshal(result.Data, &data) != nil || data == nil {
		t.Error("data must be a JSON object; provider body suppressed")
	}
}
