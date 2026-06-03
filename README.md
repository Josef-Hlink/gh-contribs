# gh-contribs

_created on: 28-10-2021 · rewritten as a Go CLI: 01-06-2026_

## Description

A tiny CLI that gives you some insight into your contribution behaviour. It reads a
GitHub user's public contribution graph and prints it in the terminal. The view
is chosen from the length of the range:

- the **last year** (default) → a GitHub-style heatmap grid
- a range of **≤7 days** → a bar per day
- **`--avg`** → average contributions per weekday (over any range)

No browser, no scraping headaches: it reads the plain-HTML calendar GitHub
serves at `/users/<username>/contributions`.

## Example

The default grid — `gh-contribs torvalds`:

```
Contributions, last year — torvalds
2025-06-01 → 2026-06-01

    Jun     Jul     Aug       Sep     Oct     Nov       Dec     Jan       Feb     Mar     Apr     May
    ░░▒▒░░░░░░▒▒░░▒▒░░░░░░░░░░░░░░░░░░░░▒▒░░░░▒▒░░░░░░░░░░░░░░░░▒▒░░░░▓▓░░░░░░░░░░▒▒░░▒▒░░▒▒▒▒░░░░░░░░░░░░▒▒▓▓
Mon ▒▒░░░░░░░░░░░░░░██░░░░░░░░··░░░░░░██▒▒··░░░░··░░░░░░▓▓░░··░░░░░░░░░░░░░░██▒▒░░░░··░░░░░░░░██▒▒░░░░░░░░░░··
    ░░░░░░░░░░░░░░░░██░░░░▒▒▒▒░░░░░░░░▒▒░░░░░░░░░░░░░░░░██░░··░░░░▒▒░░░░░░░░▓▓░░▒▒░░░░░░░░░░░░▒▒▒▒░░░░░░░░░░
Fri ▓▓▒▒░░▒▒▒▒░░▒▒░░▒▒░░▒▒▒▒░░▒▒░░▒▒▒▒██▒▒░░▒▒▒▒░░▒▒░░▒▒██░░░░▒▒▒▒▒▒▒▒▒▒░░▒▒░░▓▓▒▒▓▓▒▒▒▒▒▒▒▒░░▓▓▓▓▒▒▒▒▓▓▓▓▒▒

  ██  34+
  ▓▓  24–33
  ▒▒  12–23
  ░░  1–11
  ··  0

  3045 contributions over 366 days
```

(The grid is shaded with GitHub's green levels in a real terminal; the rows
above are abbreviated. The key shows the contribution-count range observed at
each level — read straight from GitHub's own data, so it's exact even for
spiky histories.) Average per weekday — `gh-contribs torvalds --avg`:

```
Avg per weekday, last year — torvalds
2025-06-01 → 2026-06-01

  Mon  █████████████████████████                             6.8 avg   max 61
  Tue  ████████████████████████████                          7.6 avg   max 52
  Wed  ███████████████████████████                           7.4 avg   max 37
  Thu  █████████████████████████████                         8.0 avg   max 34
  Fri  ██████████████████████████████████████████████████   13.9 avg   max 33
  Sat  ██████████████████████████                            7.2 avg   max 28
  Sun  ███████████████████████████                           7.4 avg   max 20

  3045 contributions over 366 days
```

A short range — `gh-contribs torvalds --last-week`:

```
Daily contributions, last 7 days — torvalds
2026-05-26 → 2026-06-01

  Tue 05-26  ████████                                            3
  Wed 05-27                                                      0
  Thu 05-28  ███████████████████████                             9
  Fri 05-29  ████████████████████████████████████████            16
  Sat 05-30  █████████████████████████                           10
  Sun 05-31  ██████████████████████████████████████████████████  20
  Mon 06-01                                                      0

  58 contributions over 7 days
```

## Install

```bash
go install github.com/Josef-Hlink/gh-contribs@latest
```

Or build from a clone:

```bash
go build -o gh-contribs .
```

## Usage

```bash
gh-contribs                                     # your own graph, via the gh CLI
gh-contribs <username>                          # last year, as a grid
gh-contribs <username> --avg                    # weekday averages
gh-contribs <username> --last-week              # the last 7 days, a bar per day
gh-contribs <username> --last-month             # the last month, as a grid
gh-contribs <username> --from 2024-03-01 --to 2024-03-07
```

The username is optional: when omitted, gh-contribs asks the [GitHub CLI](https://cli.github.com/)
(`gh`) for your authenticated login, so `gh-contribs` on its own shows your own
graph. You'll get a helpful nudge if `gh` isn't installed or signed in.

| flag                 | description                                                  |
| -------------------- | ------------------------------------------------------------ |
| `--last-week`        | the last 7 days                                              |
| `--last-month`       | the last month                                               |
| `--last-year`        | the last year (the default)                                  |
| `--from YYYY-MM-DD`  | custom range start                                           |
| `--to YYYY-MM-DD`    | custom range end                                             |
| `--avg`              | draw average contributions per weekday instead              |
| `--no-color`         | disable colored output (also honors the `NO_COLOR` env var)  |
| `-h, --help`         | show help                                                    |

A range of ≤7 days is drawn as a bar per day; anything longer is a heatmap grid
(or weekday averages with `--avg`). Ranges spanning multiple years are fetched
year by year and stitched together. A grid wider than your terminal is split
into stacked blocks. Color is automatically disabled when output isn't a
terminal (e.g. piped to a file).

## Requirements

[Go](https://go.dev) 1.21+ to build. No runtime dependencies — `go install`
produces a single static binary.

## License

MIT
