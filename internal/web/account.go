package web

import (
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type accountData struct {
	Tab       string
	LoggedIn  bool
	Email     string
	Username  string
	Success   string
	Error     string
	CSRFToken string
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (s *Server) accountPage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	s.render(w, "account", "layout", accountData{
		Tab:       "account",
		LoggedIn:  true,
		Email:     u.Email,
		Username:  u.Username,
		CSRFToken: csrfFrom(r.Context()),
	})
}

func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	action := r.FormValue("action")
	csrf := csrfFrom(r.Context())

	base := accountData{
		Tab:       "account",
		LoggedIn:  true,
		Email:     u.Email,
		Username:  u.Username,
		CSRFToken: csrf,
	}

	switch action {
	case "profile":
		username := strings.TrimSpace(r.FormValue("username"))
		if username != "" {
			if len(username) < 2 || len(username) > 30 {
				base.Error = "username must be 2–30 characters."
				s.render(w, "account", "layout", base)
				return
			}
			if !usernameRe.MatchString(username) {
				base.Error = "username can only contain letters, numbers, hyphens, and underscores."
				s.render(w, "account", "layout", base)
				return
			}
		}
		if err := s.store.UpdateUsername(r.Context(), u.ID, username); err != nil {
			if strings.Contains(err.Error(), "idx_users_username") {
				base.Error = "that username is already taken."
				s.render(w, "account", "layout", base)
				return
			}
			s.serverError(w, err)
			return
		}
		base.Username = username
		base.Success = "username updated."
		s.render(w, "account", "layout", base)

	case "password":
		current := r.FormValue("current_password")
		newPw := r.FormValue("new_password")
		if len(newPw) < 8 {
			base.Error = "new password must be at least 8 characters."
			s.render(w, "account", "layout", base)
			return
		}
		if len(newPw) > maxPasswordLen {
			newPw = newPw[:maxPasswordLen]
		}
		if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(current)) != nil {
			base.Error = "current password is incorrect."
			s.render(w, "account", "layout", base)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
		if err != nil {
			s.serverError(w, err)
			return
		}
		if err := s.store.UpdatePassword(r.Context(), u.ID, string(hash)); err != nil {
			s.serverError(w, err)
			return
		}
		base.Success = "password changed."
		s.render(w, "account", "layout", base)

	default:
		http.Redirect(w, r, "/account", http.StatusSeeOther)
	}
}
