package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepresentativeTextGoldens(t *testing.T) {
	tests := []struct {
		name         string
		capabilityID string
		fixture      string
	}{
		{name: "current-object", capabilityID: "weather.city.current", fixture: "getWeatherNow-en.json"},
		{name: "array-forecast", capabilityID: "weather.city.forecast.daily", fixture: "getWeatherDailyForecast-en.json"},
		{name: "nested-metadata", capabilityID: "air.current", fixture: "getAirqualityCurrent.json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := renderOfficialTemplate(t, test.capabilityID, test.fixture)
			path := filepath.Join("testdata", "golden", test.name+".txt")
			if os.Getenv("QWEATHER_UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			expected, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if actual != string(expected) {
				t.Fatalf("Text output differs from %s; review the layout and regenerate with QWEATHER_UPDATE_GOLDEN=1", path)
			}
		})
	}
}
