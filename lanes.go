package main

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// laneNameMax caps the repo-name column; longer names are truncated.
const laneNameMax = 24

// renderLanes draws one row of weekly tiles per repo, GitHub-heatmap style,
// with each repo's commit total on the right. Tiles are two columns wide when
// that fits the terminal, one otherwise, and the lanes wrap into stacked blocks
// only if even that is too wide.
func renderLanes(w io.Writer, cfg config, weeks []time.Time, lanes []repoLane) {
	names := make([]string, len(lanes))
	totals := make([]string, len(lanes))
	sum := 0
	for i, l := range lanes {
		names[i] = truncate(l.name, laneNameMax)
		totals[i] = fmt.Sprint(l.total)
		sum += l.total
	}
	nameW, totalW := 0, maxLen(totals)
	for _, n := range names {
		nameW = max(nameW, utf8.RuneCountInString(n))
	}
	gutter := 2 + nameW + 2

	levels := laneLevels(cfg, lanes)

	whose, scale := "own commits", "shared scale"
	if cfg.allAuthors {
		whose = "all authors"
	}
	if cfg.perRepoScale {
		scale = "per-repo scale"
	}
	fmt.Fprintf(w, "\nWeekly commits per repo, %s — %s\n", cfg.label, cfg.username)
	fmt.Fprintf(w, "%s → %s · %s, %s\n", dateStr(weeks[0]), dateStr(cfg.to), whose, scale)

	tw := termWidth(w)
	cw := 2
	if gutter+len(weeks)*cw+2+totalW > tw {
		cw = 1
	}
	perBlock := max(1, (tw-gutter-2-totalW)/cw)

	for start := 0; start < len(weeks); start += perBlock {
		end := min(start+perBlock, len(weeks))
		lastBlock := end == len(weeks)
		fmt.Fprintln(w)
		fmt.Fprintln(w, monthLabels(weeks[start:end], gutter, cw))
		for i := range lanes {
			var b strings.Builder
			fmt.Fprintf(&b, "  %-*s  ", nameW+len(names[i])-utf8.RuneCountInString(names[i]), names[i])
			for _, lvl := range levels[i][start:end] {
				b.WriteString(tile(cfg, lvl, cw))
			}
			if lastBlock {
				fmt.Fprintf(&b, "  %*s", totalW, totals[i])
			}
			fmt.Fprintln(w, b.String())
		}
	}

	if cfg.perRepoScale {
		var b strings.Builder
		for l := 0; l <= 4; l++ {
			b.WriteString(tile(cfg, l, 2))
		}
		fmt.Fprintf(w, "\n  less %s more — each repo shaded relative to its own busiest weeks\n", b.String())
	} else {
		var pts []day
		for i, l := range lanes {
			for j, c := range l.weeks {
				pts = append(pts, day{count: c, level: levels[i][j]})
			}
		}
		renderLegend(w, cfg, pts)
	}
	fmt.Fprintf(w, "\n  %d commits across %d repos\n\n", sum, len(lanes))
}

// laneLevels shades each week 0–4: on one scale shared by all lanes, or with
// --per-repo-scale on each lane's own scale.
func laneLevels(cfg config, lanes []repoLane) [][]int {
	levels := make([][]int, len(lanes))
	var shared int
	if !cfg.perRepoScale {
		var all []int
		for _, l := range lanes {
			all = append(all, l.weeks...)
		}
		shared = activeScale(all)
	}
	for i, l := range lanes {
		scale := shared
		if cfg.perRepoScale {
			scale = activeScale(l.weeks)
		}
		levels[i] = make([]int, len(l.weeks))
		for j, c := range l.weeks {
			levels[i][j] = levelFor(c, scale)
		}
	}
	return levels
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
