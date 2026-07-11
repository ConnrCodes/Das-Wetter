package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"daswetter/internal/cache"
)

// ExecuteInteractive runs the human-friendly `das wetter` terminal session.
// The canonical `weather` command remains one-shot for scripts and pipelines.
func ExecuteInteractive() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	root := newRoot()
	// Use an explicitly empty slice. A nil Cobra argument list falls back to
	// os.Args, which would incorrectly geocode the brand word "wetter".
	root.SetArgs([]string{})
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		printInteractiveError(err)
	}
	sessionLocation := cache.Default().LastIPLocation()

	reader := bufio.NewReader(os.Stdin)
	for {
		if ctx.Err() != nil {
			return
		}
		fmt.Fprint(os.Stdout, "\n~ ")
		line, err := readInteractiveLine(ctx, reader)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Fprintln(os.Stdout)
				return
			}
			if !errors.Is(err, io.EOF) {
				fmt.Fprintln(os.Stderr, "weather:", err)
			}
			return
		}
		args, err := splitInput(strings.TrimSpace(line))
		if err != nil {
			fmt.Fprintln(os.Stderr, "weather:", err)
			continue
		}
		args = trimInteractivePrefix(args)
		if len(args) == 0 {
			continue
		}
		if len(args) == 1 && (strings.EqualFold(args[0], "quit") || strings.EqualFold(args[0], "exit")) {
			fmt.Fprintln(os.Stdout)
			return
		}
		args = pinSessionLocation(args, sessionLocation)

		root = newRoot()
		root.SetArgs(args)
		root.SetOut(os.Stdout)
		root.SetErr(os.Stderr)
		if err := root.ExecuteContext(ctx); err != nil {
			printInteractiveError(err)
		} else if location := cache.Default().LastLocation(); location != "" {
			sessionLocation = location
		}
	}
}

func readInteractiveLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		resultCh <- result{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resultCh:
		return result.line, result.err
	}
}

func printInteractiveError(err error) {
	var condition *exitError
	if errors.As(err, &condition) {
		fmt.Fprintln(os.Stderr, "weather: condition false (exit 1)")
		return
	}
	fmt.Fprintln(os.Stderr, "weather:", err)
}

func trimInteractivePrefix(args []string) []string {
	if len(args) > 0 && strings.EqualFold(args[0], "weather") {
		return args[1:]
	}
	if len(args) > 1 && strings.EqualFold(args[0], "das") && strings.EqualFold(args[1], "wetter") {
		return args[2:]
	}
	return args
}

func pinSessionLocation(args []string, location string) []string {
	if location == "" {
		return args
	}
	if len(args) == 0 {
		return []string{location}
	}
	word := strings.ToLower(args[0])
	if strings.HasPrefix(word, "-") {
		return append([]string{location}, args...)
	}
	switch word {
	case "refresh", "tomorrow", "alerts", "graph", "astro", "json", "metric", "imperial", "hours", "watch":
		return append([]string{location}, args...)
	default:
		return args
	}
}

func splitInput(line string) ([]string, error) {
	var args []string
	var current []rune
	var quote rune
	escaped := false
	started := false
	flush := func() {
		if started {
			args = append(args, string(current))
			current = nil
			started = false
		}
	}
	for _, r := range line {
		if escaped {
			current = append(current, r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current = append(current, r)
			}
			started = true
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			started = true
		case ' ', '\t', '\r', '\n':
			flush()
		default:
			current = append(current, r)
			started = true
		}
	}
	if escaped {
		current = append(current, '\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return args, nil
}
