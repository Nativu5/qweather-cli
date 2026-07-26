package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSkillFrontmatter(t *testing.T) {
	valid := "---\nname: qweather\ndescription: Safe QWeather guidance.\n---\n\n# QWeather\n"
	if err := validateSkillFrontmatter(valid); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "missing", content: "# QWeather\n", want: "must begin"},
		{name: "unknown field", content: strings.Replace(valid, "description:", "extra: value\ndescription:", 1), want: "unexpected field"},
		{name: "wrong name", content: strings.Replace(valid, "name: qweather", "name: other", 1), want: "want qweather"},
		{name: "empty description", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: ", 1), want: "non-empty plain scalar"},
		{name: "quoted empty description", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: ''", 1), want: "non-empty plain scalar"},
		{name: "unterminated flow value", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: [Safe QWeather guidance.", 1), want: "non-empty plain scalar"},
		{name: "null description", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: null", 1), want: "non-empty plain scalar"},
		{name: "tilde description", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: ~", 1), want: "non-empty plain scalar"},
		{name: "boolean description", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: true", 1), want: "non-empty plain scalar"},
		{name: "trailing colon", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: guidance:", 1), want: "non-empty plain scalar"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSkillFrontmatter(test.content)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestCheckSkillStructureRejectsUnexpectedReferenceFile(t *testing.T) {
	root := scaffoldSkillStructure(t)
	unexpected := filepath.Join(root, defaultSkillPath, "references", "snapshot.yml")
	if err := os.WriteFile(unexpected, []byte("snapshot: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := checkSkillStructure(root)
	if err == nil || !strings.Contains(err.Error(), "curated one-level reference set") {
		t.Fatalf("error = %v, want unexpected reference set error", err)
	}
}

func TestCheckSkillStructureRejectsReferenceSymlink(t *testing.T) {
	root := scaffoldSkillStructure(t)
	referencesRoot := filepath.Join(root, defaultSkillPath, "references")
	link := filepath.Join(referencesRoot, curatedReferenceNames[0])
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(curatedReferenceNames[1], link); err != nil {
		t.Fatal(err)
	}

	err := checkSkillStructure(root)
	if err == nil || !strings.Contains(err.Error(), "must be a regular file") {
		t.Fatalf("error = %v, want non-regular reference error", err)
	}
}

func scaffoldSkillStructure(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	skillRoot := filepath.Join(root, defaultSkillPath)
	referencesRoot := filepath.Join(skillRoot, "references")
	if err := os.MkdirAll(referencesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skillRoot, "agents"), 0o700); err != nil {
		t.Fatal(err)
	}

	var routes strings.Builder
	for _, name := range curatedReferenceNames {
		routes.WriteString("references/" + name + "\n")
		if err := os.WriteFile(filepath.Join(referencesRoot, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	skill := "---\nname: qweather\ndescription: Safe QWeather guidance.\n---\n" + routes.String()
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(skill), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "agents", "openai.yaml"), []byte("prompt: $qweather\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}
