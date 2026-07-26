package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

var curatedReferenceNames = []string{
	"command-reference.md",
	"common-tasks.md",
	"places-and-errors.md",
	"products-and-attribution.md",
	"result-schema.md",
}

func checkSkill(root string) error {
	if err := checkVersionSync(root); err != nil {
		return err
	}
	if err := checkGeneratedReferences(root); err != nil {
		return err
	}
	if err := checkSkillStructure(root); err != nil {
		return err
	}
	return nil
}

func checkSkillStructure(root string) error {
	skillRoot := filepath.Join(root, defaultSkillPath)
	for _, required := range []string{"SKILL.md", filepath.Join("agents", "openai.yaml")} {
		status, err := os.Stat(filepath.Join(skillRoot, required))
		if err != nil || !status.Mode().IsRegular() {
			return fmt.Errorf("required Skill file %s is missing or not regular", required)
		}
	}
	referencesRoot := filepath.Join(skillRoot, "references")
	entries, err := os.ReadDir(referencesRoot)
	if err != nil {
		return fmt.Errorf("read Skill references: %w", err)
	}
	markdown := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("Skill references must be one-level files; found directory %s", entry.Name())
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			markdown = append(markdown, entry.Name())
		}
	}
	if !reflect.DeepEqual(markdown, curatedReferenceNames) {
		return fmt.Errorf("curated one-level reference set = %v, want %v", markdown, curatedReferenceNames)
	}
	skillBytes, err := os.ReadFile(filepath.Join(skillRoot, "SKILL.md"))
	if err != nil {
		return err
	}
	if err := validateSkillFrontmatter(string(skillBytes)); err != nil {
		return err
	}
	if lines := strings.Count(string(skillBytes), "\n") + 1; lines > 500 {
		return fmt.Errorf("SKILL.md has %d lines, must remain at or below 500", lines)
	}
	for _, link := range curatedReferenceNames {
		if strings.Count(string(skillBytes), "references/"+link) != 1 {
			return fmt.Errorf("SKILL.md must route exactly once to references/%s", link)
		}
	}
	openAIBytes, err := os.ReadFile(filepath.Join(skillRoot, "agents", "openai.yaml"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(openAIBytes), "$qweather") {
		return fmt.Errorf("agents/openai.yaml default prompt must explicitly mention $qweather")
	}
	return nil
}

type skillFrontmatter struct {
	Name        string
	Description string
}

func validateSkillFrontmatter(content string) error {
	if !strings.HasPrefix(content, "---\n") {
		return fmt.Errorf("SKILL.md must begin with YAML frontmatter")
	}
	frontmatterEnd := strings.Index(content[len("---\n"):], "\n---\n")
	if frontmatterEnd < 0 {
		return fmt.Errorf("SKILL.md YAML frontmatter is not closed")
	}
	frontmatterText := content[len("---\n") : len("---\n")+frontmatterEnd]
	var frontmatter skillFrontmatter
	for _, line := range strings.Split(frontmatterText, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("SKILL.md frontmatter line %q must be a non-empty key/value pair", line)
		}
		switch key {
		case "name":
			if frontmatter.Name != "" {
				return fmt.Errorf("SKILL.md frontmatter repeats name")
			}
			frontmatter.Name = value
		case "description":
			if frontmatter.Description != "" {
				return fmt.Errorf("SKILL.md frontmatter repeats description")
			}
			frontmatter.Description = value
		default:
			return fmt.Errorf("SKILL.md frontmatter contains unexpected field %q", key)
		}
	}
	if !regexp.MustCompile(`^[a-z0-9-]{1,64}$`).MatchString(frontmatter.Name) || strings.HasPrefix(frontmatter.Name, "-") || strings.HasSuffix(frontmatter.Name, "-") || strings.Contains(frontmatter.Name, "--") {
		return fmt.Errorf("SKILL.md frontmatter name %q is not valid hyphen-case", frontmatter.Name)
	}
	if frontmatter.Name != "qweather" {
		return fmt.Errorf("SKILL.md frontmatter name = %q, want qweather", frontmatter.Name)
	}
	description := strings.TrimSpace(frontmatter.Description)
	if description == "" || len(description) > 1024 || strings.ContainsAny(description, "<>") {
		return fmt.Errorf("SKILL.md frontmatter description must be non-empty, at most 1024 bytes, and contain no angle brackets")
	}
	return nil
}
