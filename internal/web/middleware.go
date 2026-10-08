package web

import (
	"errors"
	"net/http"
	"strings"

	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

// userHandler ist ein Handler, der den angemeldeten Benutzer erhält.
type userHandler func(w http.ResponseWriter, r *http.Request, u *model.User)

// requireUser verlangt ein gültiges Bearer-Token.
func (s *Server) requireUser(h userHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.authenticate(w, r, bearerToken(r))
		if !ok {
			return
		}
		h(w, r, u)
	})
}

// requireAdmin verlangt ein gültiges Bearer-Token eines Admins.
func (s *Server) requireAdmin(h userHandler) http.Handler {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request, u *model.User) {
		if !u.IsAdmin {
			writeError(w, http.StatusForbidden, "Keine ausreichende Berechtigung")
			return
		}
		h(w, r, u)
	})
}

// authenticate prüft token und lädt den zugehörigen Benutzer. Schlägt das
// fehl, ist die Fehlerantwort bereits geschrieben und ok ist false.
// Wird auch für die SSE-Route genutzt, die das Token als Query-Parameter erhält.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, token string) (u *model.User, ok bool) {
	unauthorized := func() {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "Anmeldedaten konnten nicht geprüft werden")
	}
	if token == "" {
		unauthorized()
		return nil, false
	}
	username, err := s.tokens.Parse(token)
	if err != nil {
		unauthorized()
		return nil, false
	}
	u, err = s.store.UserByUsername(r.Context(), username)
	if errors.Is(err, store.ErrNotFound) {
		unauthorized()
		return nil, false
	}
	if err != nil {
		writeInternal(w, r, err)
		return nil, false
	}
	if !u.IsActive {
		writeError(w, http.StatusBadRequest, "Benutzer ist deaktiviert")
		return nil, false
	}
	return u, true
}

func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
