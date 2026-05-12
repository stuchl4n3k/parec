// Package progress is a tiny single-line terminal progress bar.
//
// Two modes:
//   - total > 0: classic bar with percentage, current/total, elapsed.
//   - total <= 0: indeterminate; shows running count + elapsed only.
//
// Draws are throttled so callers can hammer Add/Set in tight loops.
// Output goes to stderr by default. Done() prints a newline so subsequent
// output starts on a fresh line.
package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	drawInterval = 80 * time.Millisecond
	defaultWidth = 28
)

type Bar struct {
	out     io.Writer
	label   string
	total   int
	width   int
	started time.Time

	mu       sync.Mutex
	current  int
	suffix   string
	lastDraw time.Time
	finished bool
}

func New(label string, total int) *Bar {
	b := &Bar{
		out:     os.Stderr,
		label:   label,
		total:   total,
		width:   defaultWidth,
		started: time.Now(),
	}
	b.mu.Lock()
	b.drawLocked(true)
	b.mu.Unlock()
	return b
}

func (b *Bar) Add(n int) {
	b.mu.Lock()
	b.current += n
	b.drawLocked(false)
	b.mu.Unlock()
}

func (b *Bar) Set(n int) {
	b.mu.Lock()
	b.current = n
	b.drawLocked(false)
	b.mu.Unlock()
}

// Suffix sets the trailing per-tick text (e.g. the current item name).
// Truncated to keep the rendered line short. Rune-aware so multi-byte
// UTF-8 (Czech, accented letters) doesn't get sliced mid-codepoint.
func (b *Bar) Suffix(s string) {
	const maxRunes = 50
	if r := []rune(s); len(r) > maxRunes {
		s = string(r[:maxRunes-1]) + "…"
	}
	b.mu.Lock()
	b.suffix = s
	b.drawLocked(false)
	b.mu.Unlock()
}

// Done renders one final, complete frame and writes a newline.
func (b *Bar) Done() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return
	}
	b.finished = true
	if b.total > 0 && b.current < b.total {
		b.current = b.total
	}
	b.drawLocked(true)
	fmt.Fprintln(b.out)
}

func (b *Bar) drawLocked(force bool) {
	now := time.Now()
	if !force && now.Sub(b.lastDraw) < drawInterval {
		return
	}
	b.lastDraw = now

	elapsed := now.Sub(b.started).Truncate(time.Second)
	var line string
	if b.total <= 0 {
		line = fmt.Sprintf("%s ... %d (%s)", b.label, b.current, elapsed)
	} else {
		pct := float64(b.current) / float64(b.total)
		if pct > 1 {
			pct = 1
		}
		filled := int(pct * float64(b.width))
		bar := strings.Repeat("#", filled) + strings.Repeat("-", b.width-filled)
		line = fmt.Sprintf("%s [%s] %3.0f%% %d/%d (%s)",
			b.label, bar, pct*100, b.current, b.total, elapsed)
	}
	if b.suffix != "" {
		line += " " + b.suffix
	}
	// CR + clear-to-end-of-line keeps redraws clean even when suffix shrinks.
	fmt.Fprintf(b.out, "\r\033[K%s", line)
}
