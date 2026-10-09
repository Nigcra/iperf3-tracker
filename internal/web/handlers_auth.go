package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

// checkPassword prüft ein Passwort gegen einen Hash (in Tests ersetzbar).
var checkPassword = auth.CheckPassword

// errTooManyAttempts ist die Antwort, solange eine Client-Adresse wegen zu
// vieler Fehlversuche gesperrt ist.
const errTooManyAttempts = "Zu viele Fehlversuche – bitte später erneut versuchen"

// tooManyAttempts antwortet mit 429, wenn die Client-Adresse gesperrt ist.
func (s *Server) tooManyAttempts(w http.ResponseWriter, r *http.Request, addr string) bool {
	if !s.limiter.Blocked(addr) {
		return false
	}
	slog.Warn("Anmeldung gesperrt – zu viele Fehlversuche", "path", r.URL.Path, "remote", addr)
	w.Header().Set("Retry-After", strconv.Itoa(int(auth.LoginWindow/time.Second)))
	writeError(w, r, http.StatusTooManyRequests, errTooManyAttempts)
	return true
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	addr := clientAddr(r)
	if s.tooManyAttempts(w, r, addr) {
		return
	}
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
	// Auch bei unbekanntem Benutzer einen Argon2id-Hash prüfen, damit die
	// Antwortzeit nicht verrät, ob es das Konto gibt.
	hash := auth.DummyHash()
	if u != nil {
		hash = u.HashedPassword
	}
	if !checkPassword(hash, req.Password) || u == nil {
		s.limiter.Fail(addr)
		slog.Warn("Anmeldung fehlgeschlagen", "user", req.Username, "remote", addr)
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

	token, claims, err := s.tokens.Create(u.Username, u.TokenVersion)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	// Die Oberfläche nutzt das HttpOnly-Cookie; access_token bleibt für API-Clients
	// (Authorization: Bearer) erhalten.
	setSessionCookie(w, r, token, s.tokens.TTL())
	writeJSON(w, r, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "bearer",
		"csrf_token":   s.tokens.CSRF(claims.SessionID),
		"user":         u,
	})
}

// handleLogout beendet die Sitzung der Anfrage (Cookie oder Bearer-Token):
// Ihre Kennung kommt bis zum spätestmöglichen Ablauf auf die Sperrliste, das
// Cookie wird gelöscht. Ohne gültiges Token wird nur das Cookie gelöscht.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token, _ := sessionToken(r); token != "" {
		if claims, err := s.tokens.Parse(token); err == nil {
			// Verlängerte Tokens derselben Sitzung laufen spätestens jetzt + TTL ab.
			if !s.revoked.Revoke(claims.SessionID, time.Now().Add(s.tokens.TTL())) {
				// Sperrliste voll: ersatzweise alle Sitzungen des Benutzers beenden.
				if u, err := s.store.UserByUsername(r.Context(), claims.Username); err == nil {
					if _, err := s.store.BumpTokenVersion(r.Context(), u.ID); err != nil {
						writeInternal(w, r, err)
						return
					}
				}
			}
		}
	}
	clearSessionCookies(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// handleLogoutAll beendet alle Sitzungen des angemeldeten Benutzers auf allen
// Geräten, auch Bearer-Tokens (Token-Version erhöhen).
func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request, u *model.User) {
	if _, err := s.store.BumpTokenVersion(r.Context(), u.ID); err != nil {
		writeInternal(w, r, err)
		return
	}
	slog.Info("Alle Sitzungen beendet", "user", u.Username, "remote", clientAddr(r))
	clearSessionCookies(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// handleAuthStatus liefert ohne Anmeldung den Zustand für die Anmeldeseite.
// initial_pending ist true, solange der Admin noch das beim ersten Start
// erzeugte Passwort hat; nur dann zeigt die Seite den Hinweis darauf.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	pending, err := s.initialPasswordPending(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"auth_enabled": true, "initial_pending": pending})
}

// handleMe liefert den angemeldeten Benutzer und im Header X-CSRF-Token das
// CSRF-Token der Sitzung (nach einem Neuladen der Seite).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *model.User) {
	if token, _ := sessionToken(r); token != "" {
		if claims, err := s.tokens.Parse(token); err == nil {
			w.Header().Set(csrfHeader, s.tokens.CSRF(claims.SessionID))
		}
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

// validatePassword prüft die Länge eines neuen Passworts. Bestehende kürzere
// Passwörter bleiben für die Anmeldung gültig.
func validatePassword(password string) string {
	switch {
	case utf8.RuneCountInString(password) < auth.MinPasswordLength:
		return fmt.Sprintf("Passwort muss mindestens %d Zeichen lang sein", auth.MinPasswordLength)
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

// handleChangePassword ändert das Passwort des angemeldeten Benutzers. Das
// aktuelle Passwort muss angegeben werden; ein falsches zählt als
// Fehlversuch. Danach ist ein erzwungener Wechsel erledigt und alle anderen
// Sitzungen des Benutzers sind beendet (Token-Version). Die eigene Sitzung
// läuft mit einem neuen Token weiter: als Cookie und für API-Clients als
// access_token in der Antwort.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, u *model.User) {
	addr := clientAddr(r)
	if s.tooManyAttempts(w, r, addr) {
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !checkPassword(u.HashedPassword, req.CurrentPassword) {
		s.limiter.Fail(addr)
		slog.Warn("Passwortwechsel: aktuelles Passwort falsch", "user", u.Username, "remote", addr)
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
	// Die Sitzung wurde in requireUserPending bereits geprüft.
	token, fromCookie := sessionToken(r)
	claims, err := s.tokens.Parse(token)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err == nil {
		claims.Version, err = s.store.ChangePassword(r.Context(), u.ID, hash)
	}
	if err == nil {
		token, claims, err = s.tokens.Renew(claims)
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if fromCookie {
		setSessionCookie(w, r, token, s.tokens.TTL())
	}
	slog.Info("Passwort geändert, andere Sitzungen beendet", "user", u.Username, "remote", addr)
	writeJSON(w, r, http.StatusOK, map[string]string{
		"message":      "Passwort geändert",
		"access_token": token,
		"token_type":   "bearer",
		"csrf_token":   s.tokens.CSRF(claims.SessionID),
	})
}
