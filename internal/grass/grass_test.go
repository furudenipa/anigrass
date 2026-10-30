package grass

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func date(value string) time.Time {
	d, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return d
}

func TestCalendarAlignment(t *testing.T) {
	var b bytes.Buffer
	err := Render(&b, "alice", map[string]int{"2026-10-01": 1, "2026-10-03": 3, "2026-10-04": 10}, 14,
		Options{From: date("2026-10-01"), To: date("2026-10-05"), Theme: "dark", Glyph: "■",
			ShowWeekdays: true, ShowMonths: true, ShowUser: true, ShowRange: true, ShowLegend: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "alice  2026-10-01 → 2026-10-05\n\n" +
		"    10  \n" +
		"Sun   █ \nMon   · \nTue     \nWed     \nThu ░   \nFri ·   \nSat ▒   \n" +
		"\nLess · ░ ▒ ▓ █ More  (0 / 1–2 / 3–5 / 6–9 / 10+ records/day)\nTotal: 14 episode records\n"
	if b.String() != want {
		t.Fatalf("got:\n%q\nwant:\n%q", b.String(), want)
	}
}

func TestOptionalLabels(t *testing.T) {
	const bare = "· \n· \n  \n  \n  \n  \n  \n"
	for _, tc := range []struct {
		name  string
		opts  Options
		total bool
		want  string
	}{
		{"grass only", Options{}, false, bare},
		{"user only", Options{ShowUser: true}, false, "alice\n\n" + bare},
		{"range only", Options{ShowRange: true}, false, "2026-10-04 → 2026-10-05\n\n" + bare},
		{"months only", Options{ShowMonths: true}, false, "10\n" + bare},
		{"weekdays only", Options{ShowWeekdays: true}, false, "Sun · \nMon · \nTue   \nWed   \nThu   \nFri   \nSat   \n"},
		{"legend only", Options{ShowLegend: true}, false, bare + "\nLess · ░ ▒ ▓ █ More  (0 / 1–2 / 3–5 / 6–9 / 10+ records/day)\n"},
		{"total only", Options{}, true, bare + "Total: 0 episode records\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.From, tc.opts.To = date("2026-10-04"), date("2026-10-05")
			var b bytes.Buffer
			if err := Render(&b, "alice", nil, 0, tc.opts, tc.total); err != nil {
				t.Fatal(err)
			}
			if b.String() != tc.want {
				t.Fatalf("got %q; want %q", b.String(), tc.want)
			}
		})
	}
}

func TestCustomGlyphCalendarAlignment(t *testing.T) {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, tc := range []struct {
		name, glyph, visible, blank, months string
		color                               bool
	}{
		{"default", "■", "■ ", "  ", "1011  ", true},
		{"multiple", "■■", "■■ ", "   ", " 10 11   ", true},
		{"wide", "草", "草 ", "   ", " 10 11   ", true},
		{"multiple wide", "草草", "草草 ", "     ", "   10   11     ", true},
		{"combining", "e\u0301", "e\u0301 ", "  ", "1011  ", true},
		{"no color", "草草", "· ", "  ", "1011  ", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, weekdays := range []bool{false, true} {
				var b bytes.Buffer
				err := Render(&b, "alice", nil, 0, Options{
					From: date("2026-10-29"), To: date("2026-11-09"),
					Theme: "dark", Glyph: tc.glyph, Color: tc.color,
					ShowMonths: true, ShowWeekdays: weekdays,
				}, false)
				if err != nil {
					t.Fatal(err)
				}
				want := tc.months + "\n"
				if weekdays {
					want = "    " + want
				}
				for day, name := range []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"} {
					if weekdays {
						want += name + " "
					}
					if day < 4 {
						want += tc.blank
					} else {
						want += tc.visible
					}
					want += tc.visible
					if day > 1 {
						want += tc.blank
					} else {
						want += tc.visible
					}
					want += "\n"
				}
				if got := ansi.ReplaceAllString(b.String(), ""); got != want {
					t.Fatalf("weekdays=%t: got %q; want %q", weekdays, got, want)
				}
			}
		})
	}
}

func TestLeapDayAndDST(t *testing.T) {
	for _, tc := range []struct {
		timezone, from, to string
		count              int
	}{
		{"UTC", "2024-02-28", "2024-03-01", 3},
		{"America/New_York", "2026-03-07", "2026-03-10", 4},
		{"America/New_York", "2026-10-31", "2026-11-03", 4},
		{"UTC", "2026-12-31", "2027-01-01", 2},
	} {
		t.Run(tc.from, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.timezone)
			if err != nil {
				t.Fatal(err)
			}
			from, _ := time.ParseInLocation(time.DateOnly, tc.from, loc)
			to, _ := time.ParseInLocation(time.DateOnly, tc.to, loc)
			var b bytes.Buffer
			if err := Render(&b, "alice", nil, 0, Options{From: from, To: to}, false); err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(b.String(), "·"); got != tc.count {
				t.Fatalf("got %d days; want %d: %s", got, tc.count, b.String())
			}
		})
	}
}

func TestColorAndLevels(t *testing.T) {
	for _, tc := range []struct{ n, want int }{{0, 0}, {1, 1}, {2, 1}, {3, 2}, {5, 2}, {6, 3}, {9, 3}, {10, 4}, {100, 4}} {
		if got := level(tc.n); got != tc.want {
			t.Errorf("level(%d)=%d; want %d", tc.n, got, tc.want)
		}
	}
	for _, theme := range []string{"dark", "light"} {
		var b bytes.Buffer
		if err := Render(&b, "alice", nil, 0, Options{From: date("2026-10-01"), To: date("2026-10-01"), Theme: theme, Glyph: "●", Color: true}, false); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "\x1b[38;2;") || !strings.Contains(b.String(), "●\x1b[0m") {
			t.Fatal("missing colored custom glyph")
		}
		if strings.Contains(b.String(), "Total:") {
			t.Fatal("unexpected total")
		}
	}
}

func TestOKLCHPaletteColors(t *testing.T) {
	// Precomputed sRGB values from the OKLCH palettes, emitted as true color.
	for _, tc := range []struct {
		theme string
		count int
		want  string
	}{
		{"dark", 0, "35;31;34"},
		{"light", 0, "233;227;229"},
		{"dark", 1, "88;39;49"},
		{"light", 1, "255;192;201"},
		{"dark", 3, "146;53;75"},
		{"light", 3, "255;113;143"},
		{"dark", 6, "224;82;116"},
		{"light", 6, "212;89;116"},
		{"dark", 10, "255;135;158"},
		{"light", 10, "144;62;79"},
	} {
		got := cell(tc.count, Options{Theme: tc.theme, Glyph: "■", Color: true})
		want := "\x1b[38;2;" + tc.want + "m■\x1b[0m"
		if got != want {
			t.Errorf("cell(%d, %s) = %q; want %q", tc.count, tc.theme, got, want)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestWriteError(t *testing.T) {
	err := Render(failingWriter{}, "alice", nil, 0, Options{From: date("2026-10-01"), To: date("2026-10-01")}, false)
	if err == nil || err.Error() != "write failed" {
		t.Fatalf("got %v", err)
	}
}
