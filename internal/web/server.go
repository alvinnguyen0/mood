package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"

	"mood-tracker/internal/service"
	"mood-tracker/internal/store"
)

//go:embed templates static
var assets embed.FS

// Server holds shared dependencies and the routed handler. Handlers are
// methods on *Server so they share the store and parsed templates.
type Server struct {
	store  store.Store
	pages  map[string]*template.Template
	router http.Handler
}

func New(st store.Store) (*Server, error) {
	s := &Server{store: st}
	if err := s.parseTemplates(); err != nil {
		return nil, err
	}
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// parseTemplates builds one template set per page (layout + page + partials),
// so each page's {{define "content"}} stays isolated.
func (s *Server) parseTemplates() error {
	funcs := template.FuncMap{
		"face":    service.Face,
		"palette": service.Palette,
	}
	partials := []string{
		"templates/partials/todaycard.html",
		"templates/partials/grid.html",
		"templates/partials/legend.html",
	}
	pages := []string{"you", "everyone", "login", "signup"}
	s.pages = map[string]*template.Template{}
	for _, p := range pages {
		files := append([]string{"templates/layout.html", "templates/" + p + ".html"}, partials...)
		t, err := template.New("base").Funcs(funcs).ParseFS(assets, files...)
		if err != nil {
			return err
		}
		s.pages[p] = t
	}
	return nil
}

// render executes a named template ("layout" for full pages, or a partial
// name like "todaycard" for htmx fragments) from the given page's set.
func (s *Server) render(w http.ResponseWriter, page, name string, data any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s/%s: %v", page, name, err)
	}
}

func (s *Server) serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
