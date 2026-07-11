// Package model contains the provider-independent weather data structures.
package model

import "time"

type Hour struct {
	Time        time.Time `json:"time"`
	Temperature float64   `json:"temperature"`
	RainChance  float64   `json:"rain_chance"`
	WeatherCode int       `json:"weather_code,omitempty"`
}

type Day struct {
	Date        time.Time `json:"date"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	WeatherCode int       `json:"weather_code,omitempty"`
}

type Alert struct {
	Event       string    `json:"event"`
	Severity    string    `json:"severity,omitempty"`
	Starts      time.Time `json:"starts,omitempty"`
	Ends        time.Time `json:"ends,omitempty"`
	Description string    `json:"description,omitempty"`
}

type Weather struct {
	Location      string    `json:"location"`
	Latitude      float64   `json:"latitude,omitempty"`
	Longitude     float64   `json:"longitude,omitempty"`
	Timezone      string    `json:"timezone,omitempty"`
	Temperature   float64   `json:"temperature"`
	FeelsLike     float64   `json:"feels_like"`
	Humidity      float64   `json:"humidity"`
	Wind          float64   `json:"wind"`
	WindDirection float64   `json:"wind_direction,omitempty"`
	CloudCover    float64   `json:"cloud_cover"`
	Pressure      float64   `json:"pressure,omitempty"`
	Visibility    float64   `json:"visibility,omitempty"`
	UVIndex       float64   `json:"uv_index,omitempty"`
	RainChance    float64   `json:"rain_chance"`
	Condition     string    `json:"condition"`
	WeatherCode   int       `json:"weather_code,omitempty"`
	Source        string    `json:"source,omitempty"`
	ValidAt       time.Time `json:"valid_at,omitempty"`
	Sunset        time.Time `json:"sunset,omitempty"`
	MoonPhase     string    `json:"moon_phase,omitempty"`
	Viewing       string    `json:"viewing_quality,omitempty"`
	Hours         []Hour    `json:"hours,omitempty"`
	Days          []Day     `json:"days,omitempty"`
	Alerts        []Alert   `json:"alerts,omitempty"`
	FetchedAt     time.Time `json:"fetched_at"`
	Units         string    `json:"units"`
	Stale         bool      `json:"stale,omitempty"`
}

func Condition(code int) (string, string) {
	switch {
	case code == 0:
		return "Clear", "☀"
	case code == 1:
		return "Mainly Clear", "🌤"
	case code == 2:
		return "Partly Cloudy", "🌤"
	case code == 3:
		return "Overcast", "☁"
	case code == 45 || code == 48:
		return "Fog", "🌫"
	case code >= 51 && code <= 57:
		return "Drizzle", "🌦"
	case code >= 61 && code <= 67:
		return "Rain", "🌧"
	case code >= 71 && code <= 77:
		return "Snow", "🌨"
	case code >= 80 && code <= 82:
		return "Showers", "🌧"
	case code == 85 || code == 86:
		return "Snow Showers", "🌨"
	case code >= 95:
		return "Thunderstorm", "⛈"
	default:
		return "Unknown", "·"
	}
}
