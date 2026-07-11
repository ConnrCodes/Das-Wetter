// Package api resolves locations and fetches normalized weather data from the
// configured public weather services.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"daswetter/internal/model"
)

const (
	geocodeURL  = "https://geocoding-api.open-meteo.com/v1/search"
	forecastURL = "https://api.open-meteo.com/v1/forecast"
	weatherURL  = "https://api.weatherapi.com/v1/forecast.json"
)

type Client struct{ HTTP *http.Client }

func New() *Client { return &Client{HTTP: &http.Client{Timeout: 5 * time.Second}} }

type Location struct {
	Name                string
	Latitude, Longitude float64
}
type place struct {
	Name        string  `json:"name"`
	Admin1      string  `json:"admin1"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

func (c *Client) IPLocation(ctx context.Context) (Location, error) {
	var first struct {
		Success     bool    `json:"success"`
		Message     string  `json:"message"`
		City        string  `json:"city"`
		Region      string  `json:"region"`
		Country     string  `json:"country"`
		CountryCode string  `json:"country_code"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
	}
	if err := c.getJSON(ctx, "https://ipwho.is/", &first); err == nil && first.Success && first.City != "" {
		return Location{Name: formatLocation(first.City, first.Region, first.Country, first.CountryCode), Latitude: first.Latitude, Longitude: first.Longitude}, nil
	}
	var second struct {
		City        string  `json:"city"`
		Region      string  `json:"region"`
		CountryName string  `json:"country_name"`
		CountryCode string  `json:"country_code"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
	}
	if err := c.getJSON(ctx, "https://ipapi.co/json/", &second); err != nil {
		return Location{}, fmt.Errorf("IP location failed: %w", err)
	}
	if second.City == "" || (second.Latitude == 0 && second.Longitude == 0) {
		return Location{}, fmt.Errorf("IP location returned no usable coordinates")
	}
	return Location{Name: formatLocation(second.City, second.Region, second.CountryName, second.CountryCode), Latitude: second.Latitude, Longitude: second.Longitude}, nil
}

func (c *Client) geocode(ctx context.Context, query string) (Location, error) {
	parts := strings.Split(query, ",")
	city := strings.TrimSpace(parts[0])
	if city == "" {
		return Location{}, fmt.Errorf("location is empty")
	}
	if expanded, ok := locationAliases[strings.ToLower(city)]; ok {
		city = expanded
	}
	q := url.Values{"name": {city}, "count": {"10"}, "language": {"en"}, "format": {"json"}}
	var r struct {
		Results []place `json:"results"`
	}
	if err := c.getJSON(ctx, geocodeURL+"?"+q.Encode(), &r); err != nil {
		return Location{}, err
	}
	if len(r.Results) == 0 {
		return Location{}, fmt.Errorf("location %q not found", query)
	}
	qualifier := strings.ToLower(strings.Join(parts[1:], " "))
	best := r.Results[0]
	bestScore := -1
	for _, p := range r.Results {
		score := 0
		hay := strings.ToLower(p.Admin1 + " " + p.Country + " " + p.CountryCode + " " + stateCode(p.Admin1))
		if qualifier != "" && strings.Contains(hay, qualifier) {
			score += 20
		}
		if strings.EqualFold(p.Name, city) {
			score += 5
		}
		if score > bestScore {
			best, bestScore = p, score
		}
	}
	return Location{Name: formatLocation(best.Name, best.Admin1, best.Country, best.CountryCode), Latitude: best.Latitude, Longitude: best.Longitude}, nil
}

func (c *Client) Fetch(ctx context.Context, query, units string, tomorrow bool, hours int) (model.Weather, error) {
	loc, err := c.geocode(ctx, query)
	if err != nil {
		return c.weatherAPIFallback(ctx, query, units, tomorrow, hours, err)
	}
	w, err := c.fetchOpenMeteo(ctx, loc, units, tomorrow, hours)
	if err == nil {
		return w, nil
	}
	return c.weatherAPIFallback(ctx, query, units, tomorrow, hours, err)
}
func (c *Client) FetchLocation(ctx context.Context, p Location, units string, tomorrow bool, hours int) (model.Weather, error) {
	w, err := c.fetchOpenMeteo(ctx, p, units, tomorrow, hours)
	if err == nil {
		return w, nil
	}
	query := strconv.FormatFloat(p.Latitude, 'f', 5, 64) + "," + strconv.FormatFloat(p.Longitude, 'f', 5, 64)
	return c.weatherAPIFallback(ctx, query, units, tomorrow, hours, err)
}

func (c *Client) fetchOpenMeteo(ctx context.Context, p Location, units string, tomorrow bool, hours int) (model.Weather, error) {
	tempUnit, windUnit := "fahrenheit", "mph"
	if units == "metric" {
		tempUnit, windUnit = "celsius", "kmh"
	}
	q := url.Values{"latitude": {fmt.Sprint(p.Latitude)}, "longitude": {fmt.Sprint(p.Longitude)}, "timezone": {"auto"}, "forecast_days": {"5"}, "temperature_unit": {tempUnit}, "wind_speed_unit": {windUnit}, "current": {"temperature_2m,apparent_temperature,relative_humidity_2m,cloud_cover,wind_speed_10m,wind_direction_10m,weather_code,precipitation_probability,pressure_msl,visibility,uv_index"}, "daily": {"sunset,temperature_2m_max,temperature_2m_min,weather_code"}}
	if hours > 0 || tomorrow {
		q.Set("hourly", "temperature_2m,apparent_temperature,relative_humidity_2m,cloud_cover,wind_speed_10m,wind_direction_10m,precipitation_probability,pressure_msl,visibility,uv_index,weather_code")
	}
	var raw forecastResponse
	if err := c.getJSON(ctx, forecastURL+"?"+q.Encode(), &raw); err != nil {
		return model.Weather{}, err
	}
	return normalize(raw, p, units, tomorrow, hours), nil
}

func (c *Client) weatherAPIFallback(ctx context.Context, query, units string, tomorrow bool, hours int, primaryErr error) (model.Weather, error) {
	key := strings.TrimSpace(os.Getenv("WEATHERAPI_KEY"))
	if key == "" {
		return model.Weather{}, primaryErr
	}
	w, err := c.fetchWeatherAPI(ctx, query, key, units, tomorrow, hours)
	if err != nil {
		return model.Weather{}, fmt.Errorf("Open-Meteo failed (%v); WeatherAPI fallback failed: %w", primaryErr, err)
	}
	return w, nil
}

type forecastResponse struct {
	Timezone string `json:"timezone"`
	Current  struct {
		Time          string  `json:"time"`
		Temperature   float64 `json:"temperature_2m"`
		Apparent      float64 `json:"apparent_temperature"`
		Humidity      float64 `json:"relative_humidity_2m"`
		Cloud         float64 `json:"cloud_cover"`
		Wind          float64 `json:"wind_speed_10m"`
		WindDirection float64 `json:"wind_direction_10m"`
		Code          int     `json:"weather_code"`
		Rain          float64 `json:"precipitation_probability"`
		Pressure      float64 `json:"pressure_msl"`
		Visibility    float64 `json:"visibility"`
		UVIndex       float64 `json:"uv_index"`
	} `json:"current"`
	Hourly struct {
		Time          []string  `json:"time"`
		Temperature   []float64 `json:"temperature_2m"`
		Apparent      []float64 `json:"apparent_temperature"`
		Humidity      []float64 `json:"relative_humidity_2m"`
		Cloud         []float64 `json:"cloud_cover"`
		Wind          []float64 `json:"wind_speed_10m"`
		WindDirection []float64 `json:"wind_direction_10m"`
		Rain          []float64 `json:"precipitation_probability"`
		Pressure      []float64 `json:"pressure_msl"`
		Visibility    []float64 `json:"visibility"`
		UVIndex       []float64 `json:"uv_index"`
		Code          []int     `json:"weather_code"`
	} `json:"hourly"`
	Daily struct {
		Sunset []string  `json:"sunset"`
		Time   []string  `json:"time"`
		High   []float64 `json:"temperature_2m_max"`
		Low    []float64 `json:"temperature_2m_min"`
		Code   []int     `json:"weather_code"`
	} `json:"daily"`
}

func normalize(raw forecastResponse, p Location, units string, tomorrow bool, hours int) model.Weather {
	loc, _ := time.LoadLocation(raw.Timezone)
	if loc == nil {
		loc = time.Local
	}
	validAt, _ := time.ParseInLocation("2006-01-02T15:04", raw.Current.Time, loc)
	idx := -1
	if tomorrow {
		target := time.Now().In(loc).AddDate(0, 0, 1)
		for i, s := range raw.Hourly.Time {
			t, _ := time.ParseInLocation("2006-01-02T15:04", s, loc)
			if sameDate(t, target) && t.Hour() >= 9 {
				idx = i
				break
			}
		}
	}
	temp, apparent, rain, code := raw.Current.Temperature, raw.Current.Apparent, raw.Current.Rain, raw.Current.Code
	if tomorrow && idx >= 0 && idx < len(raw.Hourly.Temperature) {
		validAt, _ = time.ParseInLocation("2006-01-02T15:04", raw.Hourly.Time[idx], loc)
		temp = raw.Hourly.Temperature[idx]
		apparent = valueAt(raw.Hourly.Apparent, idx, temp)
		raw.Current.Humidity = valueAt(raw.Hourly.Humidity, idx, raw.Current.Humidity)
		raw.Current.Cloud = valueAt(raw.Hourly.Cloud, idx, raw.Current.Cloud)
		raw.Current.Wind = valueAt(raw.Hourly.Wind, idx, raw.Current.Wind)
		raw.Current.WindDirection = valueAt(raw.Hourly.WindDirection, idx, raw.Current.WindDirection)
		raw.Current.Pressure = valueAt(raw.Hourly.Pressure, idx, raw.Current.Pressure)
		raw.Current.Visibility = valueAt(raw.Hourly.Visibility, idx, raw.Current.Visibility)
		raw.Current.UVIndex = valueAt(raw.Hourly.UVIndex, idx, raw.Current.UVIndex)
		if idx < len(raw.Hourly.Rain) {
			rain = raw.Hourly.Rain[idx]
		}
		if idx < len(raw.Hourly.Code) {
			code = raw.Hourly.Code[idx]
		}
	}
	w := model.Weather{Location: p.Name, Latitude: p.Latitude, Longitude: p.Longitude, Timezone: raw.Timezone, Temperature: temp, FeelsLike: apparent, Humidity: raw.Current.Humidity, Wind: raw.Current.Wind, WindDirection: raw.Current.WindDirection, CloudCover: raw.Current.Cloud, RainChance: rain, WeatherCode: code, Pressure: raw.Current.Pressure, Visibility: raw.Current.Visibility, UVIndex: raw.Current.UVIndex, Source: "Open-Meteo", ValidAt: validAt, FetchedAt: time.Now(), Units: units}
	w.Condition, _ = model.Condition(code)
	day := 0
	if tomorrow {
		day = 1
	}
	if day < len(raw.Daily.Sunset) {
		w.Sunset, _ = time.ParseInLocation("2006-01-02T15:04", raw.Daily.Sunset[day], loc)
	}
	for i, s := range raw.Daily.Time {
		d := model.Day{}
		d.Date, _ = time.ParseInLocation("2006-01-02", s, loc)
		if i < len(raw.Daily.High) {
			d.High = raw.Daily.High[i]
		}
		if i < len(raw.Daily.Low) {
			d.Low = raw.Daily.Low[i]
		}
		if i < len(raw.Daily.Code) {
			d.WeatherCode = raw.Daily.Code[i]
		}
		w.Days = append(w.Days, d)
	}
	start := time.Now().In(loc)
	if tomorrow {
		start = time.Date(start.Year(), start.Month(), start.Day()+1, 0, 0, 0, 0, loc)
	}
	if hours <= 0 {
		return w
	}
	limit := hours
	for i, s := range raw.Hourly.Time {
		t, e := time.ParseInLocation("2006-01-02T15:04", s, loc)
		if e == nil && !t.Before(start) && i < len(raw.Hourly.Temperature) {
			h := model.Hour{Time: t, Temperature: raw.Hourly.Temperature[i]}
			if i < len(raw.Hourly.Rain) {
				h.RainChance = raw.Hourly.Rain[i]
			}
			if i < len(raw.Hourly.Code) {
				h.WeatherCode = raw.Hourly.Code[i]
			}
			w.Hours = append(w.Hours, h)
			if len(w.Hours) >= limit {
				break
			}
		}
	}
	return w
}

func valueAt(values []float64, index int, fallback float64) float64 {
	if index >= 0 && index < len(values) {
		return values[index]
	}
	return fallback
}

type weatherAPICondition struct {
	Text string `json:"text"`
}

type weatherAPICurrent struct {
	LastUpdatedEpoch int64               `json:"last_updated_epoch"`
	TempC            float64             `json:"temp_c"`
	TempF            float64             `json:"temp_f"`
	FeelsC           float64             `json:"feelslike_c"`
	FeelsF           float64             `json:"feelslike_f"`
	Humidity         float64             `json:"humidity"`
	WindMPH          float64             `json:"wind_mph"`
	WindKPH          float64             `json:"wind_kph"`
	WindDegree       float64             `json:"wind_degree"`
	Cloud            float64             `json:"cloud"`
	PressureMB       float64             `json:"pressure_mb"`
	VisKM            float64             `json:"vis_km"`
	UV               float64             `json:"uv"`
	Condition        weatherAPICondition `json:"condition"`
}

type weatherAPIHour struct {
	TimeEpoch  int64               `json:"time_epoch"`
	Time       string              `json:"time"`
	TempC      float64             `json:"temp_c"`
	TempF      float64             `json:"temp_f"`
	FeelsC     float64             `json:"feelslike_c"`
	FeelsF     float64             `json:"feelslike_f"`
	Humidity   float64             `json:"humidity"`
	WindMPH    float64             `json:"wind_mph"`
	WindKPH    float64             `json:"wind_kph"`
	WindDegree float64             `json:"wind_degree"`
	Cloud      float64             `json:"cloud"`
	Rain       float64             `json:"chance_of_rain"`
	PressureMB float64             `json:"pressure_mb"`
	VisKM      float64             `json:"vis_km"`
	UV         float64             `json:"uv"`
	Condition  weatherAPICondition `json:"condition"`
}

type weatherAPIDay struct {
	Date string `json:"date"`
	Day  struct {
		HighC     float64             `json:"maxtemp_c"`
		HighF     float64             `json:"maxtemp_f"`
		LowC      float64             `json:"mintemp_c"`
		LowF      float64             `json:"mintemp_f"`
		Rain      float64             `json:"daily_chance_of_rain"`
		Condition weatherAPICondition `json:"condition"`
	} `json:"day"`
	Astro struct {
		Sunset string `json:"sunset"`
	} `json:"astro"`
	Hour []weatherAPIHour `json:"hour"`
}

type weatherAPIResponse struct {
	Location struct {
		Name      string  `json:"name"`
		Region    string  `json:"region"`
		Country   string  `json:"country"`
		Timezone  string  `json:"tz_id"`
		Latitude  float64 `json:"lat"`
		Longitude float64 `json:"lon"`
	} `json:"location"`
	Current  weatherAPICurrent `json:"current"`
	Forecast struct {
		Days []weatherAPIDay `json:"forecastday"`
	} `json:"forecast"`
}

func (c *Client) fetchWeatherAPI(ctx context.Context, query, key, units string, tomorrow bool, hours int) (model.Weather, error) {
	q := url.Values{"key": {key}, "q": {query}, "days": {"5"}, "aqi": {"no"}, "alerts": {"no"}}
	var raw weatherAPIResponse
	if err := c.getJSON(ctx, weatherURL+"?"+q.Encode(), &raw); err != nil {
		return model.Weather{}, err
	}
	return normalizeWeatherAPI(raw, units, tomorrow, hours), nil
}

func normalizeWeatherAPI(raw weatherAPIResponse, units string, tomorrow bool, hours int) model.Weather {
	loc, err := time.LoadLocation(raw.Location.Timezone)
	if err != nil {
		loc = time.Local
	}
	countryCode := ""
	country := strings.ToLower(raw.Location.Country)
	if country == "usa" || strings.Contains(country, "united states") {
		countryCode = "US"
	}
	code := weatherCode(raw.Current.Condition.Text)
	temp, feels, wind := raw.Current.TempF, raw.Current.FeelsF, raw.Current.WindMPH
	if units == "metric" {
		temp, feels, wind = raw.Current.TempC, raw.Current.FeelsC, raw.Current.WindKPH
	}
	w := model.Weather{
		Location:      formatLocation(raw.Location.Name, raw.Location.Region, raw.Location.Country, countryCode),
		Latitude:      raw.Location.Latitude,
		Longitude:     raw.Location.Longitude,
		Timezone:      raw.Location.Timezone,
		Temperature:   temp,
		FeelsLike:     feels,
		Humidity:      raw.Current.Humidity,
		Wind:          wind,
		WindDirection: raw.Current.WindDegree,
		CloudCover:    raw.Current.Cloud,
		Pressure:      raw.Current.PressureMB,
		Visibility:    raw.Current.VisKM * 1000,
		UVIndex:       raw.Current.UV,
		WeatherCode:   code,
		Source:        "WeatherAPI",
		FetchedAt:     time.Now(),
		Units:         units,
	}
	if raw.Current.LastUpdatedEpoch > 0 {
		w.ValidAt = time.Unix(raw.Current.LastUpdatedEpoch, 0).In(loc)
	}
	dayIndex := 0
	if tomorrow {
		dayIndex = 1
	}
	if dayIndex < len(raw.Forecast.Days) {
		selected := raw.Forecast.Days[dayIndex]
		w.RainChance = selected.Day.Rain
		for _, h := range selected.Hour {
			t := weatherAPITime(h, loc)
			if t.Hour() == 9 {
				applyWeatherAPIHour(&w, h, units)
				w.ValidAt = t
				break
			}
		}
		if d, e := time.ParseInLocation("2006-01-02", selected.Date, loc); e == nil {
			w.Sunset, _ = time.ParseInLocation("2006-01-02 3:04 PM", d.Format("2006-01-02")+" "+selected.Astro.Sunset, loc)
		}
	}
	w.Condition, _ = model.Condition(w.WeatherCode)
	for _, d := range raw.Forecast.Days {
		day := model.Day{WeatherCode: weatherCode(d.Day.Condition.Text)}
		day.Date, _ = time.ParseInLocation("2006-01-02", d.Date, loc)
		if units == "metric" {
			day.High, day.Low = d.Day.HighC, d.Day.LowC
		} else {
			day.High, day.Low = d.Day.HighF, d.Day.LowF
		}
		w.Days = append(w.Days, day)
	}
	start := time.Now().In(loc)
	if tomorrow {
		start = time.Date(start.Year(), start.Month(), start.Day()+1, 0, 0, 0, 0, loc)
	}
	if hours <= 0 {
		return w
	}
	limit := hours
	for _, d := range raw.Forecast.Days {
		for _, h := range d.Hour {
			t := weatherAPITime(h, loc)
			if t.Before(start) {
				continue
			}
			temp := h.TempF
			if units == "metric" {
				temp = h.TempC
			}
			w.Hours = append(w.Hours, model.Hour{Time: t, Temperature: temp, RainChance: h.Rain, WeatherCode: weatherCode(h.Condition.Text)})
			if len(w.Hours) >= limit {
				return w
			}
		}
	}
	return w
}

func applyWeatherAPIHour(w *model.Weather, h weatherAPIHour, units string) {
	w.Temperature, w.FeelsLike, w.Wind = h.TempF, h.FeelsF, h.WindMPH
	if units == "metric" {
		w.Temperature, w.FeelsLike, w.Wind = h.TempC, h.FeelsC, h.WindKPH
	}
	w.Humidity, w.WindDirection, w.CloudCover = h.Humidity, h.WindDegree, h.Cloud
	w.RainChance, w.Pressure, w.Visibility, w.UVIndex = h.Rain, h.PressureMB, h.VisKM*1000, h.UV
	w.WeatherCode = weatherCode(h.Condition.Text)
}

func weatherAPITime(h weatherAPIHour, loc *time.Location) time.Time {
	if h.TimeEpoch > 0 {
		return time.Unix(h.TimeEpoch, 0).In(loc)
	}
	t, _ := time.ParseInLocation("2006-01-02 15:04", h.Time, loc)
	return t
}

func weatherCode(condition string) int {
	s := strings.ToLower(condition)
	switch {
	case strings.Contains(s, "thunder"):
		return 95
	case strings.Contains(s, "snow") || strings.Contains(s, "sleet") || strings.Contains(s, "ice"):
		return 75
	case strings.Contains(s, "shower"):
		return 80
	case strings.Contains(s, "rain"):
		return 63
	case strings.Contains(s, "drizzle"):
		return 53
	case strings.Contains(s, "fog") || strings.Contains(s, "mist"):
		return 45
	case strings.Contains(s, "partly"):
		return 2
	case strings.Contains(s, "cloud") || strings.Contains(s, "overcast"):
		return 3
	default:
		return 0
	}
}

func (c *Client) Alerts(ctx context.Context, lat, lon float64) ([]model.Alert, error) {
	u := "https://api.weather.gov/alerts/active?point=" + strconv.FormatFloat(lat, 'f', 4, 64) + "," + strconv.FormatFloat(lon, 'f', 4, 64)
	var r struct {
		Features []struct {
			Properties struct {
				Event       string    `json:"event"`
				Severity    string    `json:"severity"`
				Description string    `json:"description"`
				Onset       time.Time `json:"onset"`
				Ends        time.Time `json:"ends"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := c.getJSON(ctx, u, &r); err != nil {
		return nil, err
	}
	out := make([]model.Alert, 0, len(r.Features))
	for _, f := range r.Features {
		p := f.Properties
		out = append(out, model.Alert{Event: p.Event, Severity: p.Severity, Starts: p.Onset, Ends: p.Ends, Description: p.Description})
	}
	return out, nil
}
func (c *Client) getJSON(ctx context.Context, endpoint string, dst any) error {
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "das-wetter/2.0 (terminal weather)")
		resp, lastErr = c.HTTP.Do(req)
		if lastErr == nil {
			break
		}
		if ctx.Err() != nil {
			return lastErr
		}
	}
	if lastErr != nil {
		return lastErr
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("service returned %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("invalid service response: %w", err)
	}
	return nil
}

func formatLocation(city, region, country, code string) string {
	parts := []string{city}
	if code == "US" {
		region = stateCode(region)
		country = ""
	}
	if region != "" {
		parts = append(parts, region)
	}
	if country != "" {
		parts = append(parts, country)
	}
	return strings.Join(parts, ", ")
}
func stateCode(name string) string {
	if v, ok := usStates[name]; ok {
		return v
	}
	return name
}
func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

var usStates = map[string]string{"Alabama": "AL", "Alaska": "AK", "Arizona": "AZ", "Arkansas": "AR", "California": "CA", "Colorado": "CO", "Connecticut": "CT", "Delaware": "DE", "Florida": "FL", "Georgia": "GA", "Hawaii": "HI", "Idaho": "ID", "Illinois": "IL", "Indiana": "IN", "Iowa": "IA", "Kansas": "KS", "Kentucky": "KY", "Louisiana": "LA", "Maine": "ME", "Maryland": "MD", "Massachusetts": "MA", "Michigan": "MI", "Minnesota": "MN", "Mississippi": "MS", "Missouri": "MO", "Montana": "MT", "Nebraska": "NE", "Nevada": "NV", "New Hampshire": "NH", "New Jersey": "NJ", "New Mexico": "NM", "New York": "NY", "North Carolina": "NC", "North Dakota": "ND", "Ohio": "OH", "Oklahoma": "OK", "Oregon": "OR", "Pennsylvania": "PA", "Rhode Island": "RI", "South Carolina": "SC", "South Dakota": "SD", "Tennessee": "TN", "Texas": "TX", "Utah": "UT", "Vermont": "VT", "Virginia": "VA", "Washington": "WA", "West Virginia": "WV", "Wisconsin": "WI", "Wyoming": "WY", "District of Columbia": "DC"}

var locationAliases = map[string]string{
	"atl": "Atlanta",
	"la":  "Los Angeles",
	"nyc": "New York",
	"sf":  "San Francisco",
}
