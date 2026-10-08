package web

import (
	"context"
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

var (
	errUnauthorized = errors.New("Anmeldedaten konnten nicht geprüft werden")
	errInactive     = errors.New("Benutzer ist deaktiviert")
)

// authenticate prüft token und lädt den zugehörigen Benutzer. Schlägt das
// fehl, ist die Fehlerantwort bereits geschrieben und ok ist false.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, token string) (u *model.User, ok bool) {
	u, err := s.userForToken(r.Context(), token)
	switch {
	case errors.Is(err, errUnauthorized):
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, errInactive):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeInternal(w, r, err)
	default:
		return u, true
	}
	return nil, false
}

// userForToken prüft token und lädt den zugehörigen, aktiven Benutzer.
// Fehler sind errUnauthorized, errInactive oder Datenbankfehler.
func (s *Server) userForToken(ctx context.Context, token string) (*model.User, error) {
	if token == "" {
		return nil, errUnauthorized
	}
	username, err := s.tokens.Parse(token)
	if err != nil {
		return nil, errUnauthorized
	}
	u, err := s.store.UserByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		return nil, errInactive
	}
	return u, nil
}

func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
