package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/model"
	"iperf3-tracker/internal/store"
)

// Sitzungs-Cookie: unter TLS mit __Host--Präfix (nur mit Secure erlaubt),
// sonst ohne. Gelesen werden beide Namen.
const (
	sessionCookie       = "iperf3_session"
	sessionCookieSecure = "__Host-iperf3_session"
	csrfHeader          = "X-CSRF-Token"
)

// userHandler ist ein Handler, der den angemeldeten Benutzer erhält.
type userHandler func(w http.ResponseWriter, r *http.Request, u *model.User)

// requireUser verlangt eine gültige Sitzung (Cookie oder Bearer-Token). Ein
// Benutzer mit erzwungenem Passwortwechsel wird abgewiesen.
func (s *Server) requireUser(h userHandler) http.Handler {
	return s.withUser(h, false)
}

// requireUserPending lässt auch Benutzer mit ausstehendem Passwortwechsel zu
// (für /auth/me und /auth/change-password).
func (s *Server) requireUserPending(h userHandler) http.Handler {
	return s.withUser(h, true)
}

func (s *Server) withUser(h userHandler, allowPending bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.authenticate(w, r, allowPending)
		if !ok {
			return
		}
		h(w, r, u)
	})
}

// requireAdmin verlangt eine gültige Sitzung eines Admins.
func (s *Server) requireAdmin(h userHandler) http.Handler {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request, u *model.User) {
		if !u.IsAdmin {
			writeError(w, r, http.StatusForbidden, "Keine ausreichende Berechtigung")
			return
		}
		h(w, r, u)
	})
}

var (
	errUnauthorized   = errors.New("Anmeldedaten konnten nicht geprüft werden")
	errInactive       = errors.New("Benutzer ist deaktiviert")
	errCSRF           = errors.New("CSRF-Prüfung fehlgeschlagen – bitte die Seite neu laden")
	errMustChangePass = errors.New("Passwortwechsel erforderlich")
)

// session ist die geprüfte Sitzung einer Anfrage.
type session struct {
	token      string
	claims     auth.Claims
	fromCookie bool
}

// authenticate prüft die Sitzung der Anfrage und lädt den zugehörigen
// Benutzer. Schlägt das fehl, ist die Fehlerantwort bereits geschrieben und
// ok ist false. Eine Cookie-Sitzung, deren halbe Laufzeit verstrichen ist,
// wird dabei verlängert (gleitende Gültigkeit); Bearer-Tokens behalten ihre
// feste Laufzeit, der Client meldet sich danach neu an.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, allowPending bool) (u *model.User, ok bool) {
	u, sess, err := s.userForRequest(r, allowPending)
	if err == nil && sess.fromCookie && s.tokens.NeedsRenewal(sess.claims) {
		if token, _, rerr := s.tokens.Renew(sess.claims); rerr == nil {
			setSessionCookie(w, r, token, s.tokens.TTL())
		}
	}
	switch {
	case errors.Is(err, errUnauthorized):
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, r, http.StatusUnauthorized, err.Error())
	case errors.Is(err, errInactive):
		writeError(w, r, http.StatusBadRequest, err.Error())
	case errors.Is(err, errCSRF), errors.Is(err, errMustChangePass):
		writeError(w, r, http.StatusForbidden, err.Error())
	case err != nil:
		writeInternal(w, r, err)
	default:
		return u, true
	}
	return nil, false
}

// userForRequest prüft Sitzung, CSRF-Token (nur bei Cookie-Anmeldung und
// ändernden Methoden) und den Passwortwechsel-Zwang.
func (s *Server) userForRequest(r *http.Request, allowPending bool) (*model.User, session, error) {
	token, fromCookie := sessionToken(r)
	u, claims, err := s.userForToken(r.Context(), token)
	if err != nil {
		return nil, session{}, err
	}
	// Bearer-Tokens schickt der Browser nie von selbst mit, Cookies schon –
	// daher nur dort zusätzlich das CSRF-Token prüfen.
	if fromCookie && !safeMethod(r.Method) && !s.tokens.CheckCSRF(claims.SessionID, r.Header.Get(csrfHeader)) {
		return nil, session{}, errCSRF
	}
	if u.MustChangePassword && !allowPending {
		return nil, session{}, errMustChangePass
	}
	return u, session{token: token, claims: claims, fromCookie: fromCookie}, nil
}

// userForToken prüft token und lädt den zugehörigen, aktiven Benutzer.
// Abgemeldete Sitzungen und Tokens mit veralteter Token-Version (nach
// Passwortwechsel oder „überall abmelden“) gelten nicht. Fehler sind
// errUnauthorized, errInactive oder Datenbankfehler.
func (s *Server) userForToken(ctx context.Context, token string) (*model.User, auth.Claims, error) {
	if token == "" {
		return nil, auth.Claims{}, errUnauthorized
	}
	claims, err := s.tokens.Parse(token)
	if err != nil || s.revoked.Revoked(claims.SessionID) {
		return nil, auth.Claims{}, errUnauthorized
	}
	u, err := s.store.UserByUsername(ctx, claims.Username)
	if errors.Is(err, store.ErrNotFound) {
		return nil, auth.Claims{}, errUnauthorized
	}
	if err != nil {
		return nil, auth.Claims{}, err
	}
	if claims.Version != u.TokenVersion {
		return nil, auth.Claims{}, errUnauthorized
	}
	if !u.IsActive {
		return nil, auth.Claims{}, errInactive
	}
	return u, claims, nil
}

// clientAddr liefert die Adresse des Clients für die Bremse bei
// Fehlanmeldungen. Bewusst nur RemoteAddr: X-Forwarded-For ließe sich ohne
// konfigurierten vertrauenswürdigen Proxy beliebig fälschen.
func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// sessionToken liefert das Sitzungs-Token aus dem Authorization-Header
// (API-Clients) oder dem Sitzungs-Cookie (Oberfläche).
func sessionToken(r *http.Request) (token string, fromCookie bool) {
	if t := bearerToken(r); t != "" {
		return t, false
	}
	for _, name := range []string{sessionCookieSecure, sessionCookie} {
		if c, err := r.Cookie(name); err == nil && c.Value != "" {
			return c.Value, true
		}
	}
	return "", false
}

func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// isTLS meldet, ob die Anfrage per HTTPS kam – direkt oder über einen
// Reverse-Proxy, der X-Forwarded-Proto setzt.
func isTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// setSessionCookie setzt das Sitzungs-Cookie (HttpOnly, SameSite=Lax, unter
// TLS Secure mit __Host--Präfix). maxAge ist die Sitzungsdauer.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge time.Duration) {
	secure := isTLS(r)
	name := sessionCookie
	if secure {
		name = sessionCookieSecure
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(maxAge / time.Second),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookies löscht beide möglichen Sitzungs-Cookies.
func clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{sessionCookie, sessionCookieSecure} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
			Secure: name == sessionCookieSecure, SameSite: http.SameSiteLaxMode,
		})
	}
}

// crossOriginGuard weist ändernde Anfragen ab, die erkennbar von einer
// fremden Seite stammen (Sec-Fetch-Site bzw. Origin), wie
// net/http.CrossOriginProtection ab Go 1.25. Ergänzt das CSRF-Token und
// schützt zusätzlich den Login selbst.
func crossOriginGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !safeMethod(r.Method) && crossOrigin(r) {
			writeError(w, r, http.StatusForbidden, "Anfrage von fremder Herkunft abgelehnt")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func crossOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return false
	case "":
		// Ältere Browser oder Nicht-Browser-Clients: Origin prüfen, falls vorhanden.
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		u, err := url.Parse(origin)
		return err != nil || u.Host != r.Host
	default: // same-site, cross-site
		return true
	}
}

// securityHeaders setzt CSP und weitere Schutz-Header für alle Antworten.
// imgSrc enthält zusätzliche Bildquellen (eigener Kachelserver).
func securityHeaders(next http.Handler, imgSrc string) http.Handler {
	csp := "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:" + imgSrc +
		"; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
