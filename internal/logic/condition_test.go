package logic

import (
	"testing"

	"daswetter/internal/model"
)

func TestEvaluate(t *testing.T) {
	w := model.Weather{Temperature: 31, RainChance: 60, Humidity: 70}
	for _, tc := range []struct {
		expr string
		want bool
	}{{"rain > 50", true}, {"temp < 32", true}, {"humidity == 50", false}} {
		got, err := Evaluate(tc.expr, w)
		if err != nil || got != tc.want {
			t.Fatalf("Evaluate(%q) = %v, %v; want %v", tc.expr, got, err, tc.want)
		}
	}
}

func TestEvaluateRejectsUnknownField(t *testing.T) {
	if _, err := Evaluate("pressure > 1", model.Weather{}); err == nil {
		t.Fatal("expected error")
	}
}
