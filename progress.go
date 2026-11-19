package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// spinTick is the animation frame interval.
const spinTick = 80 * time.Millisecond

// spinTiles is how many tiles the indicator sweeps across.
const spinTiles = 8

// scan is the progress indicator: a bright tile sweeping back and forth
// across a strip of heatmap tiles, trailing fading greens.
type scan struct {
	pos, dir int
	levels   []int
}

func (s *scan) frame(color bool) string {
	if s.levels == nil {
		s.levels, s.dir = make([]int, spinTiles), 1
	}
	for i := range s.levels {
		s.levels[i] = max(0, s.levels[i]-1)
	}
	s.levels[s.pos] = 4
	out := tileStrip(s.levels, color)
	if s.pos+s.dir < 0 || s.pos+s.dir >= spinTiles {
		s.dir = -s.dir
	}
	s.pos += s.dir
	return out
}

// tileStrip renders levels as adjacent single-column heatmap tiles.
func tileStrip(levels []int, color bool) string {
	var b strings.Builder
	for _, l := range levels {
		b.WriteString(tile(config{color: color}, l, 1))
	}
	return b.String()
}

// spinner animates a progress indicator plus a status message on stderr while
// we wait on GitHub. It's a no-op when stderr isn't a terminal, and all methods
// are safe on a nil *spinner.
type spinner struct {
	mu     sync.Mutex
	anim   scan
	color  bool
	status string
	frame  string
	stop   chan struct{}
	done   chan struct{}
}

func startSpinner(color bool, status string) *spinner {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return nil
	}
	s := &spinner{
		color:  color,
		status: status,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	s.frame = s.anim.frame(color)
	s.draw()
	go s.run()
	return s
}

func (s *spinner) run() {
	defer close(s.done)
	t := time.NewTicker(spinTick)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			s.frame = s.anim.frame(s.color)
			s.draw()
			s.mu.Unlock()
		}
	}
}

// draw repaints the spinner line. Callers hold s.mu (or own s exclusively).
func (s *spinner) draw() {
	fmt.Fprintf(os.Stderr, "\r\x1b[2K%s  %s", s.frame, s.status)
}

// Status replaces the message shown next to the indicator.
func (s *spinner) Status(format string, a ...any) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = fmt.Sprintf(format, a...)
	s.draw()
}

// Warnf prints a line above the spinner without garbling it.
func (s *spinner) Warnf(format string, a ...any) {
	if s == nil {
		fmt.Fprintf(os.Stderr, format, a...)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprint(os.Stderr, "\r\x1b[2K")
	fmt.Fprintf(os.Stderr, format, a...)
	s.draw()
}

// Stop ends the animation and clears its line.
func (s *spinner) Stop() {
	if s == nil {
		return
	}
	close(s.stop)
	<-s.done
	fmt.Fprint(os.Stderr, "\r\x1b[2K")
}
