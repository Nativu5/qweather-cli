package output

import (
	"strings"
	"testing"
)

func TestGroup2OfficialTemplates(t *testing.T) {
	tests := []struct {
		name         string
		capabilityID string
		fixture      string
		want         []string
	}{
		{
			name:         "grid current",
			capabilityID: "weather.grid.current",
			fixture:      "getGridWeatherNow-en.json",
			want: []string{
				"Current grid weather:\n",
				"  Temperature: 31 °C\n",
				"  Wind speed: 15 km/h\n",
			},
		},
		{
			name:         "grid daily forecast",
			capabilityID: "weather.grid.forecast.daily",
			fixture:      "getGridWeatherDaily-en.json",
			want: []string{
				"Forecast day 1:\n  Date: 2023-05-30\n",
				"  Day condition: Few Clouds\n",
				"Forecast day 3:\n  Date: 2023-06-01\n",
			},
		},
		{
			name:         "grid hourly forecast",
			capabilityID: "weather.grid.forecast.hourly",
			fixture:      "getGridWeatherHourly-en.json",
			want: []string{
				"Forecast hour 1:\n  Forecast time: 2023-05-30T11:00+00:00\n",
				"  Condition: Light Rain\n",
				"Forecast hour 12:\n  Forecast time: 2023-05-30T22:00+00:00\n",
			},
		},
		{
			name:         "minutely precipitation",
			capabilityID: "weather.precipitation.minutely",
			fixture:      "getMinutelyPrecipitation-en.json",
			want: []string{
				"Precipitation summary:\n  Summary: Rain will stop in 95 minutes\n",
				"Interval 1:\n  Forecast time: 2021-12-16T18:55+08:00\n",
				"Interval 24:\n  Forecast time: 2021-12-16T20:50+08:00\n",
			},
		},
		{
			name:         "weather indices",
			capabilityID: "weather.indices.forecast",
			fixture:      "getWeatherIndices-en.json",
			want: []string{
				"Weather index 1:\n  Date: 2021-12-16\n  Name: Sports\n",
				"  Category: Poor\n",
				"Weather index 2:\n  Date: 2021-12-16\n  Name: Car Wash\n",
			},
		},
		{
			name:         "weather history",
			capabilityID: "weather.history",
			fixture:      "getHistoricalWeather-en.json",
			want: []string{
				"Daily history:\n  Date: 2020-07-25\n",
				"Historical hour 1:\n  Time: 2020-07-25 00:00\n",
				"Historical hour 24:\n  Time: 2020-07-25 23:00\n",
			},
		},
		{
			name:         "current alerts",
			capabilityID: "alert.current",
			fixture:      "getWeatherAlertCurrent-en.json",
			want: []string{
				"Weather alert 1:\n  ID: 202510162100007104337971\n",
				"  Headline: Strong Wind Warning - Orange\n",
				"  Severity: moderate\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := renderOfficialTemplate(t, test.capabilityID, test.fixture)
			for _, want := range test.want {
				if !strings.Contains(output, want) {
					t.Errorf("output for %s missing %q:\n%s", test.capabilityID, want, output)
				}
			}
		})
	}
}
