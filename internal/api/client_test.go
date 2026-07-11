package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLocationAliases(t *testing.T) {
	tests := map[string]string{
		"atl": "Atlanta",
		"nyc": "New York",
		"la":  "Los Angeles",
		"sf":  "San Francisco",
	}
	for input, want := range tests {
		if got := locationAliases[input]; got != want {
			t.Errorf("locationAliases[%q] = %q, want %q", input, got, want)
		}
	}
}

func TestFormatLocationUS(t *testing.T) {
	got := formatLocation("Atlanta", "Georgia", "United States", "US")
	if got != "Atlanta, GA" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatLocationInternational(t *testing.T) {
	got := formatLocation("London", "England", "United Kingdom", "GB")
	if got != "London, England, United Kingdom" {
		t.Fatalf("got %q", got)
	}
}

func TestTomorrowUsesHourlyConditions(t *testing.T) {
	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	stamp := tomorrow.Format("2006-01-02") + "T09:00"
	raw := forecastResponse{Timezone: "UTC"}
	raw.Current.Temperature = 70
	raw.Current.Apparent = 70
	raw.Current.Humidity = 10
	raw.Current.Wind = 1
	raw.Current.Cloud = 2
	raw.Hourly.Time = []string{stamp}
	raw.Hourly.Temperature = []float64{80}
	raw.Hourly.Apparent = []float64{84}
	raw.Hourly.Humidity = []float64{65}
	raw.Hourly.Wind = []float64{8}
	raw.Hourly.Cloud = []float64{30}
	raw.Hourly.Code = []int{2}
	raw.Daily.Time = []string{time.Now().UTC().Format("2006-01-02"), tomorrow.Format("2006-01-02")}

	w := normalize(raw, Location{Name: "Atlanta"}, "imperial", true, 0)
	if w.Temperature != 80 || w.FeelsLike != 84 || w.Humidity != 65 || w.Wind != 8 || w.CloudCover != 30 {
		t.Fatalf("tomorrow mixed current and forecast conditions: %+v", w)
	}
	if w.Source != "Open-Meteo" || w.ValidAt.Hour() != 9 {
		t.Fatalf("source timestamp was not preserved: %+v", w)
	}
	if len(w.Hours) != 0 {
		t.Fatalf("hours should be omitted unless requested: %+v", w.Hours)
	}
}

func TestWeatherAPIFallbackNormalization(t *testing.T) {
	data := []byte(`{
  "location":{"name":"Atlanta","region":"Georgia","country":"United States of America","lat":33.75,"lon":-84.39,"tz_id":"America/New_York"},
  "current":{"last_updated_epoch":1783800000,"temp_c":27,"temp_f":80.6,"feelslike_c":29,"feelslike_f":84.2,"humidity":65,"wind_mph":8,"wind_kph":12.9,"wind_degree":270,"cloud":30,"pressure_mb":1015,"vis_km":16,"uv":7,"condition":{"text":"Partly cloudy"}},
  "forecast":{"forecastday":[{"date":"2026-07-11","day":{"maxtemp_c":31,"maxtemp_f":88,"mintemp_c":22,"mintemp_f":72,"daily_chance_of_rain":40,"condition":{"text":"Patchy rain possible"}},"astro":{"sunset":"08:49 PM"},"hour":[]}]}
}`)
	var raw weatherAPIResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	w := normalizeWeatherAPI(raw, "imperial", false, 0)
	if w.Location != "Atlanta, GA" || w.Temperature != 80.6 || w.FeelsLike != 84.2 || w.Wind != 8 {
		t.Fatalf("unexpected fallback normalization: %+v", w)
	}
	if w.Source != "WeatherAPI" || w.ValidAt.IsZero() {
		t.Fatalf("fallback source timestamp was not preserved: %+v", w)
	}
	if w.Condition != "Partly Cloudy" || w.RainChance != 40 || len(w.Days) != 1 || w.Days[0].WeatherCode != 63 {
		t.Fatalf("fallback forecast fields were not normalized: %+v", w)
	}
}
