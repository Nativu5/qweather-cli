package output

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/Nativu5/qweather-cli/internal/catalog"
)

func testRenderer(t *testing.T, capabilityID, source string) *Renderer {
	t.Helper()
	entry, err := template.New(capabilityID).Funcs(parseFuncMap()).Option("missingkey=error").Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	return &Renderer{templates: map[string]*template.Template{capabilityID: entry}}
}

func renderOfficialTemplate(t *testing.T, capabilityID, fixture string) string {
	t.Helper()
	body, err := os.ReadFile("testdata/official/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := loadRenderer()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := renderer.templates[capabilityID]; !exists {
		t.Fatalf("missing output template for %s", capabilityID)
	}
	result := testResult(capabilityID, string(body))
	result.ProviderBody = body
	result.Attribution = fixtureAttribution(t, body)
	var output bytes.Buffer
	info, err := renderer.RenderResult(&output, result, ModeText)
	if err != nil {
		t.Fatal(err)
	}
	if info.Fallback {
		t.Fatalf("%s unexpectedly used generic fallback", capabilityID)
	}
	text := output.String()
	if strings.Contains(text, "<no value>") {
		t.Fatalf("%s rendered <no value>: %s", capabilityID, text)
	}
	if strings.Count(text, "Attribution:") != 1 {
		t.Fatalf("%s rendered Attribution %d times: %s", capabilityID, strings.Count(text, "Attribution:"), text)
	}
	return text
}

func fixtureAttribution(t *testing.T, body []byte) []any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatal(err)
	}
	var attribution []any
	if metadata, ok := object["metadata"].(map[string]any); ok {
		if values, ok := metadata["attributions"].([]any); ok {
			attribution = append(attribution, values...)
		}
	}
	if refer, ok := object["refer"].(map[string]any); ok {
		if values, ok := refer["sources"].([]any); ok {
			for _, value := range values {
				attribution = append(attribution, map[string]any{"source": value})
			}
		}
		if values, ok := refer["license"].([]any); ok {
			for _, value := range values {
				attribution = append(attribution, map[string]any{"license": value})
			}
		}
	}
	return attribution
}

func testResult(capabilityID, data string) *Result {
	return &Result{
		Schema:       ResultSchema,
		Outcome:      "ok",
		Capability:   capabilityID,
		Operations:   []string{capabilityID},
		Policy:       Policy{BillingGroup: "basic"},
		Cache:        Cache{Status: "miss", UpstreamRequested: true},
		Upstream:     Upstream{HTTPStatus: 200, ResponseFamily: "code-refer-v1"},
		Attribution:  []any{map[string]any{"source": "QWeather"}},
		Data:         json.RawMessage(data),
		ProviderBody: []byte(data),
		Unit:         "metric",
	}
}

func TestRenderResultModes(t *testing.T) {
	renderer := testRenderer(t, "weather.city.current", `{{- section "Current" }}{{- requiredField "Temperature" "now.temp" (unit "temperature") }}`)
	result := testResult("weather.city.current", `{"code":"200","now":{"temp":"20","future":"kept"},"refer":{"sources":["QWeather"]}}`)

	var textOutput bytes.Buffer
	info, err := renderer.RenderResult(&textOutput, result, ModeText)
	if err != nil || info.Fallback {
		t.Fatalf("info=%#v err=%v", info, err)
	}
	text := textOutput.String()
	for _, expected := range []string{
		"Capability: weather.city.current\n",
		"Current:\n  Temperature: 20 °C\n",
		"Additional fields:\n  code: 200\n  now:\n    future: kept\n",
		"Attribution:\n  [0]:\n    source: QWeather\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "refer:") || strings.Count(text, "QWeather") != 1 {
		t.Fatalf("Attribution was duplicated: %s", text)
	}

	var jsonOutput bytes.Buffer
	if _, err := renderer.RenderResult(&jsonOutput, result, ModeJSON); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonOutput.String(), "\n  ") || !strings.HasSuffix(jsonOutput.String(), "\n") || !strings.Contains(jsonOutput.String(), `"schema":"qweather.result/v1"`) {
		t.Fatalf("JSON output is not compact: %q", jsonOutput.String())
	}

	var bodyOutput bytes.Buffer
	if _, err := renderer.RenderResult(&bodyOutput, result, ModeBody); err != nil {
		t.Fatal(err)
	}
	if bodyOutput.String() != string(result.ProviderBody) {
		t.Fatalf("body = %q, want exact %q", bodyOutput.String(), result.ProviderBody)
	}
}

func TestTemplateDoesNotGuessAnUnknownEffectiveUnit(t *testing.T) {
	renderer := testRenderer(t, "weather.city.current", `{{- requiredField "Temperature" "now.temp" (unit "temperature") }}`)
	result := testResult("weather.city.current", `{"now":{"temp":"20"}}`)
	result.Unit = ""
	var output bytes.Buffer
	if _, err := renderer.RenderResult(&output, result, ModeText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Temperature: 20\n") || strings.Contains(output.String(), "20 °") {
		t.Fatalf("unknown unit was guessed: %s", output.String())
	}
}

func TestTextShapeMismatchFallsBackWithCompleteProviderData(t *testing.T) {
	renderer := testRenderer(t, "weather.city.current", `{{- field "Provider code" "code" "" }}{{- requiredField "Temperature" "now.temp" (unit "temperature") }}`)
	result := testResult("weather.city.current", `{"code":"200","unexpected":{"ordered":[1,2,3]}}`)
	var output bytes.Buffer
	info, err := renderer.RenderResult(&output, result, ModeText)
	if err != nil || !info.Fallback {
		t.Fatalf("info=%#v err=%v", info, err)
	}
	text := output.String()
	for _, expected := range []string{"Provider data:\n", "code: 200\n", "ordered:\n", "[0]: 1\n", "[1]: 2\n", "[2]: 3\n"} {
		if !strings.Contains(text, expected) {
			t.Errorf("fallback missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "Provider code:") {
		t.Fatalf("partial entry layout leaked into fallback: %s", text)
	}
}

func TestMalformedAttributionShapeFallsBackWithoutLosingProviderData(t *testing.T) {
	renderer := testRenderer(t, "weather.city.current", `{{- requiredField "Temperature" "now.temp" (unit "temperature") }}`)
	result := testResult("weather.city.current", `{"now":{"temp":"20"},"refer":{"sources":"unexpected scalar"}}`)
	result.Attribution = nil
	var output bytes.Buffer
	info, err := renderer.RenderResult(&output, result, ModeText)
	if err != nil || !info.Fallback {
		t.Fatalf("info=%#v err=%v", info, err)
	}
	text := output.String()
	for _, expected := range []string{"Provider data:\n", "sources: unexpected scalar\n", "Attribution: none\n"} {
		if !strings.Contains(text, expected) {
			t.Errorf("fallback missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "Temperature:") {
		t.Fatalf("partial entry layout leaked into fallback: %s", text)
	}
}

func TestNullableAttributionPathsDoNotTriggerFallback(t *testing.T) {
	renderer := testRenderer(t, "weather.city.current", `{{- requiredField "Temperature" "now.temp" (unit "temperature") }}`)
	result := testResult("weather.city.current", `{"now":{"temp":"20"},"refer":{"sources":null,"license":null}}`)
	result.Attribution = nil
	var output bytes.Buffer
	info, err := renderer.RenderResult(&output, result, ModeText)
	if err != nil || info.Fallback {
		t.Fatalf("info=%#v err=%v output=%q", info, err, output.String())
	}
	text := output.String()
	if !strings.Contains(text, "Temperature: 20 °C\n") || !strings.Contains(text, "Attribution: none\n") || strings.Contains(text, "refer:") {
		t.Fatalf("nullable Attribution paths were not handled as empty: %s", text)
	}
}

func TestOptionalItemsAcceptDocumentedNullWithoutFallback(t *testing.T) {
	renderer := testRenderer(t, "air.current", `{{- requiredField "Index" "indexes.0.aqi" "" }}{{- range $index := optionalItems "indexes.0.subIndexes" }}{{- field "Sub-index" (path "indexes" 0 "subIndexes" $index "aqi") "" }}{{- end }}`)
	result := testResult("air.current", `{"indexes":[{"aqi":42,"subIndexes":null}]}`)
	var output bytes.Buffer
	info, err := renderer.RenderResult(&output, result, ModeText)
	if err != nil || info.Fallback {
		t.Fatalf("info=%#v err=%v output=%q", info, err, output.String())
	}
	if !strings.Contains(output.String(), "subIndexes: null\n") {
		t.Fatalf("documented null was not preserved: %s", output.String())
	}
}

func TestOptionalObjectDistinguishesNullFromMalformedShape(t *testing.T) {
	renderer := testRenderer(t, "storm.track", `{{- requiredField "Active" "isActive" "" }}{{- if optionalObject "now" }}{{- requiredField "Latitude" "now.lat" "" }}{{- end }}`)
	for _, test := range []struct {
		name     string
		data     string
		fallback bool
	}{
		{name: "nullable", data: `{"isActive":"0","now":null}`},
		{name: "malformed", data: `{"isActive":"1","now":"invalid"}`, fallback: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := testResult("storm.track", test.data)
			var output bytes.Buffer
			info, err := renderer.RenderResult(&output, result, ModeText)
			if err != nil || info.Fallback != test.fallback {
				t.Fatalf("info=%#v err=%v output=%q", info, err, output.String())
			}
		})
	}
}

func TestTextNoDataSkipsEntryTemplate(t *testing.T) {
	renderer := testRenderer(t, "weather.history", `{{- requiredField "Never" "missing" "" }}`)
	result := testResult("weather.history", `{"code":"204"}`)
	result.Outcome = "no_data"
	var output bytes.Buffer
	info, err := renderer.RenderResult(&output, result, ModeText)
	if err != nil || info.Fallback {
		t.Fatalf("info=%#v err=%v", info, err)
	}
	if !strings.Contains(output.String(), "No Data: provider returned no matching records\n") || !strings.Contains(output.String(), "code: 204\n") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestProblemPresentation(t *testing.T) {
	problem := NewProblem(3, "CONFIG_INVALID", "QWeather configuration is invalid")
	problem.Details = map[string]any{"z": []any{"last", "second"}, "a": "first"}

	var textOutput bytes.Buffer
	if err := RenderProblem(&textOutput, problem, ModeText); err != nil {
		t.Fatal(err)
	}
	text := textOutput.String()
	if !strings.HasPrefix(text, "QWeather configuration is invalid\nCode: CONFIG_INVALID\nRetryable: false\n") || strings.Index(text, "a: first") > strings.Index(text, "z:") || !strings.Contains(text, "[1]: second") {
		t.Fatalf("Text Problem = %q", text)
	}

	var bodyError bytes.Buffer
	if err := RenderProblem(&bodyError, problem, ModeBody); err != nil {
		t.Fatal(err)
	}
	if bodyError.String() != text {
		t.Fatalf("body-mode problem differs from Text: %q != %q", bodyError.String(), text)
	}

	var jsonOutput bytes.Buffer
	if err := RenderProblem(&jsonOutput, problem, ModeJSON); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOutput.String(), `"schema":"qweather.problem/v1"`) || strings.Contains(jsonOutput.String(), "\n  ") {
		t.Fatalf("Machine Problem = %q", jsonOutput.String())
	}
}

func TestNewRendererRejectsMissingCapabilityTemplate(t *testing.T) {
	if _, err := NewRenderer([]string{"missing.capability"}); err == nil || !strings.Contains(err.Error(), "missing output template") {
		t.Fatalf("error = %v", err)
	}
}

func TestEmbeddedTemplateInventoryMatchesCurrentCapabilities(t *testing.T) {
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	capabilityIDs := make([]string, 0, len(registry.Current()))
	for _, capability := range registry.Current() {
		capabilityIDs = append(capabilityIDs, capability.ID)
	}
	renderer, err := NewRenderer(capabilityIDs)
	if err != nil {
		t.Fatal(err)
	}
	if ids := renderer.TemplateIDs(); len(ids) != 28 {
		t.Fatalf("template IDs = %d, want 28: %v", len(ids), ids)
	}
}

func TestWriteValueTextSortsObjectsAndPreservesArrays(t *testing.T) {
	var output bytes.Buffer
	if err := WriteValueText(&output, map[string]any{"z": []int{3, 2, 1}, "a": "first"}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Index(text, "a: first") > strings.Index(text, "z:") || !strings.Contains(text, "[0]: 3\n") || !strings.Contains(text, "[2]: 1\n") {
		t.Fatalf("Text value = %q", text)
	}
}
