package output

import (
	"strings"
	"testing"
)

func TestGroup4OfficialTemplates(t *testing.T) {
	tests := []struct {
		capability string
		fixture    string
		expected   []string
	}{
		{
			capability: "marine.tide",
			fixture:    "getOceanTide.json",
			expected: []string{
				"Tide forecast:\n  Provider code: 200\n  Updated at: 2021-02-04T05:02+08:00\n",
				"Tide event 1:\n  Forecast time: 2021-02-06T03:48+08:00\n  Height: 2.17 m\n  Type: H\n",
				"Tide event 4:\n  Forecast time: 2021-02-06T23:22+08:00\n  Height: 0.73 m\n  Type: L\n",
				"Hourly tide 24:\n  Forecast time: 2021-02-06T23:00+08:00\n  Height: 0.74 m\n",
			},
		},
		{
			capability: "solar.radiation.forecast",
			fixture:    "getSolarradiationForecast.json",
			expected: []string{
				"Solar radiation forecast 1:\n  Forecast time: 2023-10-15T11:30Z\n  Solar azimuth: 184 °\n  Solar elevation: 40 °\n",
				"  Direct normal irradiance: 25.16\n  Direct normal irradiance unit: W/m²\n",
				"Solar radiation forecast 1 / weather:\n  Temperature: 18.6\n  Temperature unit: °C\n",
				"Solar radiation forecast 4 / plane of array:\n  Global irradiance: 118.87\n",
				"  Reflected irradiance: 1.3\n  Reflected irradiance unit: W/m²\n",
			},
		},
		{
			capability: "astronomy.sun.events",
			fixture:    "getAstronomSun.json",
			expected: []string{
				"Sun events:\n  Provider code: 200\n",
				"  Sunrise: 2021-02-20T06:58+08:00\n",
				"  Sunset: 2021-02-20T17:57+08:00\n",
			},
		},
		{
			capability: "astronomy.moon.events",
			fixture:    "getAstronomyMoon-en.json",
			expected: []string{
				"Moon events:\n  Provider code: 200\n",
				"Moon phase 1:\n  Forecast time: 2021-11-20T00:00+08:00\n  Name: Waning gibbous\n",
				"Moon phase 24:\n  Forecast time: 2021-11-20T23:00+08:00\n  Name: Waning gibbous\n  Value: 0.54\n  Illumination: 98 %\n",
			},
		},
		{
			capability: "astronomy.solar.position",
			fixture:    "getAstronomySolarElevationAngle.json",
			expected: []string{
				"Solar position:\n  Provider code: 200\n",
				"  Solar elevation: 42.88 °\n  Solar azimuth: 185.92 °\n",
				"  Solar time: 1217\n  Hour angle: -4.41 °\n",
			},
		},
		{
			capability: "account.finance.summary",
			fixture:    "getConsoleFinance.json",
			expected: []string{
				"Finance summary:\n  As of: 2024-04-03T17:13Z\n  Currency: CNY\n  Balance: -17.54\n",
				"Pending bill 2:\n  Number: 605D0FYX\n  Issue date: 2024-04-02T13:34Z\n",
				"Available savings plan 1:\n  Bill number: 605D0FYX\n  Status: pending\n  Term (years): 1\n",
				"Available resource plan 1:\n  Bill number: 613D1FYX\n  Status: active\n  Requests: 1000000\n  Utilized: 12\n  Effective time: 2024-04-03T17:00Z\n",
			},
		},
		{
			capability: "account.requests.stats",
			fixture:    "getConsoleStats.json",
			expected: []string{
				"Request statistics:\n  As of: 2025-05-12T02:59Z\n",
				"Successful API 1:\n  API: Weather\n",
				"Successful API 1 / hour 1:\n  Requests: 482\n",
				"Successful API 5 / hour 24:\n  Requests: 29\n",
				"Error API 2:\n  API: WeatherAlert\n",
				"Error API 2 / hour 24:\n  Requests: 1\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.capability, func(t *testing.T) {
			output := renderOfficialTemplate(t, test.capability, test.fixture)
			for _, expected := range test.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("output missing %q:\n%s", expected, output)
				}
			}
		})
	}
}
