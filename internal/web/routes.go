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

	// Public (no auth required)
	r.Get("/login", s.loginForm)
	r.Post("/login", s.login)
	r.Get("/signup", s.signupForm)
	r.Post("/signup", s.signup)

	// Public with optional session load
	r.Group(func(r chi.Router) {
		r.Use(s.loadUser)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/everyone", http.StatusSeeOther)
		})
		r.Get("/everyone", s.everyonePage)
		r.Get("/changelog", s.changelogPage)
	})

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/you", s.youPage)
		r.Post("/mood", s.logMood)
		r.Get("/account", s.accountPage)
		r.Post("/account", s.updateAccount)
		r.Post("/logout", s.logout)
	})

	s.router = r
}
