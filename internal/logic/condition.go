// Package logic evaluates scriptable weather conditions and astro heuristics.
package logic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"daswetter/internal/model"
)

var expression = regexp.MustCompile(`^\s*([a-zA-Z_]+)\s*(<=|>=|==|!=|<|>)\s*(-?\d+(?:\.\d+)?)\s*$`)

func Evaluate(expr string, w model.Weather) (bool, error) {
	m := expression.FindStringSubmatch(expr)
	if m == nil {
		return false, fmt.Errorf("invalid condition %q (example: rain > 50)", expr)
	}
	var value float64
	switch strings.ToLower(m[1]) {
	case "rain", "precipitation":
		value = w.RainChance
	case "temp", "temperature":
		value = w.Temperature
	case "feels", "feels_like":
		value = w.FeelsLike
	case "humidity":
		value = w.Humidity
	case "wind":
		value = w.Wind
	case "cloud", "cloud_cover":
		value = w.CloudCover
	default:
		return false, fmt.Errorf("unknown condition field %q", m[1])
	}
	want, _ := strconv.ParseFloat(m[3], 64)
	switch m[2] {
	case ">":
		return value > want, nil
	case "<":
		return value < want, nil
	case ">=":
		return value >= want, nil
	case "<=":
		return value <= want, nil
	case "==":
		return value == want, nil
	case "!=":
		return value != want, nil
	}
	return false, nil
}

func Astro(w *model.Weather) {
	days := float64(w.FetchedAt.Unix()-947182440) / 86400
	phase := int(days/29.530588*8+0.5) & 7
	names := []string{"New Moon", "Waxing Crescent", "First Quarter", "Waxing Gibbous", "Full Moon", "Waning Gibbous", "Last Quarter", "Waning Crescent"}
	w.MoonPhase = names[phase]
	score := 100 - w.CloudCover - w.Humidity*.2 - w.Wind*.5
	switch {
	case score >= 65:
		w.Viewing = "Excellent"
	case score >= 40:
		w.Viewing = "Fair"
	default:
		w.Viewing = "Poor"
	}
}
