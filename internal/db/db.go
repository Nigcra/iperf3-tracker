// Package db öffnet die SQLite-Datenbank und legt das Schema an.
// Genutzt wird der reine Go-Treiber modernc.org/sqlite (kein CGO erforderlich).
package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Open öffnet (bzw. erstellt) die SQLite-Datenbank unter path und legt das
// Schema an. Das Verzeichnis wird bei Bedarf erzeugt.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("Datenbankverzeichnis konnte nicht angelegt werden: %w", err)
	}

	// Die PRAGMAs gelten pro Verbindung; über die DSN setzt der Treiber sie
	// auf jeder neu geöffneten Verbindung.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("Datenbank konnte nicht geöffnet werden: %w", err)
	}
	conn.SetMaxOpenConns(1) // SQLite: Schreibzugriffe serialisieren

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Datenbankschema konnte nicht angelegt werden: %w", err)
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Datenbankschema konnte nicht aktualisiert werden: %w", err)
	}
	return conn, nil
}

// columnMigrations ergänzt Spalten, die in älteren Datenbanken fehlen
// (CREATE TABLE IF NOT EXISTS ändert bestehende Tabellen nicht).
var columnMigrations = []struct{ table, column, ddl string }{
	{"users", "must_change_password", `ALTER TABLE users ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT 0`},
}

func migrate(conn *sql.DB) error {
	for _, m := range columnMigrations {
		var n int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, m.table, m.column).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		if _, err := conn.Exec(m.ddl); err != nil {
			return fmt.Errorf("%s.%s: %w", m.table, m.column, err)
		}
	}
	return nil
}
