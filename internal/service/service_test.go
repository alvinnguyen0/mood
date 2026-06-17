package service

import (
	"testing"

	"mood-tracker/internal/model"
)

const testToday = "2025-03-15" // a Saturday

// --- Streaks tests ---

func TestStreaks_Empty(t *testing.T) {
	cur, long := Streaks(nil, testToday)
	if cur != 0 || long != 0 {
		t.Fatalf("got (%d,%d), want (0,0)", cur, long)
	}
}

func TestStreaks_SingleDayToday(t *testing.T) {
	cur, long := Streaks([]string{"2025-03-15"}, testToday)
	if cur != 1 || long != 1 {
		t.Fatalf("got (%d,%d), want (1,1)", cur, long)
	}
}

func TestStreaks_SingleDayYesterday(t *testing.T) {
	// Unlogged today should not break the streak.
	cur, long := Streaks([]string{"2025-03-14"}, testToday)
	if cur != 1 || long != 1 {
		t.Fatalf("got (%d,%d), want (1,1)", cur, long)
	}
}

func TestStreaks_SingleDayTwoDaysAgo(t *testing.T) {
	cur, long := Streaks([]string{"2025-03-13"}, testToday)
	if cur != 0 || long != 1 {
		t.Fatalf("got (%d,%d), want (0,1)", cur, long)
	}
}

func TestStreaks_ConsecutiveIncludingToday(t *testing.T) {
	dates := []string{"2025-03-12", "2025-03-13", "2025-03-14", "2025-03-15"}
	cur, long := Streaks(dates, testToday)
	if cur != 4 || long != 4 {
		t.Fatalf("got (%d,%d), want (4,4)", cur, long)
	}
}

func TestStreaks_GapInMiddle(t *testing.T) {
	// Run1: 5 days, Run2 (recent, includes today): 3 days.
	dates := []string{
		"2025-03-01", "2025-03-02", "2025-03-03", "2025-03-04", "2025-03-05",
		// gap
		"2025-03-13", "2025-03-14", "2025-03-15",
	}
	cur, long := Streaks(dates, testToday)
	if cur != 3 {
		t.Fatalf("current: got %d, want 3", cur)
	}
	if long != 5 {
		t.Fatalf("longest: got %d, want 5", long)
	}
}

func TestStreaks_Duplicates(t *testing.T) {
	dates := []string{"2025-03-14", "2025-03-14", "2025-03-15", "2025-03-15"}
	cur, long := Streaks(dates, testToday)
	if cur != 2 || long != 2 {
		t.Fatalf("got (%d,%d), want (2,2)", cur, long)
	}
}

// --- BuildGrid tests ---

func TestBuildGrid_EmptyLevels(t *testing.T) {
	g := BuildGrid(testToday, map[string]int{})
	for _, w := range g.Weeks {
		for _, c := range w {
			if c.Future {
				continue
			}
			if c.Color != emptyColor {
				t.Fatalf("date %s: got color %s, want %s", c.Date, c.Color, emptyColor)
			}
		}
	}
}

func TestBuildGrid_KnownDate(t *testing.T) {
	levels := map[string]int{"2025-03-10": 3}
	g := BuildGrid(testToday, levels)
	var found bool
	for _, w := range g.Weeks {
		for _, c := range w {
			if c.Date == "2025-03-10" {
				found = true
				wantColor := palette[2] // level 3 → index 2
				if c.Color != wantColor {
					t.Fatalf("color: got %s, want %s", c.Color, wantColor)
				}
				wantTip := "2025-03-10 · \U0001F610 Neutral"
				if c.Tip != wantTip {
					t.Fatalf("tip: got %q, want %q", c.Tip, wantTip)
				}
			}
		}
	}
	if !found {
		t.Fatal("date 2025-03-10 not found in grid")
	}
}

// --- BuildAvgGrid tests ---

func TestBuildAvgGrid_Empty(t *testing.T) {
	g := BuildAvgGrid(testToday, map[string]model.DayAverage{})
	for _, w := range g.Weeks {
		for _, c := range w {
			if c.Future {
				continue
			}
			if c.Color != emptyColor {
				t.Fatalf("date %s: got color %s, want %s", c.Date, c.Color, emptyColor)
			}
		}
	}
}

func TestBuildAvgGrid_KnownDate(t *testing.T) {
	avgs := map[string]model.DayAverage{
		"2025-03-10": {Date: "2025-03-10", Avg: 3.7, Count: 5},
	}
	g := BuildAvgGrid(testToday, avgs)
	var found bool
	for _, w := range g.Weeks {
		for _, c := range w {
			if c.Date == "2025-03-10" {
				found = true
				wantColor := palette[3] // round(3.7)=4 → index 3
				if c.Color != wantColor {
					t.Fatalf("color: got %s, want %s", c.Color, wantColor)
				}
				wantTip := "2025-03-10 · avg 3.7 · 5 people"
				if c.Tip != wantTip {
					t.Fatalf("tip: got %q, want %q", c.Tip, wantTip)
				}
			}
		}
	}
	if !found {
		t.Fatal("date 2025-03-10 not found in grid")
	}
}
