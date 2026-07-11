// Package cmd defines the weather command and its one-shot/watch execution.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"daswetter/internal/api"
	"daswetter/internal/cache"
	"daswetter/internal/config"
	"daswetter/internal/logic"
	"daswetter/internal/model"
	"daswetter/internal/render"
	"github.com/spf13/cobra"
)

type options struct {
	json, tomorrow, alerts, compare, graph, astro bool
	hours, watch                                  int
	units, condition                              string
}

type exitError struct{ code int }

func (e *exitError) Error() string { return "condition false" }

func Execute() {
	root := newRoot()
	if err := root.Execute(); err != nil {
		var e *exitError
		if errors.As(err, &e) {
			os.Exit(e.code)
		}
		fmt.Fprintln(os.Stderr, "weather:", err)
		os.Exit(2)
	}
}

func newRoot() *cobra.Command {
	cfg := config.Load()
	o := options{units: cfg.Units}
	cmd := &cobra.Command{Use: "weather [location ...]", Short: "Fast, clean, scriptable terminal weather", Args: cobra.ArbitraryArgs, SilenceUsage: true, SilenceErrors: true, RunE: func(cmd *cobra.Command, args []string) error { return run(cmd.Context(), cmd, args, o, cfg) }}
	f := cmd.Flags()
	f.BoolVar(&o.json, "json", false, "output structured JSON only")
	f.BoolVar(&o.tomorrow, "tomorrow", false, "show tomorrow's forecast")
	f.IntVar(&o.hours, "hours", 0, "show the next N hours")
	f.BoolVar(&o.alerts, "alerts", false, "include active US weather alerts")
	f.IntVar(&o.watch, "watch", 0, "refresh every N seconds")
	f.StringVar(&o.units, "units", o.units, "units: imperial or metric")
	f.BoolVar(&o.compare, "compare", false, "compare multiple locations")
	f.BoolVar(&o.graph, "graph", false, "show a 12-hour ASCII temperature graph")
	f.StringVar(&o.condition, "if", "", "exit 0 when an expression is true, otherwise 1")
	f.BoolVar(&o.astro, "astro", false, "show astronomy viewing conditions")
	return cmd
}

func run(parent context.Context, cmd *cobra.Command, args []string, o options, cfg config.Config) error {
	var handled bool
	var err error
	args, handled, err = normalizeCommandArgs(cmd, args, &o)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	if o.units != "imperial" && o.units != "metric" {
		return fmt.Errorf("--units must be imperial or metric")
	}
	if o.hours < 0 || o.watch < 0 {
		return fmt.Errorf("--hours and --watch cannot be negative")
	}
	if o.json && o.watch > 0 {
		return fmt.Errorf("--json cannot be combined with --watch")
	}
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()
	for {
		if ctx.Err() != nil {
			return nil
		}
		ws, err := fetchAll(ctx, args, o, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		if o.watch > 0 {
			render.Clear(cmd.OutOrStdout())
		}
		if o.json {
			if err := render.JSONWeather(cmd.OutOrStdout(), ws); err != nil {
				return err
			}
		} else if o.compare || len(ws) > 1 {
			render.Compare(cmd.OutOrStdout(), ws)
		} else {
			render.Current(cmd.OutOrStdout(), ws[0], o.hours, o.astro)
			if o.graph {
				fmt.Fprintln(cmd.OutOrStdout())
				render.Graph(cmd.OutOrStdout(), ws[0])
			}
		}
		if o.condition != "" {
			ok, e := logic.Evaluate(o.condition, ws[0])
			if e != nil {
				return e
			}
			if !ok {
				return &exitError{1}
			}
		}
		if o.watch == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Duration(o.watch) * time.Second):
		}
	}
}

// normalizeCommandArgs keeps the one-shot flag interface while accepting the
// short command words people naturally type from the terminal tip, such as
// `weather hours 6` or `weather tomorrow`.
func normalizeCommandArgs(cmd *cobra.Command, args []string, o *options) ([]string, bool, error) {
	locations := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		word := strings.ToLower(strings.TrimSpace(args[i]))
		switch word {
		case "help":
			if len(args) != 1 {
				locations = append(locations, args[i])
				continue
			}
			return nil, true, cmd.Help()
		case "quit", "exit":
			if len(args) == 1 {
				return nil, true, nil
			}
			locations = append(locations, args[i])
		case "refresh":
			// A fresh one-shot request is already the default behavior.
		case "tomorrow":
			o.tomorrow = true
		case "alerts":
			o.alerts = true
		case "graph":
			o.graph = true
		case "astro":
			o.astro = true
		case "json":
			o.json = true
		case "metric":
			o.units = "metric"
		case "imperial":
			o.units = "imperial"
		case "hours":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("hours requires a number, for example: weather hours 6")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 {
				return nil, false, fmt.Errorf("hours requires a non-negative number")
			}
			o.hours = n
			i++
		case "watch":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("watch requires a number, for example: weather watch 30")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 {
				return nil, false, fmt.Errorf("watch requires a non-negative number")
			}
			o.watch = n
			i++
		default:
			locations = append(locations, args[i])
		}
	}
	return locations, false, nil
}

func fetchAll(ctx context.Context, args []string, o options, cfg config.Config) ([]model.Weather, error) {
	client, store := api.New(), cache.Default()
	if len(args) == 0 {
		// Resolve the current public-IP location first so a bare `weather`
		// command follows the network the user is currently on.
		loc, locationErr := client.IPLocation(ctx)
		if locationErr == nil {
			w, weatherErr := fetchLocation(store, cacheKey(loc.Name, o), loc.Name, o, func() (model.Weather, error) {
				return client.FetchLocation(ctx, loc, o.units, o.tomorrow, forecastHours(o))
			})
			if weatherErr == nil {
				_ = store.RememberIPLocation(loc.Name)
				return enrich(ctx, client, []model.Weather{w}, o), nil
			}
			locationErr = weatherErr
		}
		last := store.LastIPLocation()
		if last == "" {
			last = cfg.DefaultLocation
		}
		if last != "" {
			args = []string{last}
		} else {
			return nil, fmt.Errorf("could not determine location from IP: %w", locationErr)
		}
	}
	ws := make([]model.Weather, 0, len(args))
	for _, query := range args {
		q := query
		w, err := fetchLocation(store, cacheKey(q, o), q, o, func() (model.Weather, error) {
			return client.Fetch(ctx, q, o.units, o.tomorrow, forecastHours(o))
		})
		if err != nil {
			return nil, err
		}
		ws = append(ws, w)
	}
	return enrich(ctx, client, ws, o), nil
}

func fetchLocation(store cache.Store, key, last string, o options, live func() (model.Weather, error)) (model.Weather, error) {
	w, liveErr := live()
	if liveErr == nil {
		_ = store.PutFor(key, last, w)
		return w, nil
	}
	w, cacheErr := store.Get(key, o.units, false)
	if cacheErr == nil {
		w.Stale = true
		return w, nil
	}
	return model.Weather{}, fmt.Errorf("%s: live weather unavailable (%v) and no recent cached response is available", last, liveErr)
}

func enrich(ctx context.Context, client *api.Client, ws []model.Weather, o options) []model.Weather {
	for i := range ws {
		if o.alerts {
			if a, e := client.Alerts(ctx, ws[i].Latitude, ws[i].Longitude); e == nil {
				ws[i].Alerts = a
			}
		}
		if o.astro {
			logic.Astro(&ws[i])
		}
	}
	return ws
}

func cacheKey(query string, o options) string {
	key := query
	if o.tomorrow {
		key += "::tomorrow"
	}
	if n := forecastHours(o); n > 0 {
		key += fmt.Sprintf("::hours=%d", n)
	}
	return key
}

func forecastHours(o options) int {
	if o.graph && o.hours < 12 {
		return 12
	}
	return o.hours
}
