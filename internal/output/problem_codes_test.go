package output

import (
	"slices"
	"testing"
)

func TestProblemDefinitionsAreCompleteAndDeterministic(t *testing.T) {
	definitions := ProblemDefinitions()
	if got, want := len(definitions), 19; got != want {
		t.Fatalf("definition count = %d, want %d", got, want)
	}
	seen := make(map[ProblemCode]bool, len(definitions))
	for index, definition := range definitions {
		if definition.Code == "" || definition.Meaning == "" {
			t.Fatalf("definition %d is incomplete: %#v", index, definition)
		}
		if seen[definition.Code] {
			t.Fatalf("duplicate problem code %s", definition.Code)
		}
		seen[definition.Code] = true
	}
	if !slices.IsSortedFunc(definitions, func(left, right ProblemDefinition) int {
		if left.ExitCode != right.ExitCode {
			return left.ExitCode - right.ExitCode
		}
		if left.Code < right.Code {
			return -1
		}
		if left.Code > right.Code {
			return 1
		}
		return 0
	}) {
		t.Fatal("problem definitions are not sorted by exit code and symbolic code")
	}

	definitions[0].Meaning = "mutated"
	if ProblemDefinitions()[0].Meaning == "mutated" {
		t.Fatal("ProblemDefinitions returned mutable package state")
	}
}
