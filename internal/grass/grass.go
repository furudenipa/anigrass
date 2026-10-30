// Package grass renders a Sunday-first contribution calendar.
package grass

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

type Options struct {
	From, To     time.Time
	Theme, Glyph string
	Color        bool
	ShowWeekdays bool
	ShowMonths   bool
	ShowUser     bool
	ShowRange    bool
	ShowLegend   bool
}

// Nonzero colors use gh-grass's OKLCH lightness and Annict's hue
// (#d51c5b, approximately 9.32 degrees). Chroma starts from gh-grass's
// values and is reduced where necessary to fit the sRGB gamut.
// Level zero uses a muted pink-gray chosen separately for each theme.
// https://github.com/koki-develop/gh-grass/blob/main/internal/theme/theme.go
var palettes = map[string][5]uint32{
	"dark":  {0x231f22, 0x582731, 0x92354b, 0xe05274, 0xff879e},
	"light": {0xe9e3e5, 0xffc0c9, 0xff718f, 0xd45974, 0x903e4f},
}

func level(n int) int {
	switch {
	case n == 0:
		return 0
	case n <= 2:
		return 1
	case n <= 5:
		return 2
	case n <= 9:
		return 3
	default:
		return 4
	}
}

func cell(n int, o Options) string {
	l := level(n)
	if !o.Color {
		return string([]rune("·░▒▓█")[l])
	}
	color := palettes[o.Theme][l]
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", color>>16, color>>8&0xff, color&0xff, o.Glyph)
}

// Render builds the full output before writing it, propagating write errors.
func Render(w io.Writer, username string, days map[string]int, total int, o Options, showTotal bool) error {
	// Use narrow ambiguous characters, matching the default grass glyph. Keep
	// at least two columns so numeric month labels always fit.
	glyphWidth := 1
	if o.Color {
		width := runewidth.NewCondition()
		width.EastAsianWidth = false
		glyphWidth = width.StringWidth(o.Glyph)
	}
	columnWidth := max(2, glyphWidth+1)
	blank := strings.Repeat(" ", columnWidth)
	spacing := strings.Repeat(" ", columnWidth-glyphWidth)
	var b strings.Builder
	if o.ShowUser {
		b.WriteString(username)
	}
	if o.ShowRange {
		if o.ShowUser {
			b.WriteString("  ")
		}
		fmt.Fprintf(&b, "%s → %s", o.From.Format(time.DateOnly), o.To.Format(time.DateOnly))
	}
	if o.ShowUser || o.ShowRange {
		b.WriteString("\n\n")
	}
	start := o.From.AddDate(0, 0, -int(o.From.Weekday()))
	var weeks []time.Time
	for day := start; !day.After(o.To); day = day.AddDate(0, 0, 7) {
		weeks = append(weeks, day)
	}
	// Numeric month labels also work when a range crosses the year boundary.
	if o.ShowMonths {
		if o.ShowWeekdays {
			b.WriteString("    ")
		}
		for _, week := range weeks {
			label := blank
			for i := 0; i < 7; i++ {
				d := week.AddDate(0, 0, i)
				if !d.Before(o.From) && !d.After(o.To) && (d.Day() == 1 || d.Equal(o.From)) {
					label = fmt.Sprintf("%*d", columnWidth, d.Month())
					break
				}
			}
			b.WriteString(label)
		}
		b.WriteByte('\n')
	}
	for weekday, name := range []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"} {
		if o.ShowWeekdays {
			b.WriteString(name)
			b.WriteByte(' ')
		}
		for _, week := range weeks {
			day := week.AddDate(0, 0, weekday)
			if day.Before(o.From) || day.After(o.To) {
				b.WriteString(blank)
				continue
			}
			b.WriteString(cell(days[day.Format(time.DateOnly)], o))
			b.WriteString(spacing)
		}
		b.WriteByte('\n')
	}
	if o.ShowLegend {
		b.WriteString("\nLess ")
		for _, n := range []int{0, 1, 3, 6, 10} {
			b.WriteString(cell(n, o))
			b.WriteByte(' ')
		}
		b.WriteString("More  (0 / 1–2 / 3–5 / 6–9 / 10+ records/day)\n")
	}
	if showTotal {
		fmt.Fprintf(&b, "Total: %d episode records\n", total)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
