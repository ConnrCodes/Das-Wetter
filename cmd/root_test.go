package cmd

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"daswetter/internal/cache"
	"daswetter/internal/model"
)

func TestNormalizeCommandArgs(t *testing.T) {
	o := options{units: "imperial"}
	args, handled, err := normalizeCommandArgs(newRoot(), []string{"atl", "hours", "6", "tomorrow", "metric", "graph"}, &o)
	if err != nil || handled {
		t.Fatalf("normalizeCommandArgs returned handled=%v err=%v", handled, err)
	}
	if len(args) != 1 || args[0] != "atl" {
		t.Fatalf("locations = %#v, want [atl]", args)
	}
	if o.hours != 6 || !o.tomorrow || !o.graph || o.units != "metric" {
		t.Fatalf("options were not normalized: %+v", o)
	}
}

func TestNormalizeCommandArgsQuit(t *testing.T) {
	o := options{units: "imperial"}
	_, handled, err := normalizeCommandArgs(newRoot(), []string{"quit"}, &o)
	if err != nil || !handled {
		t.Fatalf("quit was not handled: handled=%v err=%v", handled, err)
	}
}

func TestNormalizeCommandArgsHoursValidation(t *testing.T) {
	o := options{units: "imperial"}
	if _, _, err := normalizeCommandArgs(newRoot(), []string{"hours", "nope"}, &o); err == nil {
		t.Fatal("invalid hours command was accepted")
	}
}

func TestSplitInput(t *testing.T) {
	got, err := splitInput(`--if "rain > 50" "Atlanta, GA"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--if", "rain > 50", "Atlanta, GA"}
	if len(got) != len(want) {
		t.Fatalf("splitInput = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitInput[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPinSessionLocation(t *testing.T) {
	if got := pinSessionLocation([]string{"--json"}, "Lilburn, GA"); len(got) != 2 || got[0] != "Lilburn, GA" {
		t.Fatalf("flag command was not pinned: %#v", got)
	}
	if got := pinSessionLocation([]string{"hours", "6"}, "Lilburn, GA"); len(got) != 3 || got[0] != "Lilburn, GA" {
		t.Fatalf("hours command was not pinned: %#v", got)
	}
	if got := pinSessionLocation([]string{"Atlanta, GA"}, "Lilburn, GA"); len(got) != 1 || got[0] != "Atlanta, GA" {
		t.Fatalf("explicit location was overwritten: %#v", got)
	}
}

func TestFetchLocationPrefersLiveData(t *testing.T) {
	store := cache.Store{Path: filepath.Join(t.TempDir(), "cache.json"), TTL: time.Hour}
	cached := model.Weather{Temperature: 70, Units: "imperial", FetchedAt: time.Now()}
	if err := store.PutFor("atl", "atl", cached); err != nil {
		t.Fatal(err)
	}
	got, err := fetchLocation(store, "atl", "atl", options{units: "imperial"}, func() (model.Weather, error) {
		return model.Weather{Temperature: 82, Units: "imperial", FetchedAt: time.Now()}, nil
	})
	if err != nil || got.Temperature != 82 {
		t.Fatalf("live data was not preferred: %+v, %v", got, err)
	}
}

func TestFetchLocationMarksCacheFallback(t *testing.T) {
	store := cache.Store{Path: filepath.Join(t.TempDir(), "cache.json"), TTL: time.Hour}
	cached := model.Weather{Temperature: 70, Units: "imperial", FetchedAt: time.Now()}
	if err := store.PutFor("atl", "atl", cached); err != nil {
		t.Fatal(err)
	}
	got, err := fetchLocation(store, "atl", "atl", options{units: "imperial"}, func() (model.Weather, error) {
		return model.Weather{}, errors.New("offline")
	})
	if err != nil || !got.Stale || got.Temperature != 70 {
		t.Fatalf("cache fallback was not disclosed: %+v, %v", got, err)
	}
}
