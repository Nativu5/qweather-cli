package catalog

import (
	"strings"
	"testing"
)

func TestDefaultRegistryCoverage(t *testing.T) {
	registry, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}
	if got, want := len(registry.Current()), 28; got != want {
		t.Fatalf("current capability count = %d, want %d", got, want)
	}
	if got, want := len(registry.Deprecated()), 5; got != want {
		t.Fatalf("Tombstone count = %d, want %d", got, want)
	}

	expectedPaths := []string{
		"geo city lookup", "geo city top", "geo poi lookup", "geo poi nearby",
		"weather city current", "weather city daily", "weather city hourly",
		"weather grid current", "weather grid daily", "weather grid hourly",
		"weather minutely", "weather indices", "weather history",
		"alert current", "air current", "air daily", "air hourly", "air station",
		"storm list", "storm track", "storm forecast", "marine tide", "solar forecast",
		"astronomy sun", "astronomy moon", "astronomy position",
		"account finance", "account usage",
	}
	seen := make(map[string]bool, len(expectedPaths))
	for _, capability := range registry.Current() {
		seen[capability.CommandPath] = true
		if capability.Cache.Mode == "" || capability.Cache.Evidence == "" {
			t.Errorf("%s has incomplete cache policy", capability.ID)
		}
	}
	for _, path := range expectedPaths {
		if !seen[path] {
			t.Errorf("missing command path %q", path)
		}
	}
	for _, capability := range registry.Deprecated() {
		if capability.CommandPath != "" {
			t.Errorf("Tombstone %s has executable path %q", capability.ID, capability.CommandPath)
		}
	}
}

func TestRegistryHashIsDeterministic(t *testing.T) {
	first, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(first.All())
	if err != nil {
		t.Fatal(err)
	}
	firstHash, err := first.Hash()
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := second.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if firstHash != secondHash {
		t.Fatalf("hashes differ: %s != %s", firstHash, secondHash)
	}
}

func TestValidateRejectsBrokenRecords(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func([]Capability)
		message string
	}{
		{
			name: "duplicate ID",
			mutate: func(items []Capability) {
				items[1].ID = items[0].ID
			},
			message: "duplicate capability ID",
		},
		{
			name: "duplicate path",
			mutate: func(items []Capability) {
				items[1].CommandPath = items[0].CommandPath
			},
			message: "duplicate command path",
		},
		{
			name: "bad docs URL",
			mutate: func(items []Capability) {
				items[0].DocsURL = "http://example.com"
			},
			message: "official HTTPS documentation URL",
		},
		{
			name: "missing transport",
			mutate: func(items []Capability) {
				items[0].Upstream.PathTemplate = ""
			},
			message: "complete GET transport mapping",
		},
		{
			name: "missing cache policy",
			mutate: func(items []Capability) {
				items[0].Cache.Evidence = ""
			},
			message: "explicit cache policy",
		},
		{
			name: "executable Tombstone",
			mutate: func(items []Capability) {
				items[len(items)-1].CommandPath = "legacy execute"
			},
			message: "Tombstone cannot have",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := records()
			test.mutate(items)
			err := Validate(items)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.message)
			}
		})
	}
}
