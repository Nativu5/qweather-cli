package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type officialTemplateTest struct {
	capabilityID string
	fixture      string
	want         []string
	golden       string
}

func TestOfficialTemplates(t *testing.T) {
	renderer, err := loadRenderer()
	if err != nil {
		t.Fatal(err)
	}

	tests := []officialTemplateTest{
		{
			capabilityID: "geo.city.lookup",
			fixture:      "getGeoCitylookup-en.json",
			want: []string{
				"Locations:\n",
				"Location 1:\n",
				"  Name: Dongcheng\n",
				"  ID: 101011600\n",
				"  Time zone: Asia/Shanghai\n",
			},
		},
		{
			capabilityID: "geo.city.top",
			fixture:      "getGeoTopcity-en.json",
			want: []string{
				"Top cities:\n",
				"City 1:\n  Name: Beijing\n",
				"City 10:\n  Name: Hangzhou\n",
				"  ID: 101210101\n",
			},
		},
		{
			capabilityID: "geo.poi.lookup",
			fixture:      "getGeoPoilookup-en.json",
			want: []string{
				"Points of interest:\n",
				"POI 1:\n",
				"  Name: Nanluoguxiang Alley\n",
				"  Type: scenic\n",
			},
		},
		{
			capabilityID: "geo.poi.nearby",
			fixture:      "getGeoPoirange-en.json",
			want: []string{
				"Nearby points of interest:\n",
				"POI 4:\n  Name: The Palace Museum\n",
				"POI 5:\n  Name: Beijing Imperial Academy\n",
				"  ID: 10101010006A\n",
			},
		},
		{
			capabilityID: "weather.city.current",
			fixture:      "getWeatherNow-en.json",
			golden:       "current-object.txt",
		},
		{
			capabilityID: "weather.city.forecast.daily",
			fixture:      "getWeatherDailyForecast-en.json",
			golden:       "array-forecast.txt",
		},
		{
			capabilityID: "weather.city.forecast.hourly",
			fixture:      "getWeatherHourlyForecast-en.json",
			want: []string{
				"Hourly forecast:\n",
				"Hour 1:\n  Forecast time: 2023-04-12T19:00+08:00\n",
				"  Temperature: 24 °C\n",
				"Hour 3:\n  Forecast time: 2023-04-12T21:00+08:00\n",
				"  Condition: Cloudy\n",
			},
		},
		{
			capabilityID: "weather.grid.current",
			fixture:      "getGridWeatherNow-en.json",
			want: []string{
				"Current grid weather:\n",
				"  Temperature: 31 °C\n",
				"  Wind speed: 15 km/h\n",
			},
		},
		{
			capabilityID: "weather.grid.forecast.daily",
			fixture:      "getGridWeatherDaily-en.json",
			want: []string{
				"Forecast day 1:\n  Date: 2023-05-30\n",
				"  Day condition: Few Clouds\n",
				"Forecast day 3:\n  Date: 2023-06-01\n",
			},
		},
		{
			capabilityID: "weather.grid.forecast.hourly",
			fixture:      "getGridWeatherHourly-en.json",
			want: []string{
				"Forecast hour 1:\n  Forecast time: 2023-05-30T11:00+00:00\n",
				"  Condition: Light Rain\n",
				"Forecast hour 12:\n  Forecast time: 2023-05-30T22:00+00:00\n",
			},
		},
		{
			capabilityID: "weather.precipitation.minutely",
			fixture:      "getMinutelyPrecipitation-en.json",
			want: []string{
				"Precipitation summary:\n  Summary: Rain will stop in 95 minutes\n",
				"Interval 1:\n  Forecast time: 2021-12-16T18:55+08:00\n",
				"Interval 24:\n  Forecast time: 2021-12-16T20:50+08:00\n",
			},
		},
		{
			capabilityID: "weather.indices.forecast",
			fixture:      "getWeatherIndices-en.json",
			want: []string{
				"Weather index 1:\n  Date: 2021-12-16\n  Name: Sports\n",
				"  Category: Poor\n",
				"Weather index 2:\n  Date: 2021-12-16\n  Name: Car Wash\n",
			},
		},
		{
			capabilityID: "weather.history",
			fixture:      "getHistoricalWeather-en.json",
			want: []string{
				"Daily history:\n  Date: 2020-07-25\n",
				"Historical hour 1:\n  Time: 2020-07-25 00:00\n",
				"Historical hour 24:\n  Time: 2020-07-25 23:00\n",
			},
		},
		{
			capabilityID: "alert.current",
			fixture:      "getWeatherAlertCurrent-en.json",
			want: []string{
				"Weather alert 1:\n  ID: 202510162100007104337971\n",
				"  Headline: Strong Wind Warning - Orange\n",
				"  Severity: moderate\n",
			},
		},
		{
			capabilityID: "air.current",
			fixture:      "getAirqualityCurrent.json",
			golden:       "nested-metadata.txt",
		},
		{
			capabilityID: "air.forecast.daily",
			fixture:      "getAirqualityDailyForecast.json",
			want: []string{
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
			capabilityID: "air.forecast.hourly",
			fixture:      "getAirqualityHourlyForecast.json",
			want: []string{
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
			capabilityID: "air.station.current",
			fixture:      "getAirqualityAirStation.json",
			want: []string{
				"Pollutant 1:\n  Name: PM 2.5\n  Code: pm2p5\n  Full name: Fine particulate matter (<2.5µm)\n  Concentration: 17.0\n  Concentration unit: μg/m3\n",
				"Pollutant 4:\n  Name: O3\n  Code: o3\n",
			},
		},
		{
			capabilityID: "storm.list",
			fixture:      "getStormList-en.json",
			want: []string{
				"Storm list:\n  Provider code: 200\n  Updated at: 2020-12-31T16:00+00:00\n",
				"Storm 1:\n  Name: Vamco\n  ID: NP_2022\n  Basin: NP\n  Year: 2020\n  Active: 0\n",
				"Storm 23:\n  Name: Vongfong\n  ID: NP_2001\n",
			},
		},
		{
			capabilityID: "storm.track",
			fixture:      "getStormTrack.json",
			want: []string{
				"Storm track:\n  Provider code: 200\n  Updated at: 2024-05-30T06:11+00:00\n",
				"Current position:\n  Published at: 2024-05-30T05:00+08:00\n  Latitude: 27.7\n  Longitude: 134.5\n  Type: STS\n",
				"Track point 3:\n  Time: 2024-05-30T02:00+08:00\n  Latitude: 27.1\n  Longitude: 133.9\n  Type: STS\n",
			},
		},
		{
			capabilityID: "storm.forecast",
			fixture:      "getStormForecast.json",
			want: []string{
				"Storm forecast:\n  Provider code: 200\n  Updated at: 2021-07-27T03:00+00:00\n",
				"Forecast point 1:\n  Forecast time: 2021-07-27T20:00+08:00\n  Latitude: 31.7\n  Longitude: 118.4\n  Type: TS\n",
				"Forecast point 7:\n  Forecast time: 2021-07-31T08:00+08:00\n  Latitude: 38\n  Longitude: 119.8\n  Type: TD\n",
			},
		},
		{
			capabilityID: "marine.tide",
			fixture:      "getOceanTide.json",
			want: []string{
				"Tide forecast:\n  Provider code: 200\n  Updated at: 2021-02-04T05:02+08:00\n",
				"Tide event 1:\n  Forecast time: 2021-02-06T03:48+08:00\n  Height: 2.17 m\n  Type: H\n",
				"Tide event 4:\n  Forecast time: 2021-02-06T23:22+08:00\n  Height: 0.73 m\n  Type: L\n",
				"Hourly tide 24:\n  Forecast time: 2021-02-06T23:00+08:00\n  Height: 0.74 m\n",
			},
		},
		{
			capabilityID: "solar.radiation.forecast",
			fixture:      "getSolarradiationForecast.json",
			want: []string{
				"Solar radiation forecast 1:\n  Forecast time: 2023-10-15T11:30Z\n  Solar azimuth: 184 °\n  Solar elevation: 40 °\n",
				"  Direct normal irradiance: 25.16\n  Direct normal irradiance unit: W/m²\n",
				"Solar radiation forecast 1 / weather:\n  Temperature: 18.6\n  Temperature unit: °C\n",
				"Solar radiation forecast 4 / plane of array:\n  Global irradiance: 118.87\n",
				"  Reflected irradiance: 1.3\n  Reflected irradiance unit: W/m²\n",
			},
		},
		{
			capabilityID: "astronomy.sun.events",
			fixture:      "getAstronomSun.json",
			want: []string{
				"Sun events:\n  Provider code: 200\n",
				"  Sunrise: 2021-02-20T06:58+08:00\n",
				"  Sunset: 2021-02-20T17:57+08:00\n",
			},
		},
		{
			capabilityID: "astronomy.moon.events",
			fixture:      "getAstronomyMoon-en.json",
			want: []string{
				"Moon events:\n  Provider code: 200\n",
				"Moon phase 1:\n  Forecast time: 2021-11-20T00:00+08:00\n  Name: Waning gibbous\n",
				"Moon phase 24:\n  Forecast time: 2021-11-20T23:00+08:00\n  Name: Waning gibbous\n  Value: 0.54\n  Illumination: 98 %\n",
			},
		},
		{
			capabilityID: "astronomy.solar.position",
			fixture:      "getAstronomySolarElevationAngle.json",
			want: []string{
				"Solar position:\n  Provider code: 200\n",
				"  Solar elevation: 42.88 °\n  Solar azimuth: 185.92 °\n",
				"  Solar time: 1217\n  Hour angle: -4.41 °\n",
			},
		},
		{
			capabilityID: "account.finance.summary",
			fixture:      "getConsoleFinance.json",
			want: []string{
				"Finance summary:\n  As of: 2024-04-03T17:13Z\n  Currency: CNY\n  Balance: -17.54\n",
				"Pending bill 2:\n  Number: 605D0FYX\n  Issue date: 2024-04-02T13:34Z\n",
				"Available savings plan 1:\n  Bill number: 605D0FYX\n  Status: pending\n  Term (years): 1\n",
				"Available resource plan 1:\n  Bill number: 613D1FYX\n  Status: active\n  Requests: 1000000\n  Utilized: 12\n  Effective time: 2024-04-03T17:00Z\n",
			},
		},
		{
			capabilityID: "account.requests.stats",
			fixture:      "getConsoleStats.json",
			want: []string{
				"Request statistics:\n  As of: 2025-05-12T02:59Z\n",
				"Successful API 1:\n  API: Weather\n",
				"Successful API 1 / hour 1:\n  Requests: 482\n",
				"Successful API 5 / hour 24:\n  Requests: 29\n",
				"Error API 2:\n  API: WeatherAlert\n",
				"Error API 2 / hour 24:\n  Requests: 1\n",
			},
		},
	}

	if len(tests) != 28 {
		t.Fatalf("official template cases = %d, want 28", len(tests))
	}
	for _, test := range tests {
		t.Run(test.capabilityID, func(t *testing.T) {
			text := renderOfficialTemplate(t, renderer, test.capabilityID, test.fixture)
			for _, expected := range test.want {
				if !strings.Contains(text, expected) {
					t.Errorf("output for %s missing %q:\n%s", test.capabilityID, expected, text)
				}
			}
			if test.golden != "" {
				assertGolden(t, test.golden, text)
			}
		})
	}
}

func renderOfficialTemplate(t *testing.T, renderer *Renderer, capabilityID, fixture string) string {
	t.Helper()
	body, err := os.ReadFile("testdata/official/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := renderer.templates[capabilityID]; !exists {
		t.Fatalf("missing output template for %s", capabilityID)
	}
	result := testResult(capabilityID, string(body))
	result.ProviderBody = body
	result.Attribution = fixtureAttribution(t, body)
	var output bytes.Buffer
	info, err := renderer.RenderResult(&output, result, ModeText)
	if err != nil {
		t.Fatal(err)
	}
	if info.Fallback {
		t.Fatalf("%s unexpectedly used generic fallback", capabilityID)
	}
	text := output.String()
	if strings.Contains(text, "<no value>") {
		t.Fatalf("%s rendered <no value>: %s", capabilityID, text)
	}
	if strings.Count(text, "Attribution:") != 1 {
		t.Fatalf("%s rendered Attribution %d times: %s", capabilityID, strings.Count(text, "Attribution:"), text)
	}
	return text
}

func assertGolden(t *testing.T, name, actual string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if os.Getenv("QWEATHER_UPDATE_GOLDEN") == "1" {
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
}

func fixtureAttribution(t *testing.T, body []byte) []any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatal(err)
	}
	var attribution []any
	if metadata, ok := object["metadata"].(map[string]any); ok {
		if values, ok := metadata["attributions"].([]any); ok {
			attribution = append(attribution, values...)
		}
	}
	if refer, ok := object["refer"].(map[string]any); ok {
		if values, ok := refer["sources"].([]any); ok {
			for _, value := range values {
				attribution = append(attribution, map[string]any{"source": value})
			}
		}
		if values, ok := refer["license"].([]any); ok {
			for _, value := range values {
				attribution = append(attribution, map[string]any{"license": value})
			}
		}
	}
	return attribution
}
