package web

import (
	"net/http"
	"strings"
)

type changelogEntry struct {
	Version string
	Date    string
	Changes []string
}

type changelogData struct {
	Tab       string
	LoggedIn  bool
	CSRFToken string
	Entries   []changelogEntry
}

func parseChangelog(raw string) []changelogEntry {
	var entries []changelogEntry
	var cur *changelogEntry
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			header := strings.TrimPrefix(line, "## ")
			parts := strings.SplitN(header, " · ", 2)
			e := changelogEntry{Version: parts[0]}
			if len(parts) == 2 {
				e.Date = parts[1]
			}
			entries = append(entries, e)
			cur = &entries[len(entries)-1]
		} else if strings.HasPrefix(line, "- ") && cur != nil {
			cur.Changes = append(cur.Changes, strings.TrimPrefix(line, "- "))
		}
	}
	return entries
}

func (s *Server) changelogPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "changelog", "layout", changelogData{
		Tab:       "",
		LoggedIn:  userFrom(r.Context()) != nil,
		CSRFToken: csrfFrom(r.Context()),
		Entries:   s.changelog,
	})
}
