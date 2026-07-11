// Package cache stores the most recent successful weather responses on disk.
package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"daswetter/internal/model"
)

type Store struct {
	Path string
	TTL  time.Duration
}

func Default() Store {
	h, _ := os.UserHomeDir()
	return Store{Path: filepath.Join(h, ".weather", "cache.json"), TTL: 10 * time.Minute}
}

type fileData struct {
	Entries        map[string]model.Weather `json:"entries"`
	LastLocation   string                   `json:"last_location"`
	LastIPLocation string                   `json:"last_ip_location,omitempty"`
}

func (s Store) read() (fileData, error) {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return fileData{}, err
	}
	var d fileData
	if err := json.Unmarshal(b, &d); err != nil {
		return d, err
	}
	if d.Entries == nil {
		d.Entries = map[string]model.Weather{}
	}
	return d, nil
}

func key(location, units string) string {
	return strings.ToLower(strings.TrimSpace(location)) + "|" + units
}

func (s Store) Get(location, units string, allowStale bool) (model.Weather, error) {
	d, err := s.read()
	if err != nil {
		return model.Weather{}, err
	}
	w, ok := d.Entries[key(location, units)]
	if !ok {
		return w, errors.New("not cached")
	}
	if w.FetchedAt.IsZero() {
		return model.Weather{}, errors.New("cache schema outdated")
	}
	if !allowStale && time.Since(w.FetchedAt) > s.TTL {
		return w, errors.New("cache expired")
	}
	w.Stale = time.Since(w.FetchedAt) > s.TTL
	return w, nil
}

func (s Store) LastLocation() string { d, _ := s.read(); return d.LastLocation }

func (s Store) LastIPLocation() string { d, _ := s.read(); return d.LastIPLocation }

func (s Store) RememberIPLocation(location string) error {
	d, _ := s.read()
	if d.Entries == nil {
		d.Entries = map[string]model.Weather{}
	}
	d.LastIPLocation = location
	return s.write(d)
}

func (s Store) Put(query string, w model.Weather) error {
	return s.PutFor(query, query, w)
}

func (s Store) PutFor(cacheKey, lastLocation string, w model.Weather) error {
	d, _ := s.read()
	if d.Entries == nil {
		d.Entries = map[string]model.Weather{}
	}
	d.Entries[key(cacheKey, w.Units)] = w
	d.LastLocation = lastLocation
	return s.write(d)
}

func (s Store) write(d fileData) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
