package cache

import (
	"path/filepath"
	"testing"
	"time"

	"daswetter/internal/model"
)

func TestRoundTrip(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "cache.json"), TTL: time.Minute}
	w := model.Weather{Location: "Atlanta, Georgia, United States", Temperature: 82, Pressure: 1013, Days: []model.Day{{High: 82, Low: 70}}, Units: "imperial", FetchedAt: time.Now()}
	if err := s.Put("atl", w); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("ATL", "imperial", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Temperature != 82 || s.LastLocation() != "atl" {
		t.Fatalf("unexpected cache result: %+v", got)
	}
}

func TestStaleFallback(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "cache.json"), TTL: time.Minute}
	w := model.Weather{Location: "Atlanta, GA", Temperature: 82, Units: "imperial", FetchedAt: time.Now().Add(-2 * time.Minute)}
	if err := s.Put("atl", w); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("atl", "imperial", false); err == nil {
		t.Fatal("expired entry was returned as fresh")
	}
	got, err := s.Get("atl", "imperial", true)
	if err != nil || !got.Stale || got.Temperature != 82 {
		t.Fatalf("stale fallback failed: %+v, %v", got, err)
	}
}

func TestIPLocationIsTrackedSeparately(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "cache.json"), TTL: time.Minute}
	w := model.Weather{Location: "Atlanta, GA", Units: "imperial", FetchedAt: time.Now()}
	if err := s.PutFor("atl", "Atlanta", w); err != nil {
		t.Fatal(err)
	}
	if err := s.RememberIPLocation("Lilburn, GA"); err != nil {
		t.Fatal(err)
	}
	if s.LastLocation() != "Atlanta" || s.LastIPLocation() != "Lilburn, GA" {
		t.Fatalf("locations were conflated: last=%q ip=%q", s.LastLocation(), s.LastIPLocation())
	}
}
