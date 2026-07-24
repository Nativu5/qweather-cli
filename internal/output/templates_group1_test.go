package output

import (
	"strings"
	"testing"
)

func TestOfficialGeoCityLookupTemplate(t *testing.T) {
	text := renderOfficialTemplate(t, "geo.city.lookup", "getGeoCitylookup-en.json")
	for _, expected := range []string{
		"Locations:\n",
		"Location 1:\n",
		"  Name: Dongcheng\n",
		"  ID: 101011600\n",
		"  Time zone: Asia/Shanghai\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
}

func TestOfficialGeoCityTopTemplate(t *testing.T) {
	text := renderOfficialTemplate(t, "geo.city.top", "getGeoTopcity-en.json")
	for _, expected := range []string{
		"Top cities:\n",
		"City 1:\n  Name: Beijing\n",
		"City 10:\n  Name: Hangzhou\n",
		"  ID: 101210101\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
}

func TestOfficialGeoPOILookupTemplate(t *testing.T) {
	text := renderOfficialTemplate(t, "geo.poi.lookup", "getGeoPoilookup-en.json")
	for _, expected := range []string{
		"Points of interest:\n",
		"POI 1:\n",
		"  Name: Nanluoguxiang Alley\n",
		"  Type: scenic\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
}

func TestOfficialGeoPOINearbyTemplate(t *testing.T) {
	text := renderOfficialTemplate(t, "geo.poi.nearby", "getGeoPoirange-en.json")
	for _, expected := range []string{
		"Nearby points of interest:\n",
		"POI 4:\n  Name: The Palace Museum\n",
		"POI 5:\n  Name: Beijing Imperial Academy\n",
		"  ID: 10101010006A\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
}

func TestOfficialWeatherCityCurrentTemplate(t *testing.T) {
	text := renderOfficialTemplate(t, "weather.city.current", "getWeatherNow-en.json")
	for _, expected := range []string{
		"Current weather:\n",
		"  Observed at: 2023-04-12T18:22+08:00\n",
		"  Condition: Fog\n",
		"  Temperature: 26 °C\n",
		"  Visibility: 4 km\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
}

func TestOfficialWeatherCityDailyForecastTemplate(t *testing.T) {
	text := renderOfficialTemplate(t, "weather.city.forecast.daily", "getWeatherDailyForecast-en.json")
	for _, expected := range []string{
		"Daily forecast:\n",
		"Day 1:\n  Date: 2023-04-12\n",
		"  Day condition: Thundershower\n",
		"  Minimum temperature: 20 °C\n",
		"Day 3:\n  Date: 2023-04-14\n",
		"  Maximum temperature: 30 °C\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
}

func TestOfficialWeatherCityHourlyForecastTemplate(t *testing.T) {
	text := renderOfficialTemplate(t, "weather.city.forecast.hourly", "getWeatherHourlyForecast-en.json")
	for _, expected := range []string{
		"Hourly forecast:\n",
		"Hour 1:\n  Forecast time: 2023-04-12T19:00+08:00\n",
		"  Temperature: 24 °C\n",
		"Hour 3:\n  Forecast time: 2023-04-12T21:00+08:00\n",
		"  Condition: Cloudy\n",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("Text output missing %q:\n%s", expected, text)
		}
	}
}
