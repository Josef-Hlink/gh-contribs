package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// gridPalette is GitHub's 5-level scale (empty + 4 greens), tuned to read on a
// dark terminal: level 0 is a faint grey, levels 1–4 ramp up in green.
var gridPalette = [5][3]int{
	{48, 54, 61},  // 0 — empty   (#30363d)
	{14, 68, 41},  // 1           (#0e4429)
	{0, 109, 50},  // 2           (#006d32)
	{38, 166, 65}, // 3           (#26a641)
	{57, 211, 83}, // 4           (#39d353)
}

// gridGlyphs shade the levels when color is disabled.
var gridGlyphs = [5]rune{'·', '░', '▒', '▓', '█'}

// gridGutter is the width of the left weekday-label column ("Mon ").
const gridGutter = 4

// cellW is how many terminal columns one day occupies; 2 makes roughly square
// boxes (character cells are about twice as tall as they are wide).
const cellW = 2

// weekRowLabels label the Mon/Wed/Fri rows, GitHub-style (Sunday is row 0).
var weekRowLabels = [7]string{"", "Mon", "", "Wed", "", "Fri", ""}

// gcol is one week column: the levels for its 7 days, -1 where there's no data
// (cells outside the requested range that pad the week to Sun–Sat).
type gcol struct {
	weekStart time.Time
	levels    [7]int
}

// renderGrid draws a GitHub-style contribution heatmap, wrapping into stacked
// blocks when it's wider than the terminal.
func renderGrid(w io.Writer, cfg config, days []day) {
	cols := buildColumns(days)
	total := 0
	for _, d := range days {
		total += d.count
	}

	fmt.Fprintf(w, "\nContributions, %s — %s\n%s\n", cfg.label, cfg.username, dateRange(days))

	perBlock := (termWidth(w) - gridGutter) / cellW
	if perBlock < 1 {
		perBlock = 1
	}
	for start := 0; start < len(cols); start += perBlock {
		end := start + perBlock
		if end > len(cols) {
			end = len(cols)
		}
		fmt.Fprintln(w)
		printGridBlock(w, cfg, cols[start:end])
	}

	renderLegend(w, cfg, days)
	fmt.Fprintf(w, "\n  %d contributions over %d days\n\n", total, len(days))
}

// buildColumns groups days into Sunday-started week columns, in date order.
func buildColumns(days []day) []gcol {
	var cols []gcol
	index := map[string]int{}
	for _, d := range days {
		ws := d.date.AddDate(0, 0, -int(d.date.Weekday())) // Sunday of this week
		key := dateStr(ws)
		ci, ok := index[key]
		if !ok {
			ci = len(cols)
			index[key] = ci
			c := gcol{weekStart: ws}
			for r := range c.levels {
				c.levels[r] = -1
			}
			cols = append(cols, c)
		}
		cols[ci].levels[int(d.date.Weekday())] = d.level
	}
	return cols
}

func printGridBlock(w io.Writer, cfg config, cols []gcol) {
	fmt.Fprintln(w, monthLabelLine(cols))
	for row := 0; row < 7; row++ {
		var b strings.Builder
		fmt.Fprintf(&b, "%-*s", gridGutter, weekRowLabels[row])
		for _, c := range cols {
			b.WriteString(gridCell(cfg, c.levels[row]))
		}
		fmt.Fprintln(w, b.String())
	}
}

// monthLabelLine places a month abbreviation above the first column of each
// month. The month is keyed on the week's Saturday, so the label lands on the
// week that first reaches into the new month (matching GitHub). Labels that
// would overlap the previous one or run off the right edge are skipped.
func monthLabelLine(cols []gcol) string {
	width := gridGutter + len(cols)*cellW
	line := []rune(strings.Repeat(" ", width))
	prev := time.Month(0)
	lastEnd := 0
	for i, c := range cols {
		sat := c.weekStart.AddDate(0, 0, 6)
		if sat.Month() == prev {
			continue
		}
		prev = sat.Month()
		label := sat.Format("Jan")
		pos := gridGutter + i*cellW
		if pos < lastEnd || pos+len(label) > width {
			continue
		}
		for j, r := range label {
			line[pos+j] = r
		}
		lastEnd = pos + len(label) + 1 // keep at least one space before the next
	}
	return strings.TrimRight(string(line), " ")
}

func gridCell(cfg config, level int) string {
	if level < 0 {
		return strings.Repeat(" ", cellW) // no data for this day (week padding)
	}
	if cfg.color {
		return colorize(strings.Repeat("█", cellW), gridPalette[level])
	}
	return strings.Repeat(string(gridGlyphs[level]), cellW)
}

// renderLegend prints a vertical key mapping each level's swatch to the range of
// contribution counts observed at that level in the displayed data. GitHub's own
// cutoffs aren't a clean function of the max (it clamps outliers), so rather than
// reverse-engineering them we report the exact ranges from the scraped levels.
func renderLegend(w io.Writer, cfg config, days []day) {
	lo := [5]int{}
	has := [5]bool{}
	for _, d := range days {
		if !has[d.level] || d.count < lo[d.level] {
			lo[d.level] = d.count
		}
		has[d.level] = true
	}

	// Levels 1–4 that actually appear, ascending; each level's upper bound is one
	// below the next present level's lower bound, so the ranges tile with no gaps.
	var present []int
	for l := 1; l <= 4; l++ {
		if has[l] {
			present = append(present, l)
		}
	}

	rangeText := func(i int) string {
		l := present[i]
		if i == len(present)-1 {
			return fmt.Sprintf("%d+", lo[l])
		}
		hi := lo[present[i+1]] - 1
		if hi <= lo[l] {
			return fmt.Sprintf("%d", lo[l])
		}
		return fmt.Sprintf("%d–%d", lo[l], hi)
	}

	fmt.Fprintln(w)
	for i := len(present) - 1; i >= 0; i-- {
		fmt.Fprintf(w, "  %s  %s\n", gridCell(cfg, present[i]), rangeText(i))
	}
	if has[0] {
		fmt.Fprintf(w, "  %s  0\n", gridCell(cfg, 0))
	}
}
