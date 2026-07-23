package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/buildinfo"
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

type recordingRuntime struct {
	invocations []Invocation
}

func (r *recordingRuntime) Run(_ context.Context, invocation Invocation) (*output.Result, *output.Problem) {
	r.invocations = append(r.invocations, invocation)
	body := []byte(`{"code":"200","now":{"temp":"20"}}`)
	return &output.Result{
		Schema:       output.ResultSchema,
		Outcome:      "ok",
		Capability:   invocation.Capability.ID,
		Operations:   []string{invocation.Capability.ID},
		Policy:       output.Policy{BillingGroup: string(invocation.Capability.BillingGroup)},
		Cache:        output.Cache{Status: "disabled", UpstreamRequested: true},
		Upstream:     output.Upstream{HTTPStatus: 200, ResponseFamily: string(invocation.Capability.Upstream.ResponseFamily)},
		Attribution:  []any{},
		Data:         json.RawMessage(body),
		ProviderBody: body,
	}, nil
}

func (*recordingRuntime) CheckConfig(context.Context, CommonOptions) (any, *output.Problem) {
	return map[string]any{"valid": true}, nil
}

func (*recordingRuntime) CacheStatus(context.Context, CacheControlOptions) (any, *output.Problem) {
	return map[string]any{"entries": 0}, nil
}

func (*recordingRuntime) CacheClear(context.Context, CacheControlOptions) (any, *output.Problem) {
	return map[string]any{"cleared": 0}, nil
}

func newTestRoot(t *testing.T, runtime Runtime) *cobra.Command {
	t.Helper()
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	root, err := NewRoot(registry, runtime, buildinfo.Current("test-hash"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func runCommand(t *testing.T, runtime Runtime, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit := Execute(context.Background(), newTestRoot(t, runtime), args, &stdout, &stderr)
	return exit, stdout.String(), stderr.String()
}

func TestCommandTreeContainsAcceptedPaths(t *testing.T) {
	root := newTestRoot(t, &recordingRuntime{})
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range registry.Current() {
		command, _, err := root.Find(strings.Fields(capability.CommandPath))
		if err != nil || command.CommandPath() != "qweather "+capability.CommandPath {
			t.Errorf("Find(%q) = %v, %v", capability.CommandPath, command, err)
		}
	}
	for _, path := range []string{"capability list", "capability show", "config check", "cache status", "cache clear", "version"} {
		command, _, err := root.Find(strings.Fields(path))
		if err != nil || command.CommandPath() != "qweather "+path {
			t.Errorf("Find(%q) = %v, %v", path, command, err)
		}
	}
}

func TestRootHelpHasNoUndocumentedCompletionCommand(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{}, "--help")
	if exit != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", exit, stderr)
	}
	for _, domain := range []string{"geo", "weather", "alert", "air", "storm", "marine", "solar", "astronomy", "account"} {
		if !strings.Contains(stdout, "  "+domain) {
			t.Errorf("help missing domain %q", domain)
		}
	}
	if strings.Contains(stdout, "completion") {
		t.Fatal("help unexpectedly exposes completion command")
	}
}

func TestCapabilityDiscoveryIsOfflineAndDeterministic(t *testing.T) {
	runtime := &recordingRuntime{}
	exit1, stdout1, stderr1 := runCommand(t, runtime, "capability", "list", "--lifecycle", "all", "--format", "json")
	exit2, stdout2, stderr2 := runCommand(t, runtime, "capability", "list", "--lifecycle", "all", "--format", "json")
	if exit1 != 0 || exit2 != 0 || stderr1 != "" || stderr2 != "" {
		t.Fatalf("unexpected command failure: %d %d %q %q", exit1, exit2, stderr1, stderr2)
	}
	if stdout1 != stdout2 {
		t.Fatal("capability output is not deterministic")
	}
	var capabilities []catalog.Capability
	if err := json.Unmarshal([]byte(stdout1), &capabilities); err != nil {
		t.Fatal(err)
	}
	if len(capabilities) != 33 {
		t.Fatalf("capability count = %d, want 33", len(capabilities))
	}
	if len(runtime.invocations) != 0 {
		t.Fatal("offline discovery invoked network runtime")
	}

	exit, stdout, stderr := runCommand(t, runtime, "capability", "show", "legacy.alert.current", "--format", "json")
	if exit != 0 || stderr != "" || !strings.Contains(stdout, `"lifecycle":"deprecated"`) {
		t.Fatalf("Tombstone output: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestNetworkLeafParsesCommonAndTypedFlags(t *testing.T) {
	runtime := &recordingRuntime{}
	exit, stdout, stderr := runCommand(t, runtime,
		"weather", "city", "daily", "--place-id", "101010100", "--days", "7", "--timeout", "2s",
	)
	if exit != 0 || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	if len(runtime.invocations) != 1 {
		t.Fatalf("invocation count = %d", len(runtime.invocations))
	}
	invocation := runtime.invocations[0]
	if invocation.Capability.ID != "weather.city.forecast.daily" || invocation.Input.Days != 7 || invocation.Common.Timeout.String() != "2s" {
		t.Fatalf("unexpected invocation: %#v", invocation)
	}
	if !strings.Contains(stdout, `"schema":"qweather.result/v1"`) {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestBodyOutputRemainsProviderOnly(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{},
		"weather", "city", "current", "--place-id", "101010100", "--output", "body",
	)
	if exit != 0 || stderr != "" || stdout != "{\"code\":\"200\",\"now\":{\"temp\":\"20\"}}\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestInvalidTypedInputUsesProblemEnvelope(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{},
		"weather", "city", "daily", "--place-id", "101010100", "--days", "5",
	)
	if exit != 2 || stdout != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	if !strings.Contains(stderr, `"schema":"qweather.problem/v1"`) || !strings.Contains(stderr, `"code":"INVALID_INVOCATION"`) {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSemanticInputValidationPrecedesRuntime(t *testing.T) {
	tests := [][]string{
		{"geo", "city", "lookup", "--coordinate", "geo:39.123,116.4"},
		{"weather", "history", "--place-id", "101010100", "--date", "2026-02-30"},
		{"air", "station", "--air-station-id", "../secret"},
		{"weather", "minutely", "--coordinate", "geo:39.9,116.4", "--lang", "fr"},
	}
	for _, args := range tests {
		runtime := &recordingRuntime{}
		exit, stdout, stderr := runCommand(t, runtime, args...)
		if exit != 2 || stdout != "" || !strings.Contains(stderr, `"code":"INVALID_INVOCATION"`) || len(runtime.invocations) != 0 {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q invocations=%d", args, exit, stdout, stderr, len(runtime.invocations))
		}
	}
}

func TestCacheClearRejectsUnknownOrDeprecatedCapability(t *testing.T) {
	for _, capabilityID := range []string{"missing.capability", "legacy.alert.current"} {
		exit, stdout, stderr := runCommand(t, &recordingRuntime{}, "cache", "clear", "--capability", capabilityID)
		if exit != 2 || stdout != "" || !strings.Contains(stderr, `"code":"INVALID_INVOCATION"`) {
			t.Fatalf("capability=%q exit=%d stdout=%q stderr=%q", capabilityID, exit, stdout, stderr)
		}
	}
}

func TestNoSecretFlagsAreExposed(t *testing.T) {
	root := newTestRoot(t, &recordingRuntime{})
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		for _, forbidden := range []string{"api-key", "jwt", "private-key", "authorization"} {
			if command.Flags().Lookup(forbidden) != nil {
				t.Errorf("%s exposes forbidden flag --%s", command.CommandPath(), forbidden)
			}
		}
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(root)
}
