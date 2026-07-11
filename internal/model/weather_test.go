package model

import "testing"

func TestConditionWMOCodeMapping(t *testing.T) {
	tests := map[int]string{
		0:  "Clear",
		1:  "Mainly Clear",
		2:  "Partly Cloudy",
		3:  "Overcast",
		45: "Fog",
		53: "Drizzle",
		63: "Rain",
		75: "Snow",
		80: "Showers",
		95: "Thunderstorm",
	}
	for code, want := range tests {
		got, _ := Condition(code)
		if got != want {
			t.Errorf("Condition(%d) = %q, want %q", code, got, want)
		}
	}
}
