// Package render formats normalized weather data for terminals and scripts.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"daswetter/internal/model"
)

type palette struct {
	reset, boldCode, mutedCode, blueCode, redCode, yellowCode, cyanCode, greenCode string
}

func newPalette(out io.Writer) palette {
	if !colorEnabled(out) {
		return palette{}
	}
	return palette{
		reset:      "\x1b[0m",
		boldCode:   "\x1b[1m",
		mutedCode:  "\x1b[90m",
		blueCode:   "\x1b[34m",
		redCode:    "\x1b[31m",
		yellowCode: "\x1b[33m",
		cyanCode:   "\x1b[36m",
		greenCode:  "\x1b[32m",
	}
}

func (p palette) paint(code, text string) string {
	if code == "" || text == "" {
		return text
	}
	return code + text + p.reset
}

func (p palette) bold(text string) string   { return p.paint(p.boldCode, text) }
func (p palette) muted(text string) string  { return p.paint(p.mutedCode, text) }
func (p palette) blue(text string) string   { return p.paint(p.blueCode, text) }
func (p palette) red(text string) string    { return p.paint(p.redCode, text) }
func (p palette) yellow(text string) string { return p.paint(p.yellowCode, text) }
func (p palette) cyan(text string) string   { return p.paint(p.cyanCode, text) }
func (p palette) green(text string) string  { return p.paint(p.greenCode, text) }

func (p palette) blueBold(text string) string {
	return p.paint(p.blueCode+p.boldCode, text)
}

func colorEnabled(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func contentWidth() int {
	width := 80
	if value, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && value > 0 {
		width = value
	}
	if width < 40 {
		width = 40
	}
	if width > 100 {
		width = 100
	}
	return width - 2
}

func padVisible(value string, width int) string {
	if missing := width - visibleWidth(value); missing > 0 {
		return value + strings.Repeat(" ", missing)
	}
	return value
}

func visibleWidth(value string) int {
	width := 0
	inEscape := false
	for _, r := range value {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		width++
	}
	return width
}

func localFetched(w model.Weather) time.Time {
	stamp := w.ValidAt
	if stamp.IsZero() {
		stamp = w.FetchedAt
	}
	if stamp.IsZero() {
		return time.Time{}
	}
	if w.Timezone == "" {
		return stamp
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		return stamp
	}
	return stamp.In(loc)
}

func Current(out io.Writer, w model.Weather, hours int, astro bool) {
	colors := newPalette(out)
	tempUnit, windUnit := "°F", "mph"
	if w.Units == "metric" {
		tempUnit, windUnit = "°C", "km/h"
	}
	condition, icon := model.Condition(w.WeatherCode)
	fmt.Fprintf(out, "%s %s %s\n\n", colors.green("›"), colors.cyan("~"), colors.bold("das wetter"))
	fmt.Fprintln(out, colors.bold(w.Location))
	if stamp := localFetched(w); !stamp.IsZero() {
		line := stamp.Format("Monday, January 2, 2006 3:04 PM")
		if w.Source != "" {
			line += " · " + w.Source
		}
		fmt.Fprintln(out, colors.muted(line))
	}
	fmt.Fprintln(out)

	left := []string{
		fmt.Sprintf("%s %s", icon, colors.bold(fmt.Sprintf("%.0f%s", w.Temperature, tempUnit))),
		fmt.Sprintf("  %s", colors.blue(condition)),
		fmt.Sprintf("  Feels like %.0f%s", w.FeelsLike, tempUnit),
	}
	right := []string{
		fmt.Sprintf("💨 Wind: %.0f %s", w.Wind, windUnit),
		fmt.Sprintf("💧 Humidity: %.0f%%", w.Humidity),
		fmt.Sprintf("☁ Cloud cover: %.0f%%", w.CloudCover),
	}
	if !w.Sunset.IsZero() {
		right = append(right, fmt.Sprintf("🌅 Sunset: %s", w.Sunset.Format("3:04 PM")))
	}
	if astro {
		right = append(right, fmt.Sprintf("🌙 Moon: %s", w.MoonPhase), fmt.Sprintf("🔭 Viewing: %s", w.Viewing))
	}
	for i := 0; i < max(len(left), len(right)); i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		fmt.Fprintf(out, "  %s %s %s\n", padVisible(l, 31), colors.muted("│"), r)
	}
	fmt.Fprintln(out, colors.muted(strings.Repeat("─", contentWidth())))

	if hours > 0 && len(w.Hours) > 0 {
		fmt.Fprintln(out, colors.blueBold("Hourly"))
		limit := min(hours, len(w.Hours))
		for _, h := range w.Hours[:limit] {
			fmt.Fprintf(out, "%s  %4.0f%s  rain %3.0f%%\n", h.Time.Format("Mon 3 PM"), h.Temperature, tempUnit, h.RainChance)
		}
	}

	for _, a := range w.Alerts {
		span := ""
		if !a.Starts.IsZero() && !a.Ends.IsZero() {
			span = " (" + a.Starts.Format("3 PM") + "–" + a.Ends.Format("3 PM") + ")"
		}
		fmt.Fprintf(out, "%s %s%s\n", colors.yellow("⚠"), colors.yellow(a.Event), span)
	}

	if len(w.Days) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, colors.blueBold("Forecast"))
		for _, d := range w.Days[:min(5, len(w.Days))] {
			dayName := "---"
			if !d.Date.IsZero() {
				dayName = d.Date.Format("Mon")
			}
			_, dayIcon := model.Condition(d.WeatherCode)
			description, _ := model.Condition(d.WeatherCode)
			high := colors.red(fmt.Sprintf("%3.0f%s", d.High, tempUnit))
			low := colors.blue(fmt.Sprintf("%3.0f%s", d.Low, tempUnit))
			fmt.Fprintf(out, "  %-3s  %s  %s  %s  %s\n", dayName, dayIcon, high, low, description)
		}
		fmt.Fprintln(out, colors.muted(strings.Repeat("─", contentWidth())))
	}

	if w.Stale {
		fmt.Fprintln(out, colors.muted("(cached; live service unavailable)"))
	}
	fmt.Fprintf(out, "\n%s\n", colors.muted("Tip: weather --json · weather --hours 6 · weather tomorrow · weather help · weather quit"))
}

func Compare(out io.Writer, ws []model.Weather) {
	colors := newPalette(out)
	for _, w := range ws {
		unit := "°F"
		if w.Units == "metric" {
			unit = "°C"
		}
		_, icon := model.Condition(w.WeatherCode)
		temp := colors.red(fmt.Sprintf("%3.0f%s", w.Temperature, unit))
		fmt.Fprintf(out, "%-10s %s %s\n", colors.bold(shortLocation(w.Location)), temp, icon)
	}
}

func Graph(out io.Writer, w model.Weather) {
	h := w.Hours
	if len(h) > 12 {
		h = h[:12]
	}
	if len(h) == 0 {
		return
	}
	minT, maxT := h[0].Temperature, h[0].Temperature
	for _, v := range h {
		minT = math.Min(minT, v.Temperature)
		maxT = math.Max(maxT, v.Temperature)
	}
	span := maxT - minT
	if span < 1 {
		span = 1
	}
	plotWidth := graphWidth(len(h))
	for row := 7; row >= 0; row-- {
		level := minT + span*float64(row)/7
		line := []rune(strings.Repeat(" ", plotWidth))
		for i, v := range h {
			pos := int(math.Round((v.Temperature - minT) / span * 7))
			if pos == row {
				x := 0
				if len(h) > 1 {
					x = int(math.Round(float64(i) * float64(plotWidth-1) / float64(len(h)-1)))
				}
				line[x] = '*'
			}
		}
		fmt.Fprintf(out, "%4.0f|%s\n", level, string(line))
	}
	fmt.Fprint(out, "    +", strings.Repeat("-", plotWidth), "\n     ")
	labels := []rune(strings.Repeat(" ", plotWidth))
	lastLabelEnd := -2
	for i, v := range h {
		x := 0
		if len(h) > 1 {
			x = int(math.Round(float64(i) * float64(plotWidth-1) / float64(len(h)-1)))
		}
		label := []rune(v.Time.Format("15"))
		if x > lastLabelEnd+1 && x+len(label) <= len(labels) {
			copy(labels[x:], label)
			lastLabelEnd = x + len(label) - 1
		}
	}
	fmt.Fprintln(out, string(labels), "hour")
}

func graphWidth(samples int) int {
	width := 80
	if value, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && value > 0 {
		width = value
	}
	width -= 10 // y-axis label, divider, and the hour legend
	if width < 12 {
		width = 12
	}
	if maxWidth := samples * 3; width > maxWidth {
		width = maxWidth
	}
	return width
}

func JSONWeather(out io.Writer, ws []model.Weather) error {
	values := make([]map[string]any, 0, len(ws))
	for _, w := range ws {
		v := map[string]any{"location": w.Location, "temperature": w.Temperature, "feels_like": w.FeelsLike, "humidity": w.Humidity, "cloud_cover": w.CloudCover, "rain_chance": w.RainChance, "condition": w.Condition, "units": w.Units, "fetched_at": w.FetchedAt}
		if w.Source != "" {
			v["source"] = w.Source
		}
		if !w.ValidAt.IsZero() {
			v["valid_at"] = w.ValidAt
		}
		if w.Units == "metric" {
			v["wind_kph"] = w.Wind
		} else {
			v["wind_mph"] = w.Wind
		}
		if len(w.Hours) > 0 {
			v["hours"] = w.Hours
		}
		if len(w.Alerts) > 0 {
			v["alerts"] = w.Alerts
		}
		if w.Stale {
			v["stale"] = true
		}
		values = append(values, v)
	}
	e := json.NewEncoder(out)
	if len(values) == 1 {
		return e.Encode(values[0])
	}
	return e.Encode(values)
}

func Clear(out io.Writer) { fmt.Fprint(out, "\033[H\033[2J") }

func shortLocation(location string) string {
	city := strings.TrimSpace(strings.Split(location, ",")[0])
	switch strings.ToLower(city) {
	case "new york":
		return "NYC"
	case "los angeles":
		return "LA"
	}
	r := []rune(strings.ToUpper(city))
	if len(r) > 3 {
		return string(r[:3])
	}
	return string(r)
}
