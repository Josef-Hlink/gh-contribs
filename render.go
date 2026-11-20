package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

var weekdayLabels = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

const block = '█'

// renderDaily shows a bar per day across the range (used for the week view).
func renderDaily(w io.Writer, cfg config, days []day) {
	labels := make([]string, len(days))
	stats := make([]string, len(days))
	values := make([]float64, len(days))
	total := 0
	for i, d := range days {
		labels[i] = fmt.Sprintf("%s %s", weekdayLabels[weekdayIndex(d.date.Weekday().String())], d.date.Format("01-02"))
		stats[i] = strconv.Itoa(d.count)
		values[i] = float64(d.count)
		total += d.count
	}

	header := fmt.Sprintf("Daily contributions, %s — %s", cfg.label, cfg.username)
	footer := fmt.Sprintf("%d contributions over %d days", total, len(days))
	drawChart(w, cfg, header, dateRange(days), labels, stats, values, footer)
}

// renderAverages shows average contributions per weekday across the range.
func renderAverages(w io.Writer, cfg config, days []day) {
	sums := [7]int{}
	occ := [7]int{}
	maxes := [7]int{}
	total := 0
	for _, d := range days {
		wd := weekdayIndex(d.date.Weekday().String())
		sums[wd] += d.count
		occ[wd]++
		if d.count > maxes[wd] {
			maxes[wd] = d.count
		}
		total += d.count
	}

	labels := make([]string, 7)
	stats := make([]string, 7)
	values := make([]float64, 7)
	for i := 0; i < 7; i++ {
		mean := 0.0
		if occ[i] > 0 {
			mean = float64(sums[i]) / float64(occ[i])
		}
		labels[i] = weekdayLabels[i]
		stats[i] = fmt.Sprintf("%5.1f avg   max %d", mean, maxes[i])
		values[i] = mean
	}

	header := fmt.Sprintf("Avg per weekday, %s — %s", cfg.label, cfg.username)
	footer := fmt.Sprintf("%d contributions over %d days", total, len(days))
	drawChart(w, cfg, header, dateRange(days), labels, stats, values, footer)
}

func dateRange(days []day) string {
	return fmt.Sprintf("%s → %s", days[0].date.Format("2006-01-02"), days[len(days)-1].date.Format("2006-01-02"))
}

// drawChart renders a horizontal bar chart: left labels, bars scaled and
// shaded relative to the largest value, and right-aligned stat text.
func drawChart(w io.Writer, cfg config, header, sub string, labels, stats []string, values []float64, footer string) {
	labelW := maxLen(labels)
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	bw := barArea(w, labelW, maxLen(stats))

	fmt.Fprintf(w, "\n%s\n%s\n\n", header, sub)
	for i := range labels {
		filled := 0
		if maxVal > 0 {
			filled = int(values[i]/maxVal*float64(bw) + 0.5)
		}
		bar := strings.Repeat(string(block), filled)
		if cfg.color {
			bar = colorize(bar, palette[levelOf(values[i], maxVal)])
		}
		pad := strings.Repeat(" ", bw-filled)
		fmt.Fprintf(w, "  %-*s  %s%s  %s\n", labelW, labels[i], bar, pad, stats[i])
	}
	fmt.Fprintf(w, "\n  %s\n\n", footer)
}

func colorize(s string, c [3]int) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", c[0], c[1], c[2], s)
}

func maxLen(ss []string) int {
	n := 0
	for _, s := range ss {
		if len(s) > n {
			n = len(s)
		}
	}
	return n
}

func weekdayIndex(name string) int {
	switch name {
	case "Monday":
		return 0
	case "Tuesday":
		return 1
	case "Wednesday":
		return 2
	case "Thursday":
		return 3
	case "Friday":
		return 4
	case "Saturday":
		return 5
	default: // Sunday
		return 6
	}
}

// termWidth is the terminal's column count, or 80 when it can't be determined.
func termWidth(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
			return cols
		}
	}
	return 80
}

// barArea sizes the bar column to the terminal, leaving room for labels/stats.
func barArea(w io.Writer, labelW, statW int) int {
	bw := termWidth(w) - labelW - statW - 6 // "  " + label + "  " + bar + "  " + stat
	if bw < 10 {
		bw = 10
	}
	if bw > 50 {
		bw = 50
	}
	return bw
}

// useColor decides the default color setting: on for a TTY, unless NO_COLOR is set.
func useColor() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}
