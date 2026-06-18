package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"mood-tracker/internal/model"
	"mood-tracker/internal/store"

	"golang.org/x/crypto/bcrypt"
)

const (
	maxEmailLen    = 254
	maxPasswordLen = 72 // bcrypt limit
)

const (
	sessionCookie = "session"
	sessionTTL    = 30 * 24 * time.Hour
)

// authData is the view model for the login/signup pages. Tab is empty so the
// layout hides the nav for logged-out users.
type authData struct {
	Tab       string
	LoggedIn  bool
	Error     string
	CSRFToken string
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login", "layout", authData{CSRFToken: csrfFrom(r.Context())})
}

func (s *Server) signupForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "signup", "layout", authData{CSRFToken: csrfFrom(r.Context())})
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pw := r.FormValue("password")
	tz := r.FormValue("tz")
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		tz = "UTC"
	}
	if email == "" || len(pw) < 8 {
		s.render(w, "signup", "layout", authData{Error: "enter an email and a password of at least 8 characters.", CSRFToken: csrfFrom(r.Context())})
		return
	}
	if len(email) > maxEmailLen || !isValidEmail(email) {
		s.render(w, "signup", "layout", authData{Error: "please enter a valid email address.", CSRFToken: csrfFrom(r.Context())})
		return
	}
	if len(pw) > maxPasswordLen {
		pw = pw[:maxPasswordLen]
	}

	_, err := s.store.UserByEmail(r.Context(), email)
	if err == nil {
		s.render(w, "signup", "layout", authData{Error: "that email is already registered.", CSRFToken: csrfFrom(r.Context())})
		return
	}
	if !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, err)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		s.serverError(w, err)
		return
	}
	u, err := s.store.CreateUser(r.Context(), email, string(hash), tz)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.startSession(w, r, u.ID)
	http.Redirect(w, r, "/you", http.StatusSeeOther)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pw := r.FormValue("password")
	if len(pw) > maxPasswordLen {
		pw = pw[:maxPasswordLen]
	}

	u, err := s.store.UserByEmail(r.Context(), email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pw)) != nil {
		s.render(w, "login", "layout", authData{Error: "incorrect email or password.", CSRFToken: csrfFrom(r.Context())})
		return
	}
	s.startSession(w, r, u.ID)
	http.Redirect(w, r, "/you", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/everyone", http.StatusSeeOther)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, uid int64) {
	tok := randToken()
	exp := time.Now().Add(sessionTTL)
	if err := s.store.CreateSession(r.Context(), model.Session{ID: tok, UserID: uid, ExpiresAt: exp}); err != nil {
		s.serverError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		Expires:  exp,
	})
}

// isValidEmail performs basic email format validation.
func isValidEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	domain := email[at+1:]
	return strings.Contains(domain, ".")
}

func randToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
