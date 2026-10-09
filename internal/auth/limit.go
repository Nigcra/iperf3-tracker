package auth

import (
	"sync"
	"time"
)

// Standardwerte der Bremse für Fehlanmeldungen: höchstens LoginMaxFailures
// Fehlversuche je Client-Adresse innerhalb von LoginWindow.
const (
	LoginMaxFailures = 10
	LoginWindow      = 5 * time.Minute
)

// Limiter bremst wiederholte Fehlanmeldungen je Client-Adresse.
// Fehlversuche verfallen nur mit der Zeit, nicht durch eine erfolgreiche
// Anmeldung: Sonst könnte jemand mit eigenem Konto den Zähler immer wieder
// zurücksetzen und weiter fremde Passwörter durchprobieren.
type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu    sync.Mutex
	fails map[string][]time.Time
}

// maxLimiterAddrs begrenzt den Speicherbedarf bei sehr vielen Adressen.
const maxLimiterAddrs = 10000

// NewLimiter erlaubt max Fehlversuche je Adresse innerhalb von window.
func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, now: time.Now, fails: map[string][]time.Time{}}
}

// Blocked meldet, ob die Adresse aktuell gesperrt ist.
func (l *Limiter) Blocked(addr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recentLocked(addr)) >= l.max
}

// Fail zählt einen Fehlversuch.
func (l *Limiter) Fail(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[addr] = append(l.recentLocked(addr), l.now())
	if len(l.fails) > maxLimiterAddrs {
		for k := range l.fails {
			l.recentLocked(k)
		}
	}
}

// recentLocked liefert die Fehlversuche im Zeitfenster und verwirft ältere.
func (l *Limiter) recentLocked(addr string) []time.Time {
	cut := l.now().Add(-l.window)
	in := l.fails[addr]
	out := in[:0]
	for _, t := range in {
		if t.After(cut) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		delete(l.fails, addr)
		return nil
	}
	l.fails[addr] = out
	return out
}

// Revocations ist die Sperrliste abgemeldeter Sitzungen (SessionID bis zum
// spätestmöglichen Ablauf ihrer Tokens). Sie liegt nur im Speicher: Nach
// einem Neustart gelten abgemeldete, noch nicht abgelaufene Tokens wieder.
type Revocations struct {
	now func() time.Time

	mu    sync.Mutex
	until map[string]time.Time
}

// maxRevocations begrenzt die Sperrliste. Ist sie voll, meldet Revoke false;
// der Aufrufer beendet dann alle Sitzungen des Benutzers (Token-Version).
const maxRevocations = 100000

// NewRevocations erzeugt eine leere Sperrliste.
func NewRevocations() *Revocations {
	return &Revocations{now: time.Now, until: map[string]time.Time{}}
}

// Revoke sperrt die Sitzung sessionID bis until. false heißt, die Liste ist
// auch nach dem Aufräumen voll und die Sitzung wurde nicht gesperrt.
func (r *Revocations) Revoke(sessionID string, until time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for id, t := range r.until {
		if !now.Before(t) {
			delete(r.until, id)
		}
	}
	if len(r.until) >= maxRevocations {
		return false
	}
	if until.After(r.until[sessionID]) {
		r.until[sessionID] = until
	}
	return true
}

// Revoked meldet, ob die Sitzung gesperrt ist.
func (r *Revocations) Revoked(sessionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.until[sessionID]
	return ok && r.now().Before(t)
}
