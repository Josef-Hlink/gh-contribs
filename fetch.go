package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// day is a single calendar day, its contribution count, and GitHub's own
// intensity level (0–4) used to shade the heatmap.
type day struct {
	date  time.Time
	count int
	level int
}

// cell holds the date and level parsed from a contribution day <td>, keyed
// elsewhere by id.
type cell struct {
	date  string
	level int
}

// fetchRange returns one entry per day within [from, to], oldest first.
//
// GitHub's intensity levels (data-level, 0–4) are normalized per response to
// that response's own max, so levels from different responses aren't comparable.
// To keep the heatmap and its legend consistent we avoid mixing responses where
// we can:
//
//   - A range inside the trailing year is served by the no-param endpoint in a
//     single response — the same data the github.com profile shows.
//   - An older range only comes back a whole calendar year at a time (from/to
//     just select the year), so we fetch each year involved. When more than one
//     year has to be stitched together we recompute the levels ourselves so they
//     share one scale.
func fetchRange(username string, from, to time.Time) ([]day, error) {
	var days []day

	if !from.Before(todayDate().AddDate(-1, 0, 0)) {
		d, err := fetchCalendar(username, "")
		if err != nil {
			return nil, err
		}
		days = d
	} else {
		merged := map[string]day{}
		years := 0
		for year := from.Year(); year <= to.Year(); year++ {
			yearDays, err := fetchCalendar(username, fmt.Sprintf("?from=%d-01-01&to=%d-12-31", year, year))
			if err != nil {
				return nil, err
			}
			for _, d := range yearDays {
				merged[dateStr(d.date)] = d
			}
			years++
		}
		for _, d := range merged {
			days = append(days, d)
		}
		if years > 1 {
			recomputeLevels(days)
		}
	}

	fromS, toS := dateStr(from), dateStr(to)
	out := make([]day, 0, len(days))
	for _, d := range days {
		if k := dateStr(d.date); k >= fromS && k <= toS {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].date.Before(out[j].date) })
	return out, nil
}

// fetchCalendar scrapes the no-JS contribution calendar. An empty query hits the
// default endpoint (the trailing year); pass "?from=..&to=.." to select a year.
func fetchCalendar(username, query string) ([]day, error) {
	url := fmt.Sprintf("https://github.com/users/%s/contributions%s", username, query)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub serves the bare calendar partial to this endpoint; a UA avoids edge cases.
	req.Header.Set("User-Agent", "gh-contribs (https://github.com/Josef-Hlink/gh-contribs)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("user %q not found on GitHub", username)
	default:
		return nil, fmt.Errorf("GitHub returned %s for %q", resp.Status, username)
	}

	return parseCalendar(resp.Body)
}

// recomputeLevels re-derives each day's 0–4 level from its count so days
// stitched from multiple yearly responses share one scale. It approximates
// GitHub's quartile shading (cutoffs at ¼/½/¾ of the scale), clamping the scale
// to the 98th percentile of active days so a few outlier days don't push
// everything into the lowest bucket — the same outlier handling we observed in
// GitHub's own levels.
func recomputeLevels(days []day) {
	var nz []int
	for _, d := range days {
		if d.count > 0 {
			nz = append(nz, d.count)
		}
	}
	if len(nz) == 0 {
		return
	}
	sort.Ints(nz)
	scale := nz[int(float64(len(nz)-1)*0.98+0.5)]
	for i := range days {
		days[i].level = levelFor(days[i].count, scale)
	}
}

func levelFor(count, max int) int {
	if count <= 0 || max <= 0 {
		return 0
	}
	switch r := float64(count) / float64(max); {
	case r <= 0.25:
		return 1
	case r <= 0.5:
		return 2
	case r <= 0.75:
		return 3
	default:
		return 4
	}
}

// parseCalendar walks the calendar HTML, joining day cells to their tooltips.
//
// Each day is a <td class="ContributionCalendar-day" data-date=".." id="..">,
// and each exact count lives in a sibling <tool-tip for="<id>">N contributions
// on ...</tool-tip>. We collect both, keyed by id, then join.
func parseCalendar(r interface{ Read([]byte) (int, error) }) ([]day, error) {
	z := html.NewTokenizer(r)

	cells := map[string]cell{} // id -> date
	var order []string         // ids in document order
	counts := map[string]int{} // id -> count

	for {
		switch z.Next() {
		case html.ErrorToken:
			return assemble(order, cells, counts)

		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			switch t.Data {
			case "td":
				id, date, level, ok := dayCell(t)
				if ok {
					cells[id] = cell{date: date, level: level}
					order = append(order, id)
				}
			case "tool-tip":
				if forID, ok := attr(t, "for"); ok {
					// The count sits in the element's text node.
					if z.Next() == html.TextToken {
						counts[forID] = parseCount(string(z.Text()))
					}
				}
			}
		}
	}
}

func assemble(order []string, cells map[string]cell, counts map[string]int) ([]day, error) {
	days := make([]day, 0, len(order))
	for _, id := range order {
		c := cells[id]
		d, err := time.Parse("2006-01-02", c.date)
		if err != nil {
			continue
		}
		days = append(days, day{date: d, count: counts[id], level: c.level})
	}
	// GitHub emits day cells column-major (all Sundays, then all Mondays, ...);
	// sort chronologically so callers can rely on oldest-first order.
	sort.Slice(days, func(i, j int) bool { return days[i].date.Before(days[j].date) })
	return days, nil
}

// dayCell reports whether t is a contribution day <td> and returns its
// id, date, and intensity level (0–4).
func dayCell(t html.Token) (id, date string, level int, ok bool) {
	class, _ := attr(t, "class")
	if !strings.Contains(class, "ContributionCalendar-day") {
		return "", "", 0, false
	}
	id, hasID := attr(t, "id")
	date, hasDate := attr(t, "data-date")
	if lvl, has := attr(t, "data-level"); has {
		level, _ = strconv.Atoi(lvl)
	}
	return id, date, level, hasID && hasDate
}

// parseCount turns "6 contributions on June 1st." (or "No contributions ...")
// into an integer count.
func parseCount(text string) int {
	text = strings.TrimSpace(text)
	first, _, _ := strings.Cut(text, " ")
	if first == "No" {
		return 0
	}
	n, err := strconv.Atoi(first)
	if err != nil {
		return 0
	}
	return n
}

func attr(t html.Token, name string) (string, bool) {
	for _, a := range t.Attr {
		if a.Key == name {
			return a.Val, true
		}
	}
	return "", false
}
