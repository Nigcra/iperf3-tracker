package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateOldUsersTable öffnet eine Datenbank im Schema früherer Versionen
// (ohne must_change_password) und prüft, dass die Spalte ergänzt wird.
func TestMigrateOldUsersTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alt.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT, username VARCHAR(50) NOT NULL UNIQUE,
		email VARCHAR(100) NOT NULL UNIQUE, hashed_password VARCHAR(255) NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT 1, is_admin BOOLEAN NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL, last_login TEXT);
		INSERT INTO users (username, email, hashed_password, created_at) VALUES ('a', 'a@x', 'h', '2024-01-01T00:00:00Z');`); err != nil {
		t.Fatal(err)
	}
	old.Close()

	conn, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var must bool
	if err := conn.QueryRow(`SELECT must_change_password FROM users WHERE username = 'a'`).Scan(&must); err != nil || must {
		t.Fatalf("must_change_password = %v, %v", must, err)
	}
	// Ein zweites Öffnen darf die Spalte nicht erneut anlegen wollen.
	conn.Close()
	conn2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	conn2.Close()
}
