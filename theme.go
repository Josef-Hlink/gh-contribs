package main

// theme is a 5-level contribution scale: level 0 (no activity) followed by
// four steps of increasing activity. Every view shades from the same theme so
// a level looks the same in the grid, the repo lanes and the bar charts.
type theme [5][3]int

// green is GitHub's dark-theme scale: level 0 is a faint grey, levels 1–4 ramp
// up in green.
var green = theme{
	{48, 54, 61},  // 0 — empty   (#30363d)
	{14, 68, 41},  // 1           (#0e4429)
	{0, 109, 50},  // 2           (#006d32)
	{38, 166, 65}, // 3           (#26a641)
	{57, 211, 83}, // 4           (#39d353)
}

// palette is the theme every view draws with.
var palette = green
