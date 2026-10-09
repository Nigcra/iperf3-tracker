package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"iperf3-tracker/internal/auth"
	"iperf3-tracker/internal/store"
)

// EnsureAdmin legt den ersten Admin an, solange kein Benutzer existiert. Das
// Passwort wird zufällig erzeugt, einmal deutlich ins Log geschrieben und muss
// bei der ersten Anmeldung geändert werden. created meldet, ob angelegt wurde.
func EnsureAdmin(ctx context.Context, st *store.Store) (created bool, err error) {
	created, password, err := createFirstAdmin(ctx, st)
	if err != nil || !created {
		return created, err
	}
	slog.Warn("Erster Admin angelegt – Passwort notieren, es wird nur dieses eine Mal angezeigt und muss bei der ersten Anmeldung geändert werden",
		"user", auth.DefaultAdminUsername, "initial_password", password)
	if InitialPasswordHook != nil {
		InitialPasswordHook(auth.DefaultAdminUsername, password)
	}
	return true, nil
}

// InitialPasswordHook wird nach dem Anlegen des ersten Admins aufgerufen
// (z. B. um das Passwort im Dienstbetrieb zusätzlich in eine Datei zu schreiben).
var InitialPasswordHook func(username, password string)

func createFirstAdmin(ctx context.Context, st *store.Store) (bool, string, error) {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return false, "", fmt.Errorf("Benutzer konnten nicht gezählt werden: %w", err)
	}
	if n > 0 {
		return false, "", nil
	}
	password := auth.GeneratePassword()
	admin, err := auth.NewAdmin(auth.DefaultAdminUsername, auth.DefaultAdminEmail, password, true)
	if err != nil {
		return false, "", err
	}
	if err := st.CreateUser(ctx, admin); err != nil {
		return false, "", fmt.Errorf("Admin konnte nicht angelegt werden: %w", err)
	}
	return true, password, nil
}

// FlagLegacyDefaultPassword erzwingt den Passwortwechsel für den Admin, wenn
// er noch das feste Standardpasswort früherer Versionen (admin123) nutzt.
func FlagLegacyDefaultPassword(ctx context.Context, st *store.Store) error {
	u, err := st.UserByUsername(ctx, auth.DefaultAdminUsername)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.MustChangePassword || !auth.CheckPassword(u.HashedPassword, auth.LegacyDefaultPassword) {
		return nil
	}
	if err := st.SetMustChangePassword(ctx, u.ID, true); err != nil {
		return err
	}
	slog.Warn("Admin nutzt noch das frühere Standardpasswort – Wechsel bei der nächsten Anmeldung erzwungen", "user", u.Username)
	return nil
}

// legacyPasswordCache merkt sich, ob ein Admin-Hash das frühere
// Standardpasswort enthält. So rechnet der öffentliche Aufruf
// /api/auth/status Argon2id nur einmal je Hash statt bei jeder Anfrage.
type legacyPasswordCache struct {
	mu     sync.Mutex
	hash   string
	legacy bool
}

func (c *legacyPasswordCache) isLegacy(hash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hash != hash {
		c.hash, c.legacy = hash, auth.CheckPassword(hash, auth.LegacyDefaultPassword)
	}
	return c.legacy
}

// initialPasswordPending meldet, ob der Admin noch das beim ersten Start
// erzeugte Passwort hat: Passwortwechsel ausstehend, aber nicht wegen des
// früheren Standardpassworts (das wurde nie erzeugt und angezeigt). Nach dem
// Wechsel bleibt es false, weil ChangePassword must_change_password aufhebt.
func (s *Server) initialPasswordPending(ctx context.Context) (bool, error) {
	u, err := s.store.UserByUsername(ctx, auth.DefaultAdminUsername)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return u.MustChangePassword && !s.legacyAdmin.isLegacy(u.HashedPassword), nil
}
