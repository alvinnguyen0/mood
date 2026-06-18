package service

import (
	"fmt"
	"html/template"
	"math"
	"sort"
	"time"

	"mood-tracker/internal/model"
)

const dateFmt = "2006-01-02"

// Palette colors for mood levels 1..5 (cool/low -> warm/high). Empty days use
// emptyColor. Colors are presentation only; mood_level is the source of truth.
// template.CSS bypasses html/template's CSS value filter (which rejects var()).
var palette = [5]template.CSS{"var(--mood-1)", "var(--mood-2)", "var(--mood-3)", "var(--mood-4)", "var(--mood-5)"}

const emptyColor template.CSS = "var(--empty)"

var faceEmoji = [5]string{"\U0001F641", "\U0001F615", "\U0001F610", "\U0001F642", "\U0001F604"}
var faceLabel = [5]string{"frown", "meh", "neutral", "slight smile", "big smile"}

// FaceInfo describes one mood level for the picker and legend.
type FaceInfo struct {
	Level int
	Emoji string
	Label string
}

// Faces returns the five mood levels in order.
func Faces() []FaceInfo {
	out := make([]FaceInfo, 5)
	for i := range out {
		out[i] = FaceInfo{Level: i + 1, Emoji: faceEmoji[i], Label: faceLabel[i]}
	}
	return out
}

// Face returns the emoji for a level (1..5), or "" if out of range.
func Face(level int) string {
	if level < 1 || level > 5 {
		return ""
	}
	return faceEmoji[level-1]
}

// Palette exposes the five band colors (low -> high) for the you-grid legend.
func Palette() []template.CSS { return palette[:] }

// SpectrumLegend returns n evenly-spaced colors across the red→green spectrum
// for use in the everyone-grid legend.
func SpectrumLegend(n int) []template.CSS {
	out := make([]template.CSS, n)
	for i := range out {
		v := 1 + float64(i)*4/float64(n-1)
		out[i] = colorForAvg(v)
	}
	return out
}

// colorForLevel maps an integer level [1,5] to the monochromatic palette used
// by the personal (you) grid.
func colorForLevel(v float64) template.CSS {
	if v <= 0 {
		return emptyColor
	}
	i := int(math.Round(v))
	if i < 1 {
		i = 1
	}
	if i > 5 {
		i = 5
	}
	return palette[i-1]
}

// colorForAvg maps a float average [1,5] to a continuous red→amber→green hue.
// Using hsl() directly lets fractional averages produce genuinely distinct colors.
func colorForAvg(v float64) template.CSS {
	if v <= 0 {
		return emptyColor
	}
	t := (v - 1) / 4
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	hue := t * 120 // 0° = red, 60° = amber, 120° = green
	return template.CSS(fmt.Sprintf("hsl(%.1f,65%%,48%%)", hue))
}

func parseDate(d string) time.Time {
	t, _ := time.Parse(dateFmt, d)
	return t
}

// --- "today" / timezone ---

func localNow(tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc)
}

// TodayStr returns the user-local calendar date "YYYY-MM-DD".
func TodayStr(tz string) string { return localNow(tz).Format(dateFmt) }

// --- grid ---

// Cell is one day in the activity grid.
type Cell struct {
	Date   string       // "YYYY-MM-DD"
	Color  template.CSS // CSS color (template.CSS bypasses html/template's CSS filter)
	Tip    string       // hover/tap text
	Future bool         // dates after today: rendered blank
}

// Grid is the GitHub-style activity grid: a slice of week-columns (each 7 days,
// index 0 = Sunday) plus a month-label row aligned to those columns.
type Grid struct {
	Weeks    [][7]Cell
	MonthRow []string
}

// assemble walks a whole number of weeks (Sunday..Saturday) ending in the week
// of `today`, going back ~52 weeks, and fills each day with `fill`.
func assemble(today string, fill func(ds string, future bool) Cell) Grid {
	end := parseDate(today)

	gridEnd := end
	for gridEnd.Weekday() != time.Saturday {
		gridEnd = gridEnd.AddDate(0, 0, 1)
	}
	start := end.AddDate(0, 0, -7*52)
	for start.Weekday() != time.Sunday {
		start = start.AddDate(0, 0, -1)
	}

	var weeks [][7]Cell
	var week [7]Cell
	for d := start; !d.After(gridEnd); d = d.AddDate(0, 0, 1) {
		wd := int(d.Weekday()) // Sunday=0 .. Saturday=6
		ds := d.Format(dateFmt)
		week[wd] = fill(ds, d.After(end))
		if wd == 6 {
			weeks = append(weeks, week)
			week = [7]Cell{}
		}
	}
	return Grid{Weeks: weeks, MonthRow: monthRow(weeks)}
}

func monthRow(weeks [][7]Cell) []string {
	out := make([]string, len(weeks))
	last := ""
	for i, w := range weeks {
		ds := w[0].Date // Sunday of the column
		if ds == "" {
			continue
		}
		ab := parseDate(ds).Format("Jan")
		if ab != last {
			out[i] = ab
			last = ab
		}
	}
	return out
}

// BuildGrid builds the personal grid from a date->level map.
func BuildGrid(today string, levels map[string]int) Grid {
	return assemble(today, func(ds string, future bool) Cell {
		c := Cell{Date: ds, Future: future}
		if future {
			return c
		}
		lv := levels[ds]
		c.Color = colorForLevel(float64(lv))
		if lv > 0 {
			c.Tip = fmt.Sprintf("%s \u00B7 %s %s", ds, faceEmoji[lv-1], faceLabel[lv-1])
		} else {
			c.Tip = ds + " \u00B7 no entry"
		}
		return c
	})
}

// BuildAvgGrid builds the Everyone grid from a date->average map.
func BuildAvgGrid(today string, avgs map[string]model.DayAverage) Grid {
	return assemble(today, func(ds string, future bool) Cell {
		c := Cell{Date: ds, Future: future}
		if future {
			return c
		}
		a, ok := avgs[ds]
		if !ok || a.Count == 0 {
			c.Color = emptyColor
			c.Tip = ds + " \u00B7 no entries"
			return c
		}
		c.Color = colorForAvg(a.Avg)
		c.Tip = fmt.Sprintf("%s \u00B7 avg %.1f \u00B7 %d %s", ds, a.Avg, a.Count, plural(a.Count, "person", "people"))
		return c
	})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// --- streaks ---

// Streaks computes the current and longest streaks from the set of logged
// dates. A streak is consecutive calendar days with any entry.
//
// Current streak rule (mirrors GitHub):
//   - anchor on today if today is logged;
//   - else anchor on yesterday, so an unlogged "today" does not break the
//     streak prematurely (the user just hasn't logged yet);
//   - if neither today nor yesterday is logged, the current streak is 0.
func Streaks(dates []string, today string) (current, longest int) {
	if len(dates) == 0 {
		return 0, 0
	}
	set := make(map[string]bool, len(dates))
	uniq := make([]string, 0, len(dates))
	for _, d := range dates {
		if !set[d] {
			set[d] = true
			uniq = append(uniq, d)
		}
	}
	// Lexical sort of "YYYY-MM-DD" is chronological.
	sort.Strings(uniq)

	run := 0
	var prev time.Time
	for i, d := range uniq {
		cur := parseDate(d)
		// Parsed dates are UTC midnights, so consecutive days differ by
		// exactly 24h (no DST edge cases).
		if i > 0 && cur.Sub(prev) == 24*time.Hour {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
		prev = cur
	}

	anchor := parseDate(today)
	if !set[today] {
		anchor = anchor.AddDate(0, 0, -1)
		if !set[anchor.Format(dateFmt)] {
			return 0, longest
		}
	}
	for set[anchor.Format(dateFmt)] {
		current++
		anchor = anchor.AddDate(0, 0, -1)
	}
	return current, longest
}
