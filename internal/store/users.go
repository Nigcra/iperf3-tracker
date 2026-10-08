package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"iperf3-tracker/internal/db"
	"iperf3-tracker/internal/model"
)

const userColumns = `id, username, email, hashed_password, is_active, is_admin, created_at, last_login`

func scanUser(row rowScanner) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.HashedPassword, &u.IsActive, &u.IsAdmin,
		db.Time(&u.CreatedAt), db.NullTime(&u.LastLogin))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UserByID lädt einen Benutzer anhand seiner ID.
func (s *Store) UserByID(ctx context.Context, id int64) (*model.User, error) {
	return scanUser(s.db.QueryRowContext(noCancel(ctx), `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// UserByUsername lädt einen Benutzer anhand seines Benutzernamens.
func (s *Store) UserByUsername(ctx context.Context, username string) (*model.User, error) {
	return scanUser(s.db.QueryRowContext(noCancel(ctx), `SELECT `+userColumns+` FROM users WHERE username = ?`, username))
}

// UserByEmail lädt einen Benutzer anhand seiner E-Mail-Adresse.
func (s *Store) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	return scanUser(s.db.QueryRowContext(noCancel(ctx), `SELECT `+userColumns+` FROM users WHERE email = ?`, email))
}

// ListUsers liefert alle Benutzer, sortiert nach ID.
func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(noCancel(ctx), `SELECT `+userColumns+` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

// CountUsers liefert die Anzahl der Benutzer.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(noCancel(ctx), `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser speichert u und setzt ID und CreatedAt.
func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	u.CreatedAt = db.Now()
	res, err := s.db.ExecContext(noCancel(ctx),
		`INSERT INTO users (username, email, hashed_password, is_active, is_admin, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.HashedPassword, u.IsActive, u.IsAdmin, db.FormatTime(u.CreatedAt))
	if err != nil {
		return err
	}
	u.ID, err = res.LastInsertId()
	return err
}

// DeleteUser löscht den Benutzer mit der angegebenen ID.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	return affectedOne(s.db.ExecContext(noCancel(ctx), `DELETE FROM users WHERE id = ?`, id))
}

// SetLastLogin setzt den Zeitpunkt der letzten Anmeldung.
func (s *Store) SetLastLogin(ctx context.Context, id int64, t time.Time) error {
	_, err := s.db.ExecContext(noCancel(ctx), `UPDATE users SET last_login = ? WHERE id = ?`, db.FormatTime(t), id)
	return err
}

// SetPassword ersetzt den Passwort-Hash eines Benutzers.
func (s *Store) SetPassword(ctx context.Context, id int64, hash string) error {
	return affectedOne(s.db.ExecContext(noCancel(ctx), `UPDATE users SET hashed_password = ? WHERE id = ?`, hash, id))
}
