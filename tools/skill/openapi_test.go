package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTrackedSnapshotIntegrityAndLocaleReport(t *testing.T) {
	directory := filepath.Join("..", "..", defaultSkillPath, filepath.FromSlash(openAPISnapshotSuffix))
	if err := validateSnapshot(directory); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest snapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if got, want := len(manifest.Files), 56; got != want {
		t.Fatalf("manifest payload file count = %d, want %d", got, want)
	}
	if got, want := manifest.LocaleComparison.OperationCount, 33; got != want {
		t.Fatalf("locale operation count = %d, want %d", got, want)
	}
	wantDifferences := []string{
		"components.schemas.aqiBasicObjectV7",
		"components.schemas.aqisArray",
		"components.schemas.colorObject",
		"components.schemas.getAlert",
		"components.schemas.getAqiNowV7",
		"components.schemas.getMoon",
		"components.schemas.pollutantSubindexArray",
	}
	if !reflect.DeepEqual(manifest.LocaleComparison.SchemaDifferences, wantDifferences) {
		t.Fatalf("schema differences = %v, want %v", manifest.LocaleComparison.SchemaDifferences, wantDifferences)
	}
}

func TestOpenAPIRejectsRemoteReferences(t *testing.T) {
	remoteSchema := strings.Replace(minimalOpenAPI("string"), "#/components/schemas/X", "https://example.com/schema.yml", 1)
	if _, err := parseOpenAPI("test", []byte(remoteSchema)); err == nil || !strings.Contains(err.Error(), "remote or malformed schema") {
		t.Fatalf("remote schema error = %v", err)
	}
	remoteExample := strings.Replace(minimalOpenAPI("string"), "schema:\n                $ref", "examples:\n                response:\n                  externalValue: https://example.com/example.json\n              schema:\n                $ref", 1)
	if _, err := parseOpenAPI("test", []byte(remoteExample)); err == nil || !strings.Contains(err.Error(), "remote or malformed example") {
		t.Fatalf("remote example error = %v", err)
	}
}

func TestLocaleComparisonReportsButDoesNotMergeSchemaDifferences(t *testing.T) {
	english, err := parseOpenAPI("English", []byte(minimalOpenAPI("string")))
	if err != nil {
		t.Fatal(err)
	}
	chinese, err := parseOpenAPI("Chinese", []byte(minimalOpenAPI("integer")))
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := compareLocales(english, chinese)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(comparison.SchemaDifferences, []string{"components.schemas.X"}) {
		t.Fatalf("schema differences = %v", comparison.SchemaDifferences)
	}
}

func TestLocaleComparisonRejectsContractInventoryDrift(t *testing.T) {
	english, err := parseOpenAPI("English", []byte(minimalOpenAPI("string")))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(minimalOpenAPI("string"), "name: query", "name: changed", 1)
	chinese, err := parseOpenAPI("Chinese", []byte(changed))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compareLocales(english, chinese); err == nil || !strings.Contains(err.Error(), "parameter sets differ") {
		t.Fatalf("parameter drift error = %v", err)
	}
}

func TestManifestEncodingIsDeterministic(t *testing.T) {
	manifest := snapshotManifest{
		Schema:     manifestSchema,
		Repository: upstreamRepository,
		Commit:     pinnedCommit,
		Files:      []manifestFile{{Path: "NOTICE.md", Bytes: 4, SHA256: strings.Repeat("a", 64)}},
		LocaleComparison: localeComparison{
			OperationCount:    1,
			SchemaDifferences: []string{},
		},
	}
	first, err := marshalManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := marshalManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || first[len(first)-1] != '\n' {
		t.Fatal("manifest encoding is not deterministic LF-terminated JSON")
	}
}

func minimalOpenAPI(schemaType string) string {
	return `openapi: 3.0.0
paths:
  /x:
    get:
      operationId: getX
      parameters:
        - name: query
          in: query
          required: true
          schema:
            type: string
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/X'
components:
  schemas:
    X:
      type: object
      properties:
        value:
          type: ` + schemaType + "\n"
}
