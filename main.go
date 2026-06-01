// Command gh-contribs fetches a GitHub user's public contribution graph and
// prints it in the terminal. A short range (≤7 days) is shown as a bar per day;
// a longer range is shown as a GitHub-style heatmap grid, or as average
// contributions per weekday with --avg.
package main

import (
	"fmt"
	"os"
	"time"
)

// dailyMax is the longest range (in days) shown as a bar-per-day graph.
const dailyMax = 7

type view int

const (
	viewDaily view = iota
	viewAverages
	viewGrid
)

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		usage(os.Stderr)
		os.Exit(2)
	}

	days, err := fetchRange(cfg.username, cfg.from, cfg.to)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(days) == 0 {
		fmt.Fprintf(os.Stderr, "error: no contributions for %q in %s\n", cfg.username, cfg.label)
		os.Exit(1)
	}

	switch cfg.view {
	case viewDaily:
		renderDaily(os.Stdout, cfg, days)
	case viewAverages:
		renderAverages(os.Stdout, cfg, days)
	default:
		renderGrid(os.Stdout, cfg, days)
	}
}

type config struct {
	username string
	color    bool
	from, to time.Time
	view     view
	label    string // human description of the range, for titles
}

func parseArgs(argv []string) (config, error) {
	color := useColor()
	avg := false
	chosen := "" // week | month | year, via --last-* flags
	var fromT, toT time.Time
	var fromSet, toSet bool

	setRange := func(name string) error {
		if chosen != "" {
			return fmt.Errorf("--last-%s conflicts with --last-%s", name, chosen)
		}
		chosen = name
		return nil
	}

	var positional []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		val := func() (string, error) {
			i++
			if i >= len(argv) {
				return "", fmt.Errorf("%s requires a value", a)
			}
			return argv[i], nil
		}

		var err error
		switch {
		case a == "-h" || a == "--help":
			usage(os.Stdout)
			os.Exit(0)
		case a == "--no-color":
			color = false
		case a == "--avg":
			avg = true
		case a == "--last-week":
			err = setRange("week")
		case a == "--last-month":
			err = setRange("month")
		case a == "--last-year":
			err = setRange("year")
		case a == "--from":
			var v string
			if v, err = val(); err == nil {
				fromT, err = parseDate(v)
				fromSet = true
			}
		case a == "--to":
			var v string
			if v, err = val(); err == nil {
				toT, err = parseDate(v)
				toSet = true
			}
		case len(a) > 0 && a[0] == '-':
			err = fmt.Errorf("unknown flag %q", a)
		default:
			positional = append(positional, a)
		}
		if err != nil {
			return config{}, err
		}
	}

	if len(positional) != 1 {
		return config{}, fmt.Errorf("expected exactly one username, got %d", len(positional))
	}
	if (fromSet || toSet) && chosen != "" {
		return config{}, fmt.Errorf("--from/--to cannot be combined with --last-%s", chosen)
	}

	cfg := config{username: positional[0], color: color}
	cfg.from, cfg.to, cfg.label = resolveRange(chosen, fromSet, toSet, fromT, toT)
	if cfg.from.After(cfg.to) {
		return config{}, fmt.Errorf("--from (%s) is after --to (%s)", dateStr(cfg.from), dateStr(cfg.to))
	}
	cfg.view = resolveView(cfg.from, cfg.to, avg)
	return cfg, nil
}

// resolveRange turns the range flags into a concrete [from, to] window (rolling,
// ending today) plus a human label.
func resolveRange(chosen string, fromSet, toSet bool, fromT, toT time.Time) (from, to time.Time, label string) {
	today := todayDate()
	switch {
	case fromSet || toSet:
		to = today
		if toSet {
			to = toT
		}
		if to.After(today) {
			to = today
		}
		from = to.AddDate(0, 0, -6)
		if fromSet {
			from = fromT
		}
		return from, to, fmt.Sprintf("%s → %s", dateStr(from), dateStr(to))
	case chosen == "week":
		return today.AddDate(0, 0, -6), today, "last 7 days"
	case chosen == "month":
		return today.AddDate(0, -1, 0), today, "last month"
	default: // year (explicit or default)
		return today.AddDate(-1, 0, 0), today, "last year"
	}
}

// resolveView picks the view: short ranges are daily bars (--avg is a noop),
// --avg forces the weekday-average graph, and everything else is the grid.
func resolveView(from, to time.Time, avg bool) view {
	span := int(to.Sub(from).Hours()/24) + 1
	switch {
	case span <= dailyMax:
		return viewDaily
	case avg:
		return viewAverages
	default:
		return viewGrid
	}
}

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s)
	}
	return t, nil
}

// date builds a UTC midnight time.Time for the given calendar day.
func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// todayDate is the current local calendar day as UTC midnight, so it compares
// cleanly with the UTC dates parsed out of the calendar HTML.
func todayDate() time.Time {
	n := time.Now()
	return date(n.Year(), n.Month(), n.Day())
}

func dateStr(t time.Time) string { return t.Format("2006-01-02") }

func usage(w *os.File) {
	fmt.Fprint(w, `gh-contribs — your GitHub contribution rhythm, in the terminal

usage:
  gh-contribs <username> [range] [--avg] [--no-color]

range (default: last year):
  --last-week         the last 7 days
  --last-month        the last month
  --last-year         the last year
  --from YYYY-MM-DD   custom range start
  --to   YYYY-MM-DD   custom range end

view:
  a range of ≤7 days is drawn as a bar per day; a longer range is drawn as a
  GitHub-style heatmap grid.
  --avg               instead draw average contributions per weekday
                      (ignored for ≤7-day ranges, which are already per-day)

other:
  --no-color          disable colored output (also honors the NO_COLOR env var)
  -h, --help          show this help
`)
}
