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
	sessionCookie = "session"
	sessionTTL    = 30 * 24 * time.Hour
)

// authData is the view model for the login/signup pages. Tab is empty so the
// layout hides the nav for logged-out users.
type authData struct {
	Tab   string
	Error string
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login", "layout", authData{})
}

func (s *Server) signupForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "signup", "layout", authData{})
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pw := r.FormValue("password")
	tz := r.FormValue("tz")
	if tz == "" {
		tz = "UTC"
	}
	if email == "" || len(pw) < 8 {
		s.render(w, "signup", "layout", authData{Error: "Enter an email and a password of at least 8 characters."})
		return
	}

	_, err := s.store.UserByEmail(r.Context(), email)
	if err == nil {
		s.render(w, "signup", "layout", authData{Error: "That email is already registered."})
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	pw := r.FormValue("password")

	u, err := s.store.UserByEmail(r.Context(), email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pw)) != nil {
		s.render(w, "login", "layout", authData{Error: "Incorrect email or password."})
		return
	}
	s.startSession(w, r, u.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
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
		Secure:   r.TLS != nil, // set true in production (behind TLS)
		Expires:  exp,
	})
}

func randToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
