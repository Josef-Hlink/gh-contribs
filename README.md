# gh-contribs

_created on: 28-10-2021 · rewritten as a Go CLI: 01-06-2026_

## Description

A tiny CLI that gives you some insight into your contribution behaviour. It reads a
GitHub user's public contribution graph and prints it in the terminal. The view
is chosen from the length of the range:

- the **last year** (default) → a GitHub-style heatmap grid
- a range of **≤7 days** → a bar per day
- **`--avg`** → average contributions per weekday (over any range)
- **`--repos`** → a lane of weekly commits per repo, for the 10 repos you
  committed to most recently — private and org repos included when you're
  signed in to `gh`

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

Commits per repo — `gh-contribs --repos` (on an 80-column terminal; wider
ones get square two-column tiles):

```
Weekly commits per repo, last year — Josef-Hlink
2025-11-16 → 2026-11-19 · own commits, shared scale

                      Nov   Jan  Feb Mar Apr May  Jun Jul Aug  Sep Oct  Nov
  klym                ······················▒·▓░·░░░░░▒░▒····░░░·········░░   83
  dots                ░░░░░░·░▒▒░░░··░░░░░·█░·░░▓▓█▒▒▒▓▒▒·█░░░░░░·░░░░██▓█▓  562
  fax                 ···················································░▓   24
  assetblocks/pylona  ··░▒▒░···▒·░░░░░░░░░░·░··░░░░░░░░░░░░·░░·░░·░░░░░░░░░  174
  forj                ································▓█░··░░░░░▒··░░·▒░░█·  183
  …

  1172 commits across 10 repos
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
gh-contribs --repos                             # weekly commits in your 10 latest repos
gh-contribs --repos -n 5 --per-repo-scale       # 5 lanes, each on its own scale
gh-contribs --theme blue                        # shade in blue instead of green
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
| `--repos`            | draw a lane of weekly commits per repo (always the last year) |
| `-n N`               | with `--repos`: number of repos (default 10)                 |
| `--all-authors`      | with `--repos`: count and rank by everyone's commits         |
| `--per-repo-scale`   | with `--repos`: shade each lane on its own scale             |
| `--only-public`      | with `--repos`: leave out private repos                      |
| `--theme NAME`       | color theme: `green` (default), `blue`, `orange`, `purple`   |
| `--no-color`         | disable colored output (also honors the `NO_COLOR` env var)  |
| `-h, --help`         | show help                                                    |

A range of ≤7 days is drawn as a bar per day; anything longer is a heatmap grid
(or weekday averages with `--avg`). Ranges spanning multiple years are fetched
year by year and stitched together. A grid wider than your terminal is split
into stacked blocks. Color is automatically disabled when output isn't a
terminal (e.g. piped to a file). Set `GH_CONTRIBS_THEME` (e.g. in your shell
profile) to pick a default theme; `--theme` overrides it.

`--repos` uses the GitHub API instead of the profile calendar, authenticated
with `GH_TOKEN` / `GITHUB_TOKEN` or the `gh` CLI's token, so private, org and
collaborator repos are included (without a token it sees public repos only;
`--only-public` leaves private ones out on purpose).
It counts commits on each repo's default branch by author date, and by default
only your own commits — both for the counts and for picking the most recently
active repos. `--all-authors` switches both to everyone's commits. Lanes share
one shading scale so repos are comparable; `--per-repo-scale` shades each
repo against its own busiest weeks instead.

## Requirements

[Go](https://go.dev) 1.21+ to build. No runtime dependencies — `go install`
produces a single static binary.

## License

MIT
