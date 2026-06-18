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

// --- trends ---

// Trends holds computed insights for the You page.
type Trends struct {
	WeekAvg     float64
	PrevWeekAvg float64
	WeekDelta   float64
	HasWeek     bool
	HasPrevWeek bool
	TopMood     int
	TopMoodPct  int
	DayOfWeek   [7]float64
	DayOfWeekN  [7]int
	DayOfWeekPct [7]int
	BestDay     string
	WorstDay    string
	HasDOW      bool
}

var dowNames = [7]string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

func ComputeTrends(today string, moods []model.MoodEntry) Trends {
	var t Trends
	if len(moods) == 0 {
		return t
	}

	td := parseDate(today)
	weekStart := td.AddDate(0, 0, -int(td.Weekday()))
	prevWeekStart := weekStart.AddDate(0, 0, -7)

	var weekSum, prevSum float64
	var weekN, prevN int
	counts := [5]int{}
	var dowSum [7]float64
	var dowN [7]int

	for _, m := range moods {
		d := parseDate(m.Date)
		counts[m.Level-1]++

		wd := int(d.Weekday())
		dowSum[wd] += float64(m.Level)
		dowN[wd]++

		if !d.Before(weekStart) && !d.After(td) {
			weekSum += float64(m.Level)
			weekN++
		} else if !d.Before(prevWeekStart) && d.Before(weekStart) {
			prevSum += float64(m.Level)
			prevN++
		}
	}

	if weekN > 0 {
		t.HasWeek = true
		t.WeekAvg = weekSum / float64(weekN)
	}
	if prevN > 0 {
		t.HasPrevWeek = true
		t.PrevWeekAvg = prevSum / float64(prevN)
	}
	if t.HasWeek && t.HasPrevWeek {
		t.WeekDelta = t.WeekAvg - t.PrevWeekAvg
	}

	maxIdx := 0
	for i, c := range counts {
		if c > counts[maxIdx] {
			maxIdx = i
		}
	}
	t.TopMood = maxIdx + 1
	t.TopMoodPct = counts[maxIdx] * 100 / len(moods)

	bestIdx, worstIdx := -1, -1
	var bestAvg, worstAvg float64
	for i := range dowN {
		if dowN[i] == 0 {
			continue
		}
		t.DayOfWeek[i] = dowSum[i] / float64(dowN[i])
		t.DayOfWeekN[i] = dowN[i]
		if bestIdx == -1 || t.DayOfWeek[i] > bestAvg {
			bestAvg = t.DayOfWeek[i]
			bestIdx = i
		}
		if worstIdx == -1 || t.DayOfWeek[i] < worstAvg {
			worstAvg = t.DayOfWeek[i]
			worstIdx = i
		}
	}
	if bestIdx >= 0 {
		t.HasDOW = true
		t.BestDay = dowNames[bestIdx]
		t.WorstDay = dowNames[worstIdx]
		for i := range t.DayOfWeek {
			t.DayOfWeekPct[i] = int(t.DayOfWeek[i] * 20)
		}
	}

	return t
}

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

// assembleRange walks whole weeks (Sunday..Saturday) from gridStart to the
// Saturday on or after today, filling each day with fill.
func assembleRange(gridStart, today string, fill func(ds string, future bool) Cell) Grid {
	end := parseDate(today)
	gridEnd := end
	for gridEnd.Weekday() != time.Saturday {
		gridEnd = gridEnd.AddDate(0, 0, 1)
	}
	start := parseDate(gridStart)
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

// assemble is assembleRange fixed at 52 weeks back from today.
func assemble(today string, fill func(ds string, future bool) Cell) Grid {
	start := parseDate(today).AddDate(0, 0, -7*52).Format(dateFmt)
	return assembleRange(start, today, fill)
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

// BuildUserGrid builds the personal grid, spanning from 2 weeks before the
// earliest logged entry to today (max 52 weeks). Returns an empty Grid when
// there are no entries so the caller can skip rendering entirely.
type UserEntry struct {
	Level int
	Note  string
}

func BuildUserGrid(today string, entries map[string]UserEntry) Grid {
	if len(entries) == 0 {
		return Grid{}
	}

	earliest := today
	for d := range entries {
		if d < earliest {
			earliest = d
		}
	}

	start := parseDate(earliest).AddDate(0, 0, -14)
	if cap52 := parseDate(today).AddDate(0, 0, -7*52); start.Before(cap52) {
		start = cap52
	}

	fill := func(ds string, future bool) Cell {
		c := Cell{Date: ds, Future: future}
		if future {
			return c
		}
		e, ok := entries[ds]
		if !ok {
			c.Color = emptyColor
			c.Tip = ds + " \u00B7 no entry"
			return c
		}
		c.Color = colorForLevel(float64(e.Level))
		c.Tip = fmt.Sprintf("%s \u00B7 %s %s", ds, faceEmoji[e.Level-1], faceLabel[e.Level-1])
		if e.Note != "" {
			c.Tip += " \u00B7 " + e.Note
		}
		return c
	}
	return assembleRange(start.Format(dateFmt), today, fill)
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
		c.Color = colorForLevel(a.Avg)
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
