package output

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"text/template"
)

//go:embed templates
var templateFiles embed.FS

// RenderInfo reports presentation decisions that are useful only for optional
// diagnostics and tests.
type RenderInfo struct {
	Fallback bool
}

// Renderer owns the compiled Capability entry templates.
type Renderer struct {
	templates map[string]*template.Template
}

// NewRenderer compiles every embedded entry template and verifies that each
// requested Current Capability has exactly one template.
func NewRenderer(capabilityIDs []string) (*Renderer, error) {
	renderer, err := loadRenderer()
	if err != nil {
		return nil, err
	}
	requested := append([]string(nil), capabilityIDs...)
	sort.Strings(requested)
	expected := make(map[string]struct{}, len(requested))
	for index, capabilityID := range requested {
		if capabilityID == "" {
			return nil, fmt.Errorf("Current Capability ID is empty")
		}
		if index > 0 && capabilityID == requested[index-1] {
			return nil, fmt.Errorf("duplicate Current Capability ID %s", capabilityID)
		}
		expected[capabilityID] = struct{}{}
		if _, exists := renderer.templates[capabilityID]; !exists {
			return nil, fmt.Errorf("missing output template for Current Capability %s", capabilityID)
		}
	}
	for capabilityID := range renderer.templates {
		if _, exists := expected[capabilityID]; !exists {
			return nil, fmt.Errorf("output template %s does not name a Current Capability", capabilityID)
		}
	}
	return renderer, nil
}

func loadRenderer() (*Renderer, error) {
	entries, err := fs.ReadDir(templateFiles, "templates")
	if err != nil {
		return nil, fmt.Errorf("read embedded output templates: %w", err)
	}
	templates := make(map[string]*template.Template)
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".tmpl" {
			continue
		}
		capabilityID := strings.TrimSuffix(entry.Name(), ".tmpl")
		if capabilityID == "" {
			return nil, fmt.Errorf("output template has an empty Capability ID")
		}
		if _, exists := templates[capabilityID]; exists {
			return nil, fmt.Errorf("duplicate output template for %s", capabilityID)
		}
		contents, readErr := templateFiles.ReadFile("templates/" + entry.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read output template for %s: %w", capabilityID, readErr)
		}
		compiled, parseErr := template.New(capabilityID).Funcs(parseFuncMap()).Option("missingkey=error").Parse(string(contents))
		if parseErr != nil {
			return nil, fmt.Errorf("parse output template for %s: %w", capabilityID, parseErr)
		}
		templates[capabilityID] = compiled
	}

	return &Renderer{templates: templates}, nil
}

// TemplateIDs returns the compiled entry-template inventory.
func (r *Renderer) TemplateIDs() []string {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.templates))
	for capabilityID := range r.templates {
		ids = append(ids, capabilityID)
	}
	sort.Strings(ids)
	return ids
}

func (r *Renderer) RenderResult(writer io.Writer, result *Result, mode Mode) (RenderInfo, error) {
	if result == nil {
		return RenderInfo{}, fmt.Errorf("result is nil")
	}
	switch mode {
	case ModeJSON:
		return RenderInfo{}, WriteJSON(writer, result)
	case ModeBody:
		_, err := writer.Write(result.ProviderBody)
		return RenderInfo{}, err
	case ModeText:
		if r == nil {
			return RenderInfo{}, fmt.Errorf("output renderer is nil")
		}
		return r.renderTextResult(writer, result)
	default:
		return RenderInfo{}, fmt.Errorf("output mode %q is invalid", mode)
	}
}

func (r *Renderer) renderEntry(state *templateState, capabilityID string) (string, error) {
	entry, exists := r.templates[capabilityID]
	if !exists {
		return "", fmt.Errorf("missing output template for Current Capability %s", capabilityID)
	}
	instance, err := entry.Clone()
	if err != nil {
		return "", fmt.Errorf("clone output template for %s: %w", capabilityID, err)
	}
	instance = instance.Funcs(state.funcMap())
	var buffer bytes.Buffer
	if err := instance.Execute(&buffer, nil); err != nil {
		return "", fmt.Errorf("execute output template for %s: %w", capabilityID, err)
	}
	return buffer.String(), nil
}
