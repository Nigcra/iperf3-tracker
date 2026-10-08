// Package store kapselt alle Datenbankzugriffe.
package store

import (
	"database/sql"
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	// ErrNotFound wird geliefert, wenn ein angefragter Datensatz nicht existiert.
	ErrNotFound = errors.New("Datensatz nicht gefunden")
	// ErrDuplicate wird geliefert, wenn ein eindeutiger Wert bereits vergeben ist.
	ErrDuplicate = errors.New("Wert bereits vergeben")
)

// mapUnique übersetzt eine UNIQUE-Verletzung in ErrDuplicate.
func mapUnique(err error) error {
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return ErrDuplicate
	}
	return err
}

// affectedOne liefert ErrNotFound, wenn die Anweisung keine Zeile betroffen hat.
func affectedOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Store bündelt die Datenbankzugriffe aller Entitäten.
type Store struct {
	db *sql.DB
}

// New erstellt einen Store auf einer bereits geöffneten Datenbank.
func New(conn *sql.DB) *Store { return &Store{db: conn} }

// rowScanner deckt *sql.Row und *sql.Rows ab.
type rowScanner interface {
	Scan(dest ...any) error
}
