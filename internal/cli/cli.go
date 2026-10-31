package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	_ "time/tzdata"
	"unicode"

	"github.com/furudenipa/anigrass/internal/annict"
	"github.com/furudenipa/anigrass/internal/grass"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string) error {
	return run(ctx, args, stdout, stderr, getenv, time.Now(), &annict.Client{})
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string, now time.Time, client *annict.Client) error {
	fs := flag.NewFlagSet("anigrass", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var username, fromText, toText, timezone, theme, glyph string
	var total, noColor bool
	var months, weeks int
	var display grass.Options
	fs.StringVar(&username, "user", "", "Annict username (default: authenticated user)")
	fs.StringVar(&username, "u", "", "Alias for --user")
	fs.StringVar(&fromText, "from", "", "Start date, inclusive (default: 364 days before --to)")
	fs.StringVar(&toText, "to", "", "End date, inclusive (default: today)")
	fs.IntVar(&months, "months", 0, "Last N months through today (1-12); incompatible with --weeks, --from, --to")
	fs.IntVar(&weeks, "weeks", 0, "Last N weeks through today (1-52); incompatible with --months, --from, --to")
	fs.StringVar(&timezone, "timezone", "Asia/Tokyo", "IANA timezone for daily counts (default: Asia/Tokyo)")
	fs.StringVar(&theme, "theme", "dark", "Color theme: dark or light (default: dark)")
	fs.StringVar(&theme, "t", "dark", "Alias for --theme")
	fs.StringVar(&glyph, "grass", "■", "Grass character in color mode (default: ■)")
	fs.StringVar(&glyph, "g", "■", "Alias for --grass")
	fs.BoolVar(&total, "total", false, "Show total episode records")
	fs.BoolVar(&display.ShowWeekdays, "show-weekdays", false, "Show weekday labels")
	fs.BoolVar(&display.ShowMonths, "show-months", false, "Show month labels")
	fs.BoolVar(&display.ShowUser, "show-user", false, "Show username")
	fs.BoolVar(&display.ShowRange, "show-range", false, "Show date range")
	fs.BoolVar(&display.ShowLegend, "show-legend", false, "Show color legend and count ranges")
	fs.BoolVar(&noColor, "no-color", false, "Use character shading instead of colors")
	fs.Usage = func() {
		printHelp(stderr, fs)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("positional arguments are not supported; use --user to specify a username")
	}
	specified := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { specified[f.Name] = true })
	if specified["months"] || specified["weeks"] {
		if specified["from"] || specified["to"] || (specified["months"] && specified["weeks"]) {
			return errors.New("--months and --weeks cannot be combined with each other or with --from or --to")
		}
		if specified["months"] && (months < 1 || months > 12) {
			return errors.New("--months must be between 1 and 12")
		}
		if specified["weeks"] && (weeks < 1 || weeks > 52) {
			return errors.New("--weeks must be between 1 and 52")
		}
	}
	if theme != "dark" && theme != "light" {
		return errors.New("--theme must be dark or light")
	}
	if strings.TrimSpace(glyph) == "" || strings.IndexFunc(glyph, unicode.IsControl) >= 0 {
		return errors.New("--grass must contain visible characters and no control characters")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("unknown timezone: %s", timezone)
	}
	to := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	if toText != "" {
		to, err = time.ParseInLocation(time.DateOnly, toText, loc)
		if err != nil {
			return errors.New("--to must be a valid date in YYYY-MM-DD format")
		}
	}
	from := to.AddDate(0, 0, -364)
	if weeks > 0 {
		from = to.AddDate(0, 0, 1-7*weeks)
	}
	if months > 0 {
		// Clamp to the target month's last day before excluding the boundary day.
		month := time.Date(to.Year(), to.Month()-time.Month(months), 1, 0, 0, 0, 0, loc)
		day := min(to.Day(), month.AddDate(0, 1, -1).Day())
		from = month.AddDate(0, 0, day)
	}
	if fromText != "" {
		from, err = time.ParseInLocation(time.DateOnly, fromText, loc)
		if err != nil {
			return errors.New("--from must be a valid date in YYYY-MM-DD format")
		}
	}
	if from.After(to) {
		return errors.New("--from must be on or before --to")
	}
	if to.After(from.AddDate(1, 0, 0)) {
		return errors.New("date range must not exceed one year; narrow it with --from and --to")
	}
	token := strings.TrimSpace(getenv("ANNICT_ACCESS_TOKEN"))
	if token == "" {
		return errors.New("set ANNICT_ACCESS_TOKEN to an Annict access token with read permission")
	}
	client.Token = token
	result, err := client.Fetch(ctx, strings.TrimSpace(username), from, to.AddDate(0, 0, 1))
	if err != nil {
		return err
	}
	color := !noColor && getenv("NO_COLOR") == "" && getenv("TERM") != "dumb"
	if f, ok := stdout.(*os.File); ok {
		if info, err := f.Stat(); err != nil || info.Mode()&os.ModeCharDevice == 0 {
			color = false
		}
	}
	display.From, display.To = from, to
	display.Theme, display.Glyph, display.Color = theme, glyph, color
	return grass.Render(stdout, result.Username, result.Days, result.Total, display, total)
}

func printHelp(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprint(w, "Display Annict episode records as a contribution calendar.\n\nUsage:\n  anigrass [options]\n\nBy default, only the seven grass rows are shown.\n\n")
	group := func(title string, entries ...[2]string) {
		fmt.Fprintln(w, title+":")
		tabs := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, entry := range entries {
			fmt.Fprintf(tabs, "  %s\t%s\n", entry[1], fs.Lookup(entry[0]).Usage)
		}
		tabs.Flush()
		fmt.Fprintln(w)
	}
	group("Data",
		[2]string{"user", "-u, --user NAME"},
		[2]string{"from", "    --from YYYY-MM-DD"},
		[2]string{"to", "    --to YYYY-MM-DD"},
		[2]string{"months", "    --months N"},
		[2]string{"weeks", "    --weeks N"},
		[2]string{"timezone", "    --timezone ZONE"})
	group("Appearance",
		[2]string{"theme", "-t, --theme THEME"},
		[2]string{"grass", "-g, --grass STRING"},
		[2]string{"no-color", "    --no-color"})
	group("Optional labels",
		[2]string{"show-user", "    --show-user"},
		[2]string{"show-range", "    --show-range"},
		[2]string{"show-months", "    --show-months"},
		[2]string{"show-weekdays", "    --show-weekdays"},
		[2]string{"show-legend", "    --show-legend"},
		[2]string{"total", "    --total"})
	fmt.Fprint(w, "Help:\n  -h, --help  Show help\n\nEnvironment:\n  ANNICT_ACCESS_TOKEN  Required Annict access token with read permission\n  NO_COLOR             Disable colors when nonempty\n\nExamples:\n  anigrass\n  anigrass --user frdnp --from 2026-01-01 --to 2026-12-31\n  anigrass --theme light --show-legend --total\n")
}
