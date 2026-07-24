package output

import (
	"strings"
	"testing"
)

func TestGroup3OfficialTemplates(t *testing.T) {
	tests := []struct {
		capability string
		fixture    string
		expected   []string
	}{
		{
			capability: "air.current",
			fixture:    "getAirqualityCurrent.json",
			expected: []string{
				"Air quality index 1:\n  Name: AQI (US)\n  Code: us-epa\n  AQI: 46\n",
				"Air quality index 2:\n  Name: QAQI\n  Code: qaqi\n  AQI: 0.9\n",
				"Pollutant 1:\n  Name: PM 2.5\n  Code: pm2p5\n",
				"Pollutant 1 / sub-index 1:\n  Code: us-epa\n  AQI: 46\n",
				"Pollutant 5:\n  Name: CO\n  Code: co\n",
				"Monitoring station 3:\n  Name: Los Angeles - N. Main Street\n  ID: P57327\n",
			},
		},
		{
			capability: "air.forecast.daily",
			fixture:    "getAirqualityDailyForecast.json",
			expected: []string{
				"Day 1:\n  Forecast start: 2023-02-14T23:00Z\n  Forecast end: 2023-02-15T23:00Z\n",
				"Day 1 / air quality index 1:\n  Name: QAQI\n  Code: qaqi\n  AQI: 1.0\n",
				"Day 1 / pollutant 1:\n  Name: PM 2.5\n  Code: pm2p5\n",
				"Day 1 / pollutant 1 / sub-index 1:\n  Code: eu-eea\n  AQI: 2\n",
				"Day 3:\n  Forecast start: 2023-02-16T23:00Z\n  Forecast end: 2023-02-17T23:00Z\n",
				"Day 3 / pollutant 5:\n  Name: SO2\n  Code: so2\n",
				"Day 3 / pollutant 5 / sub-index 2:\n  Code: qaqi\n  AQI: 0.5\n",
			},
		},
		{
			capability: "air.forecast.hourly",
			fixture:    "getAirqualityHourlyForecast.json",
			expected: []string{
				"Hour 1:\n  Forecast time: 2023-05-17T03:00Z\n",
				"Hour 1 / air quality index 1:\n  Name: QAQI\n  Code: qaqi\n  AQI: 1.4\n",
				"Hour 1 / pollutant 1:\n  Name: PM 2.5\n  Code: pm2p5\n",
				"Hour 1 / pollutant 1 / sub-index 1:\n  Code: qaqi\n  AQI: 1.4\n",
				"Hour 24:\n  Forecast time: 2023-05-18T02:00Z\n",
				"Hour 24 / pollutant 5:\n  Name: SO2\n  Code: so2\n",
				"Hour 24 / pollutant 5 / sub-index 1:\n  Code: qaqi\n  AQI: 0.7\n",
			},
		},
		{
			capability: "air.station.current",
			fixture:    "getAirqualityAirStation.json",
			expected: []string{
				"Pollutant 1:\n  Name: PM 2.5\n  Code: pm2p5\n  Full name: Fine particulate matter (<2.5µm)\n  Concentration: 17.0\n  Concentration unit: μg/m3\n",
				"Pollutant 4:\n  Name: O3\n  Code: o3\n",
			},
		},
		{
			capability: "storm.list",
			fixture:    "getStormList-en.json",
			expected: []string{
				"Storm list:\n  Provider code: 200\n  Updated at: 2020-12-31T16:00+00:00\n",
				"Storm 1:\n  Name: Vamco\n  ID: NP_2022\n  Basin: NP\n  Year: 2020\n  Active: 0\n",
				"Storm 23:\n  Name: Vongfong\n  ID: NP_2001\n",
			},
		},
		{
			capability: "storm.track",
			fixture:    "getStormTrack.json",
			expected: []string{
				"Storm track:\n  Provider code: 200\n  Updated at: 2024-05-30T06:11+00:00\n",
				"Current position:\n  Published at: 2024-05-30T05:00+08:00\n  Latitude: 27.7\n  Longitude: 134.5\n  Type: STS\n",
				"Track point 3:\n  Time: 2024-05-30T02:00+08:00\n  Latitude: 27.1\n  Longitude: 133.9\n  Type: STS\n",
			},
		},
		{
			capability: "storm.forecast",
			fixture:    "getStormForecast.json",
			expected: []string{
				"Storm forecast:\n  Provider code: 200\n  Updated at: 2021-07-27T03:00+00:00\n",
				"Forecast point 1:\n  Forecast time: 2021-07-27T20:00+08:00\n  Latitude: 31.7\n  Longitude: 118.4\n  Type: TS\n",
				"Forecast point 7:\n  Forecast time: 2021-07-31T08:00+08:00\n  Latitude: 38\n  Longitude: 119.8\n  Type: TD\n",
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
