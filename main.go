// Command gh-contribs fetches a GitHub user's public contribution graph and
// prints it in the terminal. A short range (≤7 days) is shown as a bar per day;
// a longer range is shown as a GitHub-style heatmap grid, or as average
// contributions per weekday with --avg. With --repos it instead shows weekly
// commits per repo (private ones included when signed in to gh), a lane each.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// dailyMax is the longest range (in days) shown as a bar-per-day graph.
const dailyMax = 7

// defaultRepos is how many lanes --repos shows unless -n says otherwise.
const defaultRepos = 10

// version is the build version, injected at release time via -ldflags.
var version = "dev"

type view int

const (
	viewDaily view = iota
	viewAverages
	viewGrid
	viewRepos
)

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		usage(os.Stderr)
		os.Exit(2)
	}

	if cfg.view == viewRepos {
		weeks := windowWeeks(cfg.from, cfg.to)
		sp := startSpinner(cfg.spinColor, cfg.theme, "listing repos")
		lanes, err := fetchLanes(cfg, weeks, sp)
		sp.Stop()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		if len(lanes) == 0 {
			fmt.Fprintf(os.Stderr, "error: no commits by %q in %s in any repo visible to you\n", cfg.username, cfg.label)
			os.Exit(1)
		}
		renderLanes(os.Stdout, cfg, weeks, lanes)
		return
	}

	sp := startSpinner(cfg.spinColor, cfg.theme, "fetching contributions")
	days, err := fetchRange(cfg.username, cfg.from, cfg.to)
	sp.Stop()
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
	// spinColor colors the progress indicator on stderr, which can be a
	// terminal even when stdout (and so color) isn't.
	spinColor bool
	theme     theme // colors for levels 0–4
	from, to  time.Time
	view      view
	label     string // human description of the range, for titles

	// --repos view
	nRepos       int  // lanes to show
	allAuthors   bool // count (and rank by) everyone's commits, not just the user's
	perRepoScale bool // shade each lane on its own scale instead of a shared one
	onlyPublic   bool // leave out private repos
}

func parseArgs(argv []string) (config, error) {
	color := useColor()
	_, noColorEnv := os.LookupEnv("NO_COLOR")
	spinColor := !noColorEnv
	avg := false
	repos, nSet, allAuthors, perRepoScale, onlyPublic := false, false, false, false, false
	nRepos := defaultRepos
	th, _ := lookupTheme(defaultTheme)
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
		case a == "-v" || a == "--version":
			fmt.Println("gh-contribs", version)
			os.Exit(0)
		case a == "--no-color":
			color, spinColor = false, false
		case a == "--theme":
			var v string
			if v, err = val(); err == nil {
				th, err = lookupTheme(v)
			}
		case a == "--avg":
			avg = true
		case a == "--repos":
			repos = true
		case a == "-n":
			var v string
			if v, err = val(); err == nil {
				nRepos, err = strconv.Atoi(v)
				if err != nil || nRepos < 1 {
					err = fmt.Errorf("-n wants a positive number, got %q", v)
				}
				nSet = true
			}
		case a == "--all-authors":
			allAuthors = true
		case a == "--per-repo-scale":
			perRepoScale = true
		case a == "--only-public":
			onlyPublic = true
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

	var username string
	switch len(positional) {
	case 0:
		// No username given — fall back to the gh CLI's authenticated user.
		u, err := ghUsername()
		if err != nil {
			return config{}, err
		}
		username = u
	case 1:
		username = positional[0]
	default:
		return config{}, fmt.Errorf("expected at most one username, got %d", len(positional))
	}
	if (fromSet || toSet) && chosen != "" {
		return config{}, fmt.Errorf("--from/--to cannot be combined with --last-%s", chosen)
	}
	if repos {
		// The repo walk relies on pushed_at to know when to stop, which only
		// bounds a window that ends today; so lanes always cover the last year.
		if avg || fromSet || toSet || (chosen != "" && chosen != "year") {
			return config{}, fmt.Errorf("--repos always covers the last year; drop --avg and the range flags")
		}
	} else if nSet || allAuthors || perRepoScale || onlyPublic {
		return config{}, fmt.Errorf("-n, --all-authors, --per-repo-scale and --only-public only apply to --repos")
	}

	cfg := config{username: username, color: color, spinColor: spinColor, theme: th, nRepos: nRepos, allAuthors: allAuthors, perRepoScale: perRepoScale, onlyPublic: onlyPublic}
	cfg.from, cfg.to, cfg.label = resolveRange(chosen, fromSet, toSet, fromT, toT)
	if cfg.from.After(cfg.to) {
		return config{}, fmt.Errorf("--from (%s) is after --to (%s)", dateStr(cfg.from), dateStr(cfg.to))
	}
	cfg.view = resolveView(cfg.from, cfg.to, avg)
	if repos {
		cfg.view = viewRepos
	}
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

// ghUsername resolves the current user's login from the GitHub CLI, used when no
// username is passed on the command line. It returns a helpful error when gh is
// missing or the user isn't authenticated, so the caller can tell them to either
// sign in or pass a username explicitly.
func ghUsername() (string, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("no username given and the GitHub CLI (gh) is not installed — pass a username or install gh")
	}
	out, err := exec.Command(path, "api", "user", "--jq", ".login").Output()
	if err != nil {
		return "", fmt.Errorf("no username given and the GitHub CLI isn't signed in — run `gh auth login` or pass a username")
	}
	login := strings.TrimSpace(string(out))
	if login == "" {
		return "", fmt.Errorf("no username given and could not read your login from the gh CLI — pass a username")
	}
	return login, nil
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
  gh-contribs [username] [range] [--avg] [--theme NAME] [--no-color]
  gh-contribs [username] --repos [-n N] [--all-authors] [--per-repo-scale]
                                [--only-public]

  username defaults to the GitHub CLI's authenticated user when gh is installed
  and signed in (gh auth login).

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

repos (always the last year):
  --repos             a lane of weekly commits per repo, for the repos you
                      committed to most recently; includes private and org
                      repos when signed in to gh (or GH_TOKEN is set).
                      Counts commits on each repo's default branch.
  -n N                number of repos (default 10)
  --all-authors       count and rank by everyone's commits, not just yours
  --per-repo-scale    shade each lane on its own scale (default: shared)
  --only-public       leave out private repos (e.g. for a screenshot)

other:
  --theme NAME        color theme: `+themeNames()+` (default `+defaultTheme+`)
  --no-color          disable colored output (also honors the NO_COLOR env var)
  -h, --help          show this help
  -v, --version       show version
`)
}
