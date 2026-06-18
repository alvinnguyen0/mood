package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"mood-tracker/internal/model"
)

type ctxKey int

const (
	userKey  ctxKey = iota
	csrfKey
)

const csrfCookieName = "csrf"
const csrfTokenLen = 32

func userFrom(ctx context.Context) *model.User {
	u, _ := ctx.Value(userKey).(*model.User)
	return u
}

func csrfFrom(ctx context.Context) string {
	s, _ := ctx.Value(csrfKey).(string)
	return s
}

// csrfProtect implements the double-submit cookie pattern. On every request it
// ensures a CSRF cookie is present (generating one if needed) and stores the
// token in the request context. On POST requests it compares the cookie value
// to the "csrf_token" form field.
func (s *Server) csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		// Read or create the CSRF cookie.
		if c, err := r.Cookie(csrfCookieName); err == nil && len(c.Value) == csrfTokenLen*2 {
			token = c.Value
		} else {
			b := make([]byte, csrfTokenLen)
			if _, err := rand.Read(b); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			token = hex.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: false, // needs to be readable by forms, not JS
				SameSite: http.SameSiteLaxMode,
				Secure:   r.TLS != nil,
			})
		}

		// On POST, verify the form field matches the cookie.
		if r.Method == http.MethodPost {
			formToken := r.FormValue("csrf_token")
			if subtle.ConstantTimeCompare([]byte(token), []byte(formToken)) != 1 {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), csrfKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// loadUser reads the session cookie and populates the user in context if valid.
// Unlike requireAuth it does not redirect on missing/expired sessions.
func (s *Server) loadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			if sess, err := s.store.SessionByID(r.Context(), c.Value); err == nil && !sess.ExpiresAt.Before(time.Now()) {
				if u, err := s.store.UserByID(r.Context(), sess.UserID); err == nil {
					ctx := context.WithValue(r.Context(), userKey, u)
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth loads the session + user from the cookie, or redirects to /login.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		sess, err := s.store.SessionByID(r.Context(), c.Value)
		if err != nil || sess.ExpiresAt.Before(time.Now()) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		u, err := s.store.UserByID(r.Context(), sess.UserID)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
