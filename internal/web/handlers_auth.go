package web

import (
	"errors"
	"net/http"
	"unicode/utf8"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	u, err := s.store.UserByUsername(r.Context(), req.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeInternal(w, r, err)
		return
	}
	if u == nil || !auth.CheckPassword(u.HashedPassword, req.Password) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "Benutzername oder Passwort falsch")
		return
	}
	if !u.IsActive {
		writeError(w, http.StatusBadRequest, "Benutzer ist deaktiviert")
		return
	}

	now := db.Now()
	if err := s.store.SetLastLogin(r.Context(), u.ID, now); err != nil {
		writeInternal(w, r, err)
		return
	}
	u.LastLogin = &now

	token, err := s.tokens.Create(u.Username)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "bearer",
		"user":         u,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *model.User) {
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request, _ *model.User) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request, _ *model.User) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := validateNewUser(req.Username, req.Email, req.Password); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}

	ctx := r.Context()
	if _, err := s.store.UserByUsername(ctx, req.Username); err == nil {
		writeError(w, http.StatusBadRequest, "Benutzername ist bereits vergeben")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeInternal(w, r, err)
		return
	}
	if _, err := s.store.UserByEmail(ctx, req.Email); err == nil {
		writeError(w, http.StatusBadRequest, "E-Mail-Adresse ist bereits registriert")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeInternal(w, r, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	u := &model.User{
		Username:       req.Username,
		Email:          req.Email,
		HashedPassword: hash,
		IsActive:       true,
		IsAdmin:        req.IsAdmin,
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// validateNewUser prüft die Feldlängen wie das bisherige Pydantic-Schema
// (UserCreate); bcrypt begrenzt das Passwort zusätzlich auf 72 Byte.
func validateNewUser(username, email, password string) string {
	switch {
	case utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 50:
		return "Benutzername muss 3–50 Zeichen lang sein"
	case utf8.RuneCountInString(email) < 3 || utf8.RuneCountInString(email) > 100:
		return "E-Mail-Adresse muss 3–100 Zeichen lang sein"
	}
	return validatePassword(password)
}

// validatePassword prüft die Passwortlänge; bcrypt verarbeitet höchstens 72 Byte.
func validatePassword(password string) string {
	switch {
	case utf8.RuneCountInString(password) < 6:
		return "Passwort muss mindestens 6 Zeichen lang sein"
	case len(password) > 72:
		return "Passwort darf höchstens 72 Byte lang sein"
	}
	return ""
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, current *model.User) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	if id == current.ID {
		writeError(w, http.StatusBadRequest, "Das eigene Konto kann nicht gelöscht werden")
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	} else if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Benutzer gelöscht"})
}

func (s *Server) handleInitAdmin(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.CountUsers(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if n > 0 {
		writeError(w, http.StatusBadRequest, "Es existieren bereits Benutzer. Neue Benutzer über /auth/register anlegen.")
		return
	}
	admin, err := auth.DefaultAdmin()
	if err == nil {
		err = s.store.CreateUser(r.Context(), admin)
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message":  "Standard-Admin angelegt",
		"username": auth.DefaultAdminUsername,
		"password": auth.DefaultAdminPassword,
		"warning":  "Bitte das Passwort umgehend ändern!",
	})
}

// handleChangePassword ändert das Passwort des angemeldeten Benutzers. Das
// aktuelle Passwort muss angegeben werden.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, u *model.User) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !auth.CheckPassword(u.HashedPassword, req.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "Aktuelles Passwort ist falsch")
		return
	}
	if msg := validatePassword(req.NewPassword); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err == nil {
		err = s.store.SetPassword(r.Context(), u.ID, hash)
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Passwort geändert"})
}
