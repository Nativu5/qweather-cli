package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/buildinfo"
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

type recordingRuntime struct {
	invocations []Invocation
	body        []byte
	problem     *output.Problem
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (r *recordingRuntime) Run(_ context.Context, invocation Invocation) (*output.Result, *output.Problem) {
	r.invocations = append(r.invocations, invocation)
	if r.problem != nil {
		return nil, r.problem
	}
	body := r.body
	if len(body) == 0 {
		body = []byte(`{"code":"200","now":{"temp":"20"}}`)
	}
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
		Unit:         "metric",
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
	if !strings.Contains(stdout, "--output string") || !strings.Contains(stdout, "(default \"text\")") {
		t.Fatalf("help does not document the text-first output mode: %s", stdout)
	}
	for _, removed := range []string{"--pretty", "--format", "--json"} {
		if strings.Contains(stdout, removed) {
			t.Errorf("help unexpectedly exposes removed flag %s", removed)
		}
	}
}

func TestNetworkBranchHelpUsesUsefulDescriptions(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		want     string
		unwanted string
	}{
		{
			name:     "weather",
			args:     []string{"weather", "--help"},
			want:     "Weather observations, forecasts, precipitation, indices, and history",
			unwanted: "QWeather weather commands",
		},
		{
			name:     "weather city",
			args:     []string{"weather", "city", "--help"},
			want:     "Weather for cities and named places",
			unwanted: "QWeather weather city commands",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exit, stdout, stderr := runCommand(t, &recordingRuntime{}, test.args...)
			if exit != 0 || stderr != "" {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			if !strings.Contains(stdout, test.want) || strings.Contains(stdout, test.unwanted) {
				t.Fatalf("help=%q; want %q and not %q", stdout, test.want, test.unwanted)
			}
		})
	}
}

func TestCapabilityHelpProjectsConstraintsAndSafetyRules(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		wants []string
	}{
		{
			name: "daily forecast",
			args: []string{"weather", "city", "daily", "--help"},
			wants: []string{
				"Exactly one of --place, --place-id, or --coordinate is required.",
				"--country and --adm are valid only with --place.",
				"--days int            forecast length in days (required; one of 3, 7, 10, 15, 30)",
				"Examples:",
				"qweather weather city daily --place Beijing --days 7 --output text",
			},
		},
		{
			name: "weather indices",
			args: []string{"weather", "indices", "--help"},
			wants: []string{
				"--index and --all-indices are mutually exclusive; --index values must be between 1 and 16.",
				"--index ints          weather index type; repeatable (range 1..16)",
			},
		},
		{
			name: "grid target",
			args: []string{"weather", "grid", "current", "--help"},
			wants: []string{
				"The selected target must resolve to coordinates.",
			},
		},
		{
			name: "history target",
			args: []string{"weather", "history", "--help"},
			wants: []string{
				"The selected target must resolve to a QWeather Location ID.",
			},
		},
		{
			name: "geo lookup",
			args: []string{"geo", "city", "lookup", "--help"},
			wants: []string{
				"Exactly one of --query, --place-id, or --coordinate is required.",
				"--country and --adm are valid only with --query.",
			},
		},
		{
			name: "solar forecast",
			args: []string{"solar", "forecast", "--help"},
			wants: []string{
				"--hours int              forecast length in hours (range 1..60) (default 24)",
				"--interval-min int",
				"forecast interval in minutes (one of 15, 30, 60) (default 60)",
				"--include strings",
				"optional dataset; repeatable (one of weather, poa)",
				"--include poa requires --tilt-deg and --azimuth-deg.",
				"pass --allow-product solar before network I/O",
			},
		},
		{
			name: "marine tide",
			args: []string{"marine", "tide", "--help"},
			wants: []string{
				"--tide-station-id string   QWeather tide station ID (required)",
				"--date string              UTC date from today through 9 days ahead in YYYY-MM-DD form (required)",
				"pass --allow-product marine before network I/O",
			},
		},
		{
			name: "account usage",
			args: []string{"account", "usage", "--help"},
			wants: []string{
				"--project-id and --credential-id are mutually exclusive.",
				"pass --allow-sensitive-output account before network I/O",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exit, stdout, stderr := runCommand(t, &recordingRuntime{}, test.args...)
			if exit != 0 || stderr != "" {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			for _, want := range test.wants {
				if !strings.Contains(stdout, want) {
					t.Errorf("help missing %q:\n%s", want, stdout)
				}
			}
		})
	}
}

func TestCurrentCapabilityHelpContainsRegistryFlagMetadata(t *testing.T) {
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range registry.Current() {
		t.Run(capability.ID, func(t *testing.T) {
			args := append(strings.Fields(capability.CommandPath), "--help")
			runtime := &recordingRuntime{}
			exit, stdout, stderr := runCommand(t, runtime, args...)
			if exit != 0 || stderr != "" {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			if len(runtime.invocations) != 0 {
				t.Fatalf("help invoked network runtime: %#v", runtime.invocations)
			}
			for _, flag := range capability.Flags {
				line := helpFlagLine(stdout, flag)
				if line == "" {
					t.Errorf("help is missing --%s", flag.Name)
					continue
				}
				if flag.Required && !strings.Contains(line, "(required") {
					t.Errorf("--%s is not marked required: %s", flag.Name, line)
				}
				if len(flag.Enum) > 0 && !strings.Contains(line, "one of "+strings.Join(flag.Enum, ", ")) {
					t.Errorf("--%s is missing enum values: %s", flag.Name, line)
				}
				if len(flag.IntEnum) > 0 {
					values := make([]string, len(flag.IntEnum))
					for index, value := range flag.IntEnum {
						values[index] = strconv.Itoa(value)
					}
					if !strings.Contains(line, "one of "+strings.Join(values, ", ")) {
						t.Errorf("--%s is missing integer enum values: %s", flag.Name, line)
					}
				}
				if flag.Min != nil || flag.Max != nil {
					if !strings.Contains(line, "range "+flagRange(flag.Min, flag.Max)) {
						t.Errorf("--%s is missing range: %s", flag.Name, line)
					}
				}
				if flag.Default != "" && !strings.Contains(line, "(default "+flag.Default+")") {
					t.Errorf("--%s is missing default: %s", flag.Name, line)
				}
			}

			safety := ""
			switch capability.ProductGate {
			case catalog.GateMarine:
				safety = "pass --allow-product marine before network I/O"
			case catalog.GateSolar:
				safety = "pass --allow-product solar before network I/O"
			case catalog.GateSensitiveAccount:
				safety = "pass --allow-sensitive-output account before network I/O"
			}
			if safety != "" && !strings.Contains(stdout, safety) {
				t.Errorf("help is missing Product Gate safety boundary %q:\n%s", safety, stdout)
			}
		})
	}
}

func helpFlagLine(help string, flag catalog.Flag) string {
	prefix := "--" + flag.Name + " "
	if flag.Kind != catalog.FlagBool {
		prefix += flagTypeName(flag.Kind) + " "
	}
	for _, line := range strings.Split(help, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return trimmed
		}
	}
	return ""
}

func flagTypeName(kind catalog.FlagKind) string {
	switch kind {
	case catalog.FlagString:
		return "string"
	case catalog.FlagStringSlice:
		return "strings"
	case catalog.FlagInt, catalog.FlagFloat:
		if kind == catalog.FlagFloat {
			return "float"
		}
		return "int"
	case catalog.FlagIntSlice:
		return "ints"
	default:
		return ""
	}
}

func TestCapabilityDiscoveryIsOfflineAndDeterministic(t *testing.T) {
	runtime := &recordingRuntime{}
	exit1, stdout1, stderr1 := runCommand(t, runtime, "capability", "list", "--lifecycle", "all", "--output", "json")
	exit2, stdout2, stderr2 := runCommand(t, runtime, "capability", "list", "--lifecycle", "all", "--output", "json")
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

	exit, stdout, stderr := runCommand(t, runtime, "capability", "show", "legacy.alert.current", "--output", "json")
	if exit != 0 || stderr != "" || !strings.Contains(stdout, `"lifecycle":"deprecated"`) {
		t.Fatalf("Tombstone output: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestNetworkLeafParsesCommonAndTypedFlags(t *testing.T) {
	runtime := &recordingRuntime{}
	exit, stdout, stderr := runCommand(t, runtime,
		"weather", "city", "daily", "--place-id", "101010100", "--days", "7", "--timeout", "2s", "--output", "json",
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
	if exit != 0 || stderr != "" || stdout != "{\"code\":\"200\",\"now\":{\"temp\":\"20\"}}" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestBodyModeUsesTextForQWeatherOwnedFailure(t *testing.T) {
	problem := output.NewProblem(8, "TIMEOUT", "provider request timed out")
	problem.Capability = "weather.city.current"
	problem.Retryable = true
	exit, stdout, stderr := runCommand(t, &recordingRuntime{problem: problem},
		"weather", "city", "current", "--place-id", "101010100", "--output", "body",
	)
	if exit != 8 || stdout != "" || !strings.HasPrefix(stderr, "provider request timed out\nCode: TIMEOUT\nCapability: weather.city.current\nRetryable: true\n") || strings.Contains(stderr, `"schema"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestProviderOutputFailureIncludesCapability(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Text",
			args:     []string{"weather", "city", "current", "--place-id", "101010100"},
			expected: "failed to write command output\nCode: OUTPUT_ERROR\nCapability: weather.city.current\nRetryable: false\n",
		},
		{
			name:     "JSON",
			args:     []string{"weather", "city", "current", "--place-id", "101010100", "--output", "json"},
			expected: `{"schema":"qweather.problem/v1","code":"OUTPUT_ERROR","message":"failed to write command output","capability":"weather.city.current","retryable":false}` + "\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			exit := Execute(context.Background(), newTestRoot(t, &recordingRuntime{}), test.args, failingWriter{}, &stderr)
			if exit != 10 || stderr.String() != test.expected {
				t.Fatalf("exit=%d stderr=%q, want %q", exit, stderr.String(), test.expected)
			}
		})
	}
}

func TestTextFallbackDiagnosticRequiresDebug(t *testing.T) {
	body := []byte(`{"code":"200","unexpected":{"kept":true}}`)
	args := []string{"weather", "city", "current", "--place-id", "101010100"}
	exit, stdout, stderr := runCommand(t, &recordingRuntime{body: body}, args...)
	if exit != 0 || stderr != "" || !strings.Contains(stdout, "Provider data:") || !strings.Contains(stdout, "kept: true") {
		t.Fatalf("without debug: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	args = append(args, "--debug")
	exit, stdout, stderr = runCommand(t, &recordingRuntime{body: body}, args...)
	if exit != 0 || !strings.Contains(stdout, "Provider data:") || !strings.Contains(stderr, `"event":"text.fallback"`) {
		t.Fatalf("with debug: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestInvalidTypedInputUsesTextProblemByDefault(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{},
		"weather", "city", "daily", "--place-id", "101010100", "--days", "5",
	)
	if exit != 2 || stdout != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	if !strings.HasPrefix(stderr, "--days has an unsupported value\nCode: INVALID_INVOCATION\n") || strings.Contains(stderr, `"schema"`) {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestInvalidTypedInputUsesMachineProblemWhenSelected(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{},
		"weather", "city", "daily", "--place-id", "101010100", "--days", "5", "--output", "json",
	)
	if exit != 2 || stdout != "" || !strings.Contains(stderr, `"schema":"qweather.problem/v1"`) || !strings.Contains(stderr, `"code":"INVALID_INVOCATION"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestCobraInvocationErrorsAlwaysUseText(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{}, "accouny", "--output", "json")
	if exit != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: unknown command \"accouny\" for \"qweather\"") || strings.Contains(stderr, `"schema"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
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
		args = append(args, "--output", "json")
		exit, stdout, stderr := runCommand(t, runtime, args...)
		if exit != 2 || stdout != "" || !strings.Contains(stderr, `"code":"INVALID_INVOCATION"`) || len(runtime.invocations) != 0 {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q invocations=%d", args, exit, stdout, stderr, len(runtime.invocations))
		}
	}
}

func TestIssueSevenTypedValidationPrecedesRuntime(t *testing.T) {
	today := time.Now().UTC()
	tests := [][]string{
		{"storm", "list", "--year", "2017", "--allow-product", "marine"},
		{"storm", "list", "--year", strconv.Itoa(today.Year() + 2), "--allow-product", "marine"},
		{"storm", "track", "--storm-id", "", "--allow-product", "marine"},
		{"marine", "tide", "--tide-station-id", "P66981", "--date", "2026-02-30", "--allow-product", "marine"},
		{"marine", "tide", "--tide-station-id", "P66981", "--date", today.AddDate(0, 0, catalog.TideDateWindowDays+1).Format("2006-01-02"), "--allow-product", "marine"},
		{"solar", "forecast", "--coordinate", "geo:39.9,116.4", "--hours", "61", "--allow-product", "solar"},
		{"solar", "forecast", "--coordinate", "geo:39.9,116.4", "--interval-min", "45", "--allow-product", "solar"},
		{"solar", "forecast", "--coordinate", "geo:39.9,116.4", "--include", "poa", "--allow-product", "solar"},
		{"solar", "forecast", "--coordinate", "geo:39.9,116.4", "--tilt-deg", "30.5", "--allow-product", "solar"},
		{"astronomy", "position", "--coordinate", "geo:39.9,116.4", "--at", "not-a-time", "--altitude-m", "43"},
		{"astronomy", "sun", "--place-id", "101010100", "--date", today.AddDate(0, 0, catalog.AstronomyDateWindowDays+1).Format("2006-01-02")},
		{"astronomy", "position", "--coordinate", "geo:39.9,116.4", "--at", "2026-07-25T12:30:00+08:00", "--altitude-m", "NaN"},
		{"account", "usage", "--project-id", "project_123", "--credential-id", "cred_abc", "--allow-sensitive-output", "account"},
	}
	for _, args := range tests {
		runtime := &recordingRuntime{}
		args = append(args, "--output", "json")
		exit, stdout, stderr := runCommand(t, runtime, args...)
		cobraParseError := strings.Contains(strings.Join(args, " "), "--tilt-deg 30.5")
		expectedError := strings.Contains(stderr, `"code":"INVALID_INVOCATION"`)
		if cobraParseError {
			expectedError = strings.HasPrefix(stderr, "Error: invalid argument") && !strings.Contains(stderr, `"schema"`)
		}
		if exit != 2 || stdout != "" || !expectedError || len(runtime.invocations) != 0 {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q invocations=%d", args, exit, stdout, stderr, len(runtime.invocations))
		}
	}
}

func TestIssueSevenTypedFlagsReachRuntime(t *testing.T) {
	runtime := &recordingRuntime{}
	exit, _, stderr := runCommand(t, runtime,
		"solar", "forecast", "--coordinate", "geo:39.9,116.4",
		"--hours", "12", "--interval-min", "15", "--include", "weather,poa",
		"--tilt-deg", "30", "--azimuth-deg", "180", "--local-time", "--allow-product", "solar",
	)
	if exit != 0 || stderr != "" || len(runtime.invocations) != 1 {
		t.Fatalf("exit=%d stderr=%q invocations=%d", exit, stderr, len(runtime.invocations))
	}
	invocation := runtime.invocations[0]
	if invocation.Capability.ID != "solar.radiation.forecast" || invocation.Input.Hours != 12 || invocation.Input.IntervalMinutes != 15 || invocation.Input.TiltDegrees != 30 || invocation.Input.AzimuthDegrees != 180 || !invocation.Input.LocalTime || invocation.Input.AllowProduct != "solar" {
		t.Fatalf("invocation = %#v", invocation)
	}
	if len(invocation.Input.Includes) != 2 || invocation.Input.Includes[0] != "weather" || invocation.Input.Includes[1] != "poa" {
		t.Fatalf("includes = %#v", invocation.Input.Includes)
	}
}

func TestSolarHelpShowsProviderDefaults(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{}, "solar", "forecast", "--help")
	if exit != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	if !strings.Contains(stdout, "--hours int") || !strings.Contains(stdout, "(default 24)") || !strings.Contains(stdout, "--interval-min int") || !strings.Contains(stdout, "(default 60)") {
		t.Fatalf("help = %q", stdout)
	}
}

func TestCacheClearRejectsUnknownOrDeprecatedCapability(t *testing.T) {
	for _, capabilityID := range []string{"missing.capability", "legacy.alert.current"} {
		exit, stdout, stderr := runCommand(t, &recordingRuntime{}, "cache", "clear", "--capability", capabilityID, "--output", "json")
		if exit != 2 || stdout != "" || !strings.Contains(stderr, `"code":"INVALID_INVOCATION"`) {
			t.Fatalf("capability=%q exit=%d stdout=%q stderr=%q", capabilityID, exit, stdout, stderr)
		}
	}
}

func TestLocalCommandsRejectBodyAndUseGlobalOutput(t *testing.T) {
	exit, stdout, stderr := runCommand(t, &recordingRuntime{}, "version")
	if exit != 0 || stderr != "" || !strings.HasPrefix(stdout, "qweather ") {
		t.Fatalf("Text version: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	exit, stdout, stderr = runCommand(t, &recordingRuntime{}, "version", "--output", "json")
	if exit != 0 || stderr != "" || !strings.Contains(stdout, `"registryHash":"test-hash"`) {
		t.Fatalf("JSON version: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	exit, stdout, stderr = runCommand(t, &recordingRuntime{}, "cache", "status", "--output", "body")
	if exit != 2 || stdout != "" || !strings.Contains(stderr, "--output body is not available") || strings.Contains(stderr, `"schema"`) {
		t.Fatalf("body local command: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	exit, stdout, stderr = runCommand(t, &recordingRuntime{}, "cache", "status")
	if exit != 0 || stderr != "" || !strings.Contains(stdout, "entries: 0\n") || strings.Contains(stdout, "{") {
		t.Fatalf("Text cache status: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
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

func TestRemovedFormattingFlagsAreAbsent(t *testing.T) {
	root := newTestRoot(t, &recordingRuntime{})
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		for _, removed := range []string{"pretty", "format", "json"} {
			if command.Flags().Lookup(removed) != nil || command.PersistentFlags().Lookup(removed) != nil {
				t.Errorf("%s exposes removed flag --%s", command.CommandPath(), removed)
			}
		}
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(root)
}
