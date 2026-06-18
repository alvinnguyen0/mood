package web

import (
	"html/template"
	"net/http"
	"strconv"

	"mood-tracker/internal/model"
	"mood-tracker/internal/service"
)

// todayCard is the picker + "logged today" state. Embedded into youData so the
// same fields render both inline and as an htmx fragment.
type todayCard struct {
	Today      string
	TodayLevel int
	Faces      []service.FaceInfo
	CSRFToken  string
}

type youData struct {
	Tab      string
	LoggedIn bool
	Legend   []template.CSS
	todayCard
	Grid       service.Grid
	Current    int
	Longest    int
	EntryCount int
}

type everyoneData struct {
	Tab        string
	LoggedIn   bool
	Legend     []template.CSS
	Grid       service.Grid
	EntryCount int
	CSRFToken  string
}

func (s *Server) youPage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	today := service.TodayStr(u.Timezone)

	moods, err := s.store.MoodsForUser(r.Context(), u.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	levels := make(map[string]int, len(moods))
	dates := make([]string, 0, len(moods))
	for _, m := range moods {
		levels[m.Date] = m.Level
		dates = append(dates, m.Date)
	}

	cur, longest := service.Streaks(dates, today)
	csrf := csrfFrom(r.Context())
	data := youData{
		Tab:        "you",
		LoggedIn:   true,
		Legend:     service.Palette(),
		todayCard:  todayCard{Today: today, TodayLevel: levels[today], Faces: service.Faces(), CSRFToken: csrf},
		Grid:       service.BuildUserGrid(today, levels),
		Current:    cur,
		Longest:    longest,
		EntryCount: len(moods),
	}
	s.render(w, "you", "layout", data)
}

func (s *Server) logMood(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	lvl, err := strconv.Atoi(r.FormValue("level"))
	if err != nil || lvl < 1 || lvl > 5 {
		http.Error(w, "invalid mood level", http.StatusBadRequest)
		return
	}
	today := service.TodayStr(u.Timezone)
	if err := s.store.UpsertMood(r.Context(), u.ID, today, lvl); err != nil {
		s.serverError(w, err)
		return
	}

	// htmx: swap just the today card. Plain form: redirect (Post/Redirect/Get).
	if r.Header.Get("HX-Request") == "true" {
		s.render(w, "you", "todaycard", todayCard{Today: today, TodayLevel: lvl, Faces: service.Faces(), CSRFToken: csrfFrom(r.Context())})
		return
	}
	http.Redirect(w, r, "/you", http.StatusSeeOther)
}

func (s *Server) everyonePage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())

	rows, err := s.store.DailyAverages(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	avgs := make(map[string]model.DayAverage, len(rows))
	for _, a := range rows {
		avgs[a.Date] = a
	}

	tz := "UTC"
	if u != nil {
		tz = u.Timezone
	}
	today := service.TodayStr(tz)
	data := everyoneData{
		Tab:        "everyone",
		LoggedIn:   u != nil,
		Legend:     service.Palette(),
		Grid:       service.BuildAvgGrid(today, avgs),
		EntryCount: len(rows),
		CSRFToken:  csrfFrom(r.Context()),
	}
	s.render(w, "everyone", "layout", data)
}
