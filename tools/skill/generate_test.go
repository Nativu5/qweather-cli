package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
)

func TestGeneratedReferencesCoverRegistryAndProblemCatalog(t *testing.T) {
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	commands := buildCommandReference("1.2.3", registry)
	if got, want := bytes.Count(commands, []byte("### `qweather ")), len(registry.Current()); got != want {
		t.Fatalf("generated command count = %d, want %d", got, want)
	}
	for _, capability := range registry.All() {
		if !bytes.Contains(commands, []byte("`"+capability.ID+"`")) {
			t.Errorf("generated command reference omits %s", capability.ID)
		}
		if !bytes.Contains(commands, []byte("<"+capability.DocsURL+">")) {
			t.Errorf("generated command reference omits official documentation for %s", capability.ID)
		}
	}
	if got, want := len(registry.Current()), 28; got != want {
		t.Fatalf("Current Capability count = %d, want %d", got, want)
	}
	if got, want := len(registry.Deprecated()), 5; got != want {
		t.Fatalf("Tombstone count = %d, want %d", got, want)
	}

	schema, err := buildResultSchemaReference("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range output.ProblemDefinitions() {
		if !bytes.Contains(schema, []byte("`"+definition.Code+"`")) {
			t.Errorf("generated problem reference omits %s", definition.Code)
		}
	}
	for _, field := range []string{"`resolvedPlace.id`", "`cache.upstreamRequested`", "`upstream.responseFamily`", "`details`"} {
		if !bytes.Contains(schema, []byte(field)) {
			t.Errorf("generated schema reference omits %s", field)
		}
	}
}

func TestCollectSchemaFieldsReportsMissingDescription(t *testing.T) {
	type undocumented struct {
		Value string `json:"value"`
	}
	if _, err := collectSchemaFields(reflect.TypeFor[undocumented](), "", true, nil); err == nil || !strings.Contains(err.Error(), "value") {
		t.Fatalf("missing description error = %v", err)
	}
}

func TestGenerateIsIdempotentAndSynchronizesVersion(t *testing.T) {
	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "VERSION"), "2.3.4\n")
	mustWriteTestFile(t, filepath.Join(root, "packages", "npm", "package.json"), "{\"version\":\"2.3.4\"}\n")
	mustWriteTestFile(t, filepath.Join(root, defaultSkillPath, "SKILL.md"), "Install with `npm install --global qweather-cli@0.0.1`.\n")

	if err := writeGeneratedReferences(root); err != nil {
		t.Fatal(err)
	}
	first := readGeneratedTestFiles(t, root)
	if err := writeGeneratedReferences(root); err != nil {
		t.Fatal(err)
	}
	second := readGeneratedTestFiles(t, root)
	if !strings.Contains(second["SKILL.md"], "qweather-cli@2.3.4") {
		t.Fatalf("Skill install command was not synchronized: %q", second["SKILL.md"])
	}
	for name, contents := range first {
		if second[name] != contents {
			t.Errorf("second generation changed %s", name)
		}
	}
	if err := checkGeneratedReferences(root); err != nil {
		t.Fatal(err)
	}
	if err := checkVersionSync(root); err != nil {
		t.Fatal(err)
	}
}

func mustWriteTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readGeneratedTestFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	paths := map[string]string{
		"SKILL.md":             filepath.Join(root, defaultSkillPath, "SKILL.md"),
		"command-reference.md": filepath.Join(root, defaultSkillPath, "references", "command-reference.md"),
		"result-schema.md":     filepath.Join(root, defaultSkillPath, "references", "result-schema.md"),
	}
	result := make(map[string]string, len(paths))
	for name, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result[name] = string(data)
	}
	return result
}
