package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

type templateState struct {
	data       map[string]any
	consumed   map[string]struct{}
	mismatch   bool
	unitSystem string
}

func newTemplateState(data map[string]any, unitSystem string) *templateState {
	return &templateState{data: data, consumed: make(map[string]struct{}), unitSystem: unitSystem}
}

func parseFuncMap() template.FuncMap {
	return template.FuncMap{
		"field":          func(string, string, string) string { return "" },
		"requiredField":  func(string, string, string) string { return "" },
		"section":        func(string) string { return "" },
		"items":          func(string) []int { return nil },
		"optionalItems":  func(string) []int { return nil },
		"optionalObject": func(string) bool { return false },
		"has":            func(string) bool { return false },
		"path":           buildPath,
		"inc":            func(value int) int { return value + 1 },
		"unit":           func(string) string { return "" },
	}
}

func (s *templateState) funcMap() template.FuncMap {
	return template.FuncMap{
		"field": func(label, fieldPath, unit string) string {
			return s.field(label, fieldPath, unit, false)
		},
		"requiredField": func(label, fieldPath, unit string) string {
			return s.field(label, fieldPath, unit, true)
		},
		"section": func(label string) string {
			if label == "" {
				return ""
			}
			return label + ":\n"
		},
		"items": func(fieldPath string) []int {
			return s.itemIndexes(fieldPath, true)
		},
		"optionalItems": func(fieldPath string) []int {
			return s.itemIndexes(fieldPath, false)
		},
		"optionalObject": func(fieldPath string) bool {
			value, exists, malformed := s.lookupStatus(fieldPath)
			if malformed {
				s.mismatch = true
				return false
			}
			if !exists || value == nil {
				return false
			}
			if _, ok := value.(map[string]any); !ok {
				s.mismatch = true
				return false
			}
			return true
		},
		"has": func(fieldPath string) bool {
			_, exists := s.lookup(fieldPath)
			return exists
		},
		"path": buildPath,
		"inc":  func(value int) int { return value + 1 },
		"unit": s.unit,
	}
}

func (s *templateState) field(label, fieldPath, unit string, required bool) string {
	value, exists, malformed := s.lookupStatus(fieldPath)
	if malformed {
		s.mismatch = true
		return ""
	}
	if !exists {
		if required {
			s.mismatch = true
		}
		return ""
	}
	switch value.(type) {
	case map[string]any, []any:
		s.mismatch = true
		return ""
	}
	s.consumed[fieldPath] = struct{}{}
	rendered := scalarText(value)
	if unit != "" && value != nil && rendered != "" {
		rendered += " " + unit
	}
	return "  " + label + ": " + rendered + "\n"
}

func (s *templateState) itemIndexes(fieldPath string, required bool) []int {
	value, exists, malformed := s.lookupStatus(fieldPath)
	if malformed {
		s.mismatch = true
		return nil
	}
	if !exists {
		if required {
			s.mismatch = true
		}
		return nil
	}
	if value == nil && !required {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		s.mismatch = true
		return nil
	}
	indexes := make([]int, len(items))
	for index := range items {
		indexes[index] = index
	}
	return indexes
}

func (s *templateState) lookup(fieldPath string) (any, bool) {
	value, exists, _ := s.lookupStatus(fieldPath)
	return value, exists
}

func (s *templateState) lookupStatus(fieldPath string) (any, bool, bool) {
	if fieldPath == "" {
		return s.data, true, false
	}
	var current any = s.data
	for _, segment := range strings.Split(fieldPath, ".") {
		switch value := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = value[segment]
			if !exists {
				return nil, false, false
			}
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false, true
			}
			current = value[index]
		default:
			if current == nil {
				return nil, false, false
			}
			return nil, false, true
		}
	}
	return current, true, false
}

func (s *templateState) unit(kind string) string {
	metric := s.unitSystem == "metric"
	imperial := s.unitSystem == "imperial"
	switch kind {
	case "temperature":
		if imperial {
			return "°F"
		}
		if metric {
			return "°C"
		}
	case "speed":
		if imperial {
			return "mph"
		}
		if metric {
			return "km/h"
		}
	case "precipitation":
		if imperial {
			return "in"
		}
		if metric {
			return "mm"
		}
	case "visibility":
		if imperial {
			return "mi"
		}
		if metric {
			return "km"
		}
	case "pressure":
		return "hPa"
	case "percent":
		return "%"
	case "angle":
		return "°"
	case "distance":
		return "km"
	case "solar-radiation":
		return "W/m²"
	case "energy":
		return "Wh/m²"
	case "altitude":
		return "m"
	default:
	}
	return ""
}

func (s *templateState) consumeAttributionPaths() {
	for _, fieldPath := range []string{"refer.sources", "refer.license", "metadata.attributions"} {
		value, exists, malformed := s.lookupStatus(fieldPath)
		if malformed {
			s.mismatch = true
			continue
		}
		if !exists {
			continue
		}
		if value == nil {
			s.consumed[fieldPath] = struct{}{}
			continue
		}
		if _, ok := value.([]any); !ok {
			s.mismatch = true
			continue
		}
		s.consumed[fieldPath] = struct{}{}
	}
}

func buildPath(parts ...any) string {
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segments = append(segments, fmt.Sprint(part))
	}
	return strings.Join(segments, ".")
}

func (r *Renderer) renderTextResult(writer io.Writer, result *Result) (RenderInfo, error) {
	data, err := decodeData(result.Data)
	if err != nil {
		return RenderInfo{}, err
	}
	state := newTemplateState(data, result.Unit)
	state.consumeAttributionPaths()
	attributionConsumed := make(map[string]struct{}, len(state.consumed))
	for fieldPath := range state.consumed {
		attributionConsumed[fieldPath] = struct{}{}
	}

	var body strings.Builder
	writeResultContext(&body, result)
	if result.Outcome == "no_data" {
		body.WriteString("No Data: provider returned no matching records\n")
	} else {
		entry, renderErr := r.renderEntry(state, result.Capability)
		if renderErr != nil {
			return RenderInfo{}, renderErr
		}
		if state.mismatch {
			body.WriteString(renderNamed("Provider data", data, "", 0, attributionConsumed, false))
			writeAttribution(&body, result.Attribution)
			_, writeErr := io.WriteString(writer, body.String())
			return RenderInfo{Fallback: true}, writeErr
		}
		body.WriteString(entry)
	}

	remainder := renderMapChildren(data, "", 2, state.consumed)
	if remainder != "" {
		body.WriteString("Additional fields:\n")
		body.WriteString(remainder)
	}
	writeAttribution(&body, result.Attribution)
	_, err = io.WriteString(writer, body.String())
	return RenderInfo{}, err
}

func writeResultContext(builder *strings.Builder, result *Result) {
	fmt.Fprintf(builder, "Capability: %s\n", result.Capability)
	if result.ResolvedPlace != nil {
		builder.WriteString("Resolved Place:\n")
		placeFields := [][2]string{
			{"ID", result.ResolvedPlace.ID}, {"Name", result.ResolvedPlace.Name},
			{"Administrative area 1", result.ResolvedPlace.Adm1}, {"Administrative area 2", result.ResolvedPlace.Adm2},
			{"Country", result.ResolvedPlace.Country}, {"Latitude", result.ResolvedPlace.Lat},
			{"Longitude", result.ResolvedPlace.Lon}, {"Time zone", result.ResolvedPlace.TZ},
		}
		for _, field := range placeFields {
			if field[1] != "" {
				fmt.Fprintf(builder, "  %s: %s\n", field[0], field[1])
			}
		}
	}
	builder.WriteString("Cache:\n")
	fmt.Fprintf(builder, "  Status: %s\n", result.Cache.Status)
	if result.Cache.StoredAt != "" {
		fmt.Fprintf(builder, "  Stored at: %s\n", result.Cache.StoredAt)
	}
	if result.Cache.ExpiresAt != "" {
		fmt.Fprintf(builder, "  Expires at: %s\n", result.Cache.ExpiresAt)
	}
	if result.Cache.AgeSeconds != 0 {
		fmt.Fprintf(builder, "  Age seconds: %d\n", result.Cache.AgeSeconds)
	}
	fmt.Fprintf(builder, "  Upstream requested: %t\n", result.Cache.UpstreamRequested)
	builder.WriteString("Operations:\n")
	if len(result.Operations) == 0 {
		builder.WriteString("  none\n")
	} else {
		for _, operation := range result.Operations {
			fmt.Fprintf(builder, "  - %s\n", operation)
		}
	}
}

func writeAttribution(builder *strings.Builder, attribution []any) {
	if len(attribution) == 0 {
		builder.WriteString("Attribution: none\n")
		return
	}
	builder.WriteString(renderNamed("Attribution", attribution, "", 0, nil, false))
}

func writeTextProblem(writer io.Writer, problem *Problem) error {
	var builder strings.Builder
	builder.WriteString(problem.Message)
	builder.WriteByte('\n')
	fmt.Fprintf(&builder, "Code: %s\n", problem.Code)
	if problem.Capability != "" {
		fmt.Fprintf(&builder, "Capability: %s\n", problem.Capability)
	}
	fmt.Fprintf(&builder, "Retryable: %t\n", problem.Retryable)
	if problem.Details != nil {
		normalized, err := normalizeValue(problem.Details)
		if err != nil {
			return fmt.Errorf("normalize problem details: %w", err)
		}
		builder.WriteString(renderNamed("Details", normalized, "", 0, nil, false))
	}
	_, err := io.WriteString(writer, builder.String())
	return err
}

// WriteValueText renders local-control data with deterministic object ordering.
func WriteValueText(writer io.Writer, value any) error {
	normalized, err := normalizeValue(value)
	if err != nil {
		return err
	}
	var rendered string
	if object, ok := normalized.(map[string]any); ok {
		rendered = renderMapChildren(object, "", 0, nil)
	} else {
		rendered = renderNamed("Value", normalized, "", 0, nil, false)
	}
	_, err = io.WriteString(writer, rendered)
	return err
}

func normalizeValue(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode value for Text output: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, fmt.Errorf("decode value for Text output: %w", err)
	}
	return normalized, nil
}

func decodeData(data json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("decode Result data for Text output: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("Result data is not a JSON object")
	}
	return object, nil
}

func renderMapChildren(object map[string]any, parentPath string, indent int, consumed map[string]struct{}) string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		fieldPath := key
		if parentPath != "" {
			fieldPath = parentPath + "." + key
		}
		builder.WriteString(renderNamed(key, object[key], fieldPath, indent, consumed, true))
	}
	return builder.String()
}

func renderNamed(name string, value any, fieldPath string, indent int, consumed map[string]struct{}, omitFullyConsumed bool) string {
	if pathConsumed(fieldPath, consumed) {
		return ""
	}
	prefix := strings.Repeat(" ", indent) + name + ":"
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return prefix + " {}\n"
		}
		children := renderMapChildren(typed, fieldPath, indent+2, consumed)
		if children == "" && omitFullyConsumed {
			return ""
		}
		return prefix + "\n" + children
	case []any:
		if len(typed) == 0 {
			return prefix + " []\n"
		}
		var children strings.Builder
		for index, item := range typed {
			itemPath := strconv.Itoa(index)
			if fieldPath != "" {
				itemPath = fieldPath + "." + itemPath
			}
			children.WriteString(renderNamed(fmt.Sprintf("[%d]", index), item, itemPath, indent+2, consumed, true))
		}
		if children.Len() == 0 && omitFullyConsumed {
			return ""
		}
		return prefix + "\n" + children.String()
	default:
		return prefix + " " + scalarText(value) + "\n"
	}
}

func pathConsumed(fieldPath string, consumed map[string]struct{}) bool {
	if fieldPath == "" || len(consumed) == 0 {
		return false
	}
	for candidate := fieldPath; candidate != ""; {
		if _, exists := consumed[candidate]; exists {
			return true
		}
		separator := strings.LastIndexByte(candidate, '.')
		if separator < 0 {
			break
		}
		candidate = candidate[:separator]
	}
	return false
}

func scalarText(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	default:
		return fmt.Sprint(typed)
	}
}
