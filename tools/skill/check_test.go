package main

import (
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
		{name: "unknown field", content: strings.Replace(valid, "description:", "extra: true\ndescription:", 1), want: "unexpected field"},
		{name: "wrong name", content: strings.Replace(valid, "name: qweather", "name: other", 1), want: "want qweather"},
		{name: "empty description", content: strings.Replace(valid, "description: Safe QWeather guidance.", "description: ", 1), want: "non-empty key/value"},
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
