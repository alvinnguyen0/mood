package web

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) routes() {
	r := chi.NewRouter()
	r.Use(s.recoverer, s.logger, s.csrfProtect)

	staticFS, _ := fs.Sub(assets, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Public
	r.Get("/login", s.loginForm)
	r.Post("/login", s.login)
	r.Get("/signup", s.signupForm)
	r.Post("/signup", s.signup)

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/", s.youPage)
		r.Get("/you", s.youPage)
		r.Post("/mood", s.logMood)
		r.Get("/everyone", s.everyonePage)
		r.Post("/logout", s.logout)
	})

	s.router = r
}
