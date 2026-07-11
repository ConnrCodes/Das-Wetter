package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"daswetter/internal/model"
)

func TestCurrentMinimalOutput(t *testing.T) {
	sunset := time.Date(2026, 7, 11, 20, 49, 0, 0, time.FixedZone("EDT", -4*60*60))
	w := model.Weather{
		Location:    "Atlanta, GA, USA",
		Temperature: 82,
		FeelsLike:   86,
		Humidity:    65,
		Wind:        8,
		CloudCover:  30,
		WeatherCode: 2,
		Sunset:      sunset,
		Units:       "imperial",
	}

	var b bytes.Buffer
	Current(&b, w, 0, false)
	out := b.String()
	for _, want := range []string{
		"Atlanta, GA, USA",
		"🌤 82°F",
		"💨 Wind: 8 mph",
		"💧 Humidity: 65%",
		"☁ Cloud cover: 30%",
		"🌅 Sunset: 8:49 PM",
		"Feels like 86°F",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Current output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("one-shot output unexpectedly contains ANSI control codes: %q", out)
	}
}

func TestCurrentOptionalSections(t *testing.T) {
	start := time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC)
	w := model.Weather{
		Location:  "Atlanta, GA, USA",
		Units:     "metric",
		MoonPhase: "Waxing Crescent",
		Viewing:   "Good",
		Hours: []model.Hour{
			{Time: start, Temperature: 25, RainChance: 20},
			{Time: start.Add(time.Hour), Temperature: 26, RainChance: 30},
		},
		Alerts: []model.Alert{{Event: "Thunderstorm Warning", Starts: start, Ends: start.Add(4 * time.Hour)}},
		Stale:  true,
	}

	var b bytes.Buffer
	Current(&b, w, 1, true)
	out := b.String()
	for _, want := range []string{
		"Sat 2 PM    25°C  rain  20%",
		"🌙 Moon: Waxing Crescent",
		"🔭 Viewing: Good",
		"⚠ Thunderstorm Warning (2 PM–6 PM)",
		"(cached; live service unavailable)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Current optional output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Sat 3 PM") {
		t.Fatalf("--hours limit was ignored:\n%s", out)
	}
}

func TestCurrentVisualHierarchy(t *testing.T) {
	w := model.Weather{
		Location:    "Atlanta, GA",
		Timezone:    "America/New_York",
		FetchedAt:   time.Date(2026, 7, 11, 20, 46, 0, 0, time.UTC),
		Source:      "Open-Meteo",
		Temperature: 82,
		FeelsLike:   86,
		WeatherCode: 2,
		Units:       "imperial",
		Days: []model.Day{
			{Date: time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC), High: 92, Low: 73, WeatherCode: 2},
			{Date: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), High: 89, Low: 72, WeatherCode: 80},
		},
	}
	var b bytes.Buffer
	Current(&b, w, 0, false)
	out := b.String()
	for _, want := range []string{"› ~ das wetter", "Saturday, July 11, 2026", "Open-Meteo", "Forecast", "Sat", "92°F", "Showers", "Tip: weather --json", "weather help"} {
		if !strings.Contains(out, want) {
			t.Errorf("visual hierarchy missing %q:\n%s", want, out)
		}
	}
}

func TestJSONWeatherSingleObject(t *testing.T) {
	fetched := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	w := model.Weather{
		Location:    "Atlanta, GA, USA",
		Temperature: 82,
		FeelsLike:   86,
		Humidity:    65,
		Wind:        8,
		CloudCover:  30,
		RainChance:  10,
		Condition:   "Partly Cloudy",
		Units:       "imperial",
		FetchedAt:   fetched,
	}

	var b bytes.Buffer
	if err := JSONWeather(&b, []model.Weather{w}); err != nil {
		t.Fatalf("JSONWeather: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b.String())
	}
	for key, want := range map[string]any{
		"location":    "Atlanta, GA, USA",
		"temperature": float64(82),
		"feels_like":  float64(86),
		"humidity":    float64(65),
		"wind_mph":    float64(8),
		"cloud_cover": float64(30),
		"condition":   "Partly Cloudy",
	} {
		if got[key] != want {
			t.Errorf("%s = %#v, want %#v", key, got[key], want)
		}
	}
	if _, exists := got["wind_kph"]; exists {
		t.Fatalf("imperial JSON contains wind_kph: %s", b.String())
	}
}

func TestJSONWeatherMultipleLocationsIsArray(t *testing.T) {
	ws := []model.Weather{
		{Location: "Atlanta, GA, USA", Units: "metric", Wind: 12},
		{Location: "New York, NY, USA", Units: "metric", Wind: 9},
	}
	var b bytes.Buffer
	if err := JSONWeather(&b, ws); err != nil {
		t.Fatalf("JSONWeather: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("multiple-location JSON is not an array: %v\n%s", err, b.String())
	}
	if len(got) != 2 || got[0]["wind_kph"] != float64(12) {
		t.Fatalf("unexpected JSON payload: %#v", got)
	}
}

func TestCompare(t *testing.T) {
	ws := []model.Weather{
		{Location: "Atlanta, GA, USA", Temperature: 82, WeatherCode: 0, Units: "imperial"},
		{Location: "New York, NY, USA", Temperature: 75, WeatherCode: 61, Units: "imperial"},
		{Location: "Los Angeles, CA, USA", Temperature: 88, WeatherCode: 0, Units: "imperial"},
	}
	var b bytes.Buffer
	Compare(&b, ws)
	out := b.String()
	for _, want := range []string{"ATL         82°F ☀", "NYC         75°F 🌧", "LA          88°F ☀"} {
		if !strings.Contains(out, want) {
			t.Errorf("Compare output missing %q:\n%s", want, out)
		}
	}
}

func TestGraph(t *testing.T) {
	w := model.Weather{Hours: []model.Hour{
		{Time: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), Temperature: 50},
		{Time: time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC), Temperature: 55},
	}}
	var b bytes.Buffer
	Graph(&b, w)
	if !strings.Contains(b.String(), "hour") || !strings.Contains(b.String(), "*") {
		t.Fatalf("bad graph: %s", b.String())
	}
}

func TestGraphHonorsColumns(t *testing.T) {
	t.Setenv("COLUMNS", "24")
	w := model.Weather{Hours: []model.Hour{
		{Time: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), Temperature: 50},
		{Time: time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC), Temperature: 55},
	}}
	var b bytes.Buffer
	Graph(&b, w)
	for _, line := range strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n") {
		if len([]rune(line)) > 24 {
			t.Fatalf("graph exceeded COLUMNS=24: %d columns in %q", len([]rune(line)), line)
		}
	}
}
