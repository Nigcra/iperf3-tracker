package web

import (
	"errors"
	"log/slog"
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
		writeError(w, r, http.StatusUnauthorized, "Benutzername oder Passwort falsch")
		return
	}
	if !u.IsActive {
		writeError(w, r, http.StatusBadRequest, "Benutzer ist deaktiviert")
		return
	}
	// Ältere bcrypt-Hashes bei erfolgreicher Anmeldung transparent auf Argon2id umstellen.
	if auth.NeedsRehash(u.HashedPassword) {
		if hash, err := auth.HashPassword(req.Password); err == nil {
			if err := s.store.SetPassword(r.Context(), u.ID, hash); err != nil {
				slog.Warn("Passwort-Hash konnte nicht aktualisiert werden", "user", u.Username, "error", err)
			} else {
				slog.Info("Passwort-Hash auf Argon2id umgestellt", "user", u.Username)
			}
		}
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
	// Die Oberfläche nutzt das HttpOnly-Cookie; access_token bleibt für API-Clients
	// (Authorization: Bearer) erhalten.
	setSessionCookie(w, r, token)
	writeJSON(w, r, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "bearer",
		"csrf_token":   s.tokens.CSRF(token),
		"user":         u,
	})
}

// handleLogout löscht das Sitzungs-Cookie. Das Token selbst bleibt bis zum
// Ablauf gültig (zustandslos), ist aber im Browser nicht mehr vorhanden.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookies(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// handleMe liefert den angemeldeten Benutzer und im Header X-CSRF-Token das
// CSRF-Token der Sitzung (nach einem Neuladen der Seite).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *model.User) {
	if token, _ := sessionToken(r); token != "" {
		w.Header().Set(csrfHeader, s.tokens.CSRF(token))
	}
	writeJSON(w, r, http.StatusOK, u)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request, _ *model.User) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, users)
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
		writeError(w, r, http.StatusUnprocessableEntity, msg)
		return
	}

	ctx := r.Context()
	if _, err := s.store.UserByUsername(ctx, req.Username); err == nil {
		writeError(w, r, http.StatusBadRequest, "Benutzername ist bereits vergeben")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeInternal(w, r, err)
		return
	}
	if _, err := s.store.UserByEmail(ctx, req.Email); err == nil {
		writeError(w, r, http.StatusBadRequest, "E-Mail-Adresse ist bereits registriert")
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
	writeJSON(w, r, http.StatusOK, u)
}

// validateNewUser prüft die Feldlängen.
func validateNewUser(username, email, password string) string {
	switch {
	case utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 50:
		return "Benutzername muss 3–50 Zeichen lang sein"
	case utf8.RuneCountInString(email) < 3 || utf8.RuneCountInString(email) > 100:
		return "E-Mail-Adresse muss 3–100 Zeichen lang sein"
	}
	return validatePassword(password)
}

// validatePassword prüft die Passwortlänge.
func validatePassword(password string) string {
	switch {
	case utf8.RuneCountInString(password) < 6:
		return "Passwort muss mindestens 6 Zeichen lang sein"
	case len(password) > auth.MaxPasswordBytes:
		return "Passwort darf höchstens 256 Byte lang sein"
	}
	return ""
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request, current *model.User) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	if id == current.ID {
		writeError(w, r, http.StatusBadRequest, "Das eigene Konto kann nicht gelöscht werden")
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	} else if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"message": "Benutzer gelöscht"})
}

// handleInitAdmin legt den ersten Admin an, solange kein Benutzer existiert.
// Das zufällige Passwort steht nur im Server-Log, nicht in der Antwort – der
// Aufruf ist ohne Anmeldung möglich.
func (s *Server) handleInitAdmin(w http.ResponseWriter, r *http.Request) {
	created, err := EnsureAdmin(r.Context(), s.store)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if !created {
		writeError(w, r, http.StatusBadRequest, "Es existieren bereits Benutzer. Neue Benutzer über /auth/register anlegen.")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{
		"message":  "Admin angelegt – das Passwort steht im Server-Log",
		"username": auth.DefaultAdminUsername,
		"warning":  "Das Passwort muss bei der ersten Anmeldung geändert werden",
	})
}

// handleChangePassword ändert das Passwort des angemeldeten Benutzers. Das
// aktuelle Passwort muss angegeben werden; ein erzwungener Wechsel ist danach erledigt.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, u *model.User) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !auth.CheckPassword(u.HashedPassword, req.CurrentPassword) {
		writeError(w, r, http.StatusBadRequest, "Aktuelles Passwort ist falsch")
		return
	}
	if msg := validatePassword(req.NewPassword); msg != "" {
		writeError(w, r, http.StatusUnprocessableEntity, msg)
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, r, http.StatusUnprocessableEntity, "Das neue Passwort muss sich vom bisherigen unterscheiden")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err == nil {
		err = s.store.ChangePassword(r.Context(), u.ID, hash)
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"message": "Passwort geändert"})
}
