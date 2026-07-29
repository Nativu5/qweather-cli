package cli

import (
	"strings"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/catalog"
)

func TestNetworkBranchHelpUsesSemanticSummaries(t *testing.T) {
	tests := []struct {
		path []string
		want string
	}{
		{[]string{"weather"}, "Weather observations, forecasts, precipitation, indices, and history"},
		{[]string{"weather", "city"}, "Weather for cities and named places"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			exit, stdout, stderr := runCommand(t, &recordingRuntime{}, append(test.path, "--help")...)
			if exit != 0 || stderr != "" {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			if !strings.Contains(stdout, test.want) || strings.Contains(stdout, "QWeather "+strings.Join(test.path, " ")+" commands") {
				t.Fatalf("unexpected help:\n%s", stdout)
			}
		})
	}
}

func TestProductGateUsesLeafLocalYesFlag(t *testing.T) {
	root := newTestRoot(t, &recordingRuntime{})
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range registry.Current() {
		t.Run(capability.ID, func(t *testing.T) {
			command, _, err := root.Find(strings.Fields(capability.CommandPath))
			if err != nil {
				t.Fatal(err)
			}
			yes := command.Flags().Lookup("yes")
			if capability.ProductGate == catalog.GateNone {
				if yes != nil {
					t.Fatal("ungated capability exposes --yes")
				}
			} else if yes == nil || yes.Value.Type() != "bool" || yes.Shorthand != "" {
				t.Fatalf("gated capability --yes = %#v", yes)
			}
			for _, removed := range []string{"allow-product", "allow-sensitive-output"} {
				if command.Flags().Lookup(removed) != nil {
					t.Errorf("help still exposes removed --%s", removed)
				}
			}
		})
	}
	for _, path := range []string{"capability list", "config check", "cache status", "version"} {
		command, _, err := root.Find(strings.Fields(path))
		if err != nil {
			t.Fatal(err)
		}
		if command.Flags().Lookup("yes") != nil {
			t.Errorf("local command %q exposes --yes", path)
		}
	}
}

func TestRemovedProductGateFlagsAreUnknown(t *testing.T) {
	tests := [][]string{
		{"marine", "tide", "--allow-product", "marine"},
		{"account", "usage", "--allow-sensitive-output", "account"},
	}
	for _, args := range tests {
		exit, stdout, stderr := runCommand(t, &recordingRuntime{}, args...)
		if exit != 2 || stdout != "" || !strings.Contains(stderr, "unknown flag") {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", args, exit, stdout, stderr)
		}
	}
}

func TestProductGateYesReachesRuntime(t *testing.T) {
	runtime := &recordingRuntime{}
	exit, _, stderr := runCommand(t, runtime,
		"solar", "forecast", "--coordinate", "geo:39.9,116.4", "--yes",
	)
	if exit != 0 || stderr != "" || len(runtime.invocations) != 1 {
		t.Fatalf("exit=%d stderr=%q invocations=%d", exit, stderr, len(runtime.invocations))
	}
	if !runtime.invocations[0].GateAcknowledged {
		t.Fatal("--yes did not acknowledge the Product Gate")
	}
}

func TestRepresentativeCapabilityHelpRules(t *testing.T) {
	tests := []struct {
		path  []string
		wants []string
	}{
		{[]string{"weather", "city", "daily"}, []string{
			"Exactly one of --place, --place-id, or --coordinate is required.",
			"--country and --adm are valid only with --place.",
			"qweather weather city daily --place Beijing --days 7 --output text",
		}},
		{[]string{"weather", "indices"}, []string{
			"Exactly one of --index or --all-indices is required; --index values must be between 1 and 16 and unique.",
		}},
		{[]string{"solar", "forecast"}, []string{
			"--include poa requires --tilt-deg and --azimuth-deg.",
		}},
		{[]string{"marine", "tide"}, []string{
			"--date \"$(date -u +%F)\"",
		}},
		{[]string{"account", "usage"}, []string{
			"--project-id and --credential-id are mutually exclusive.",
		}},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			exit, stdout, stderr := runCommand(t, &recordingRuntime{}, append(test.path, "--help")...)
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

func TestCapabilityFlagUsage(t *testing.T) {
	minimum, maximum := 1.0, 60.0
	tests := []struct {
		name string
		flag catalog.Flag
		want string
	}{
		{"plain", catalog.Flag{Usage: "response language"}, "response language"},
		{"required enum", catalog.Flag{Usage: "forecast length", Required: true, IntEnum: []int{3, 7}}, "forecast length (required; one of 3, 7)"},
		{"string enum", catalog.Flag{Usage: "measurement unit", Enum: []string{"metric", "imperial"}}, "measurement unit (one of metric, imperial)"},
		{"range", catalog.Flag{Usage: "forecast length", Min: &minimum, Max: &maximum}, "forecast length (range 1..60)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := capabilityFlagUsage(test.flag); got != test.want {
				t.Fatalf("capabilityFlagUsage() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCurrentCapabilityHelpProjectsRegistryUsageOffline(t *testing.T) {
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
			exit, repeated, stderr := runCommand(t, runtime, args...)
			if exit != 0 || stderr != "" || repeated != stdout {
				t.Fatalf("help is not deterministic: exit=%d stderr=%q", exit, stderr)
			}
			if len(runtime.invocations) != 0 {
				t.Fatalf("help invoked network runtime: %#v", runtime.invocations)
			}
			for _, flag := range capability.Flags {
				line := helpFlagLine(stdout, flag)
				if line == "" || !strings.Contains(line, capabilityFlagUsage(flag)) {
					t.Errorf("help does not project --%s usage: %s", flag.Name, line)
				}
				if flag.Default != "" && !strings.Contains(line, "(default "+flag.Default+")") {
					t.Errorf("help does not project --%s default: %s", flag.Name, line)
				}
			}
			if safety := expectedSafetyHelp(capability.ProductGate); safety != "" && !strings.Contains(stdout, safety) {
				t.Errorf("help missing safety boundary %q", safety)
			}
		})
	}
}

func helpFlagLine(help string, flag catalog.Flag) string {
	prefix := "--" + flag.Name
	for _, line := range strings.Split(help, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) && strings.Contains(line, flag.Usage) {
			return line
		}
	}
	return ""
}

func expectedSafetyHelp(gate catalog.ProductGate) string {
	if gate == catalog.GateNone {
		return ""
	}
	return "pass --yes to confirm this invocation before network I/O"
}
