// Package config reads the optional per-user weather configuration.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Config struct{ DefaultLocation, Units string }

func Load() Config {
	c := Config{Units: "imperial"}
	home, err := os.UserHomeDir()
	if err != nil {
		return c
	}
	f, err := os.Open(filepath.Join(home, ".weatherconfig"))
	if err != nil {
		return c
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "default_location":
			c.DefaultLocation = strings.TrimSpace(v)
		case "units":
			v = strings.TrimSpace(v)
			if v == "metric" || v == "imperial" {
				c.Units = v
			}
		}
	}
	return c
}
