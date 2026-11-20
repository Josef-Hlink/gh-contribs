package main

import (
	"fmt"
	"os"
	"strings"
)

// theme is a 5-level contribution scale: level 0 (no activity) followed by
// four steps of increasing activity. Every view shades from the same theme so
// a level looks the same in the grid, the repo lanes and the bar charts.
type theme [5][3]int

// empty is level 0 in every theme: a faint grey that reads on a dark terminal.
var empty = [3]int{48, 54, 61} // #30363d

// themes are the --theme choices, in the order --help lists them. Each ramps
// from a dark, muted step up to a bright one, like GitHub's dark-theme scale.
var themes = []struct {
	name string
	t    theme
}{
	{"green", theme{empty, {14, 68, 41}, {0, 109, 50}, {38, 166, 65}, {57, 211, 83}}},         // GitHub: #0e4429 #006d32 #26a641 #39d353
	{"blue", theme{empty, {12, 45, 107}, {17, 88, 199}, {56, 139, 253}, {121, 192, 255}}},     // #0c2d6b #1158c7 #388bfd #79c0ff
	{"orange", theme{empty, {90, 30, 2}, {155, 66, 21}, {219, 109, 40}, {255, 166, 87}}},      // #5a1e02 #9b4215 #db6d28 #ffa657
	{"purple", theme{empty, {60, 30, 112}, {110, 64, 201}, {163, 113, 247}, {210, 168, 255}}}, // #3c1e70 #6e40c9 #a371f7 #d2a8ff
}

// defaultTheme is used when neither --theme nor themeEnv is set.
const defaultTheme = "green"

// themeEnv names the environment variable that sets the default theme;
// --theme overrides it.
const themeEnv = "GH_CONTRIBS_THEME"

// envTheme is the theme to use when --theme isn't given: themeEnv's if it's
// set and non-empty, else defaultTheme.
func envTheme() (theme, error) {
	name := os.Getenv(themeEnv)
	if name == "" {
		return lookupTheme(defaultTheme)
	}
	t, err := lookupTheme(name)
	if err != nil {
		return theme{}, fmt.Errorf("%s: %w", themeEnv, err)
	}
	return t, nil
}

// themeNames lists the theme names, comma-separated, for help and errors.
func themeNames() string {
	names := make([]string, len(themes))
	for i, t := range themes {
		names[i] = t.name
	}
	return strings.Join(names, ", ")
}

// lookupTheme finds a theme by name.
func lookupTheme(name string) (theme, error) {
	for _, t := range themes {
		if t.name == name {
			return t.t, nil
		}
	}
	return theme{}, fmt.Errorf("unknown theme %q (want one of: %s)", name, themeNames())
}
