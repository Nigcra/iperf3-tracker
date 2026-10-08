// Package store kapselt alle Datenbankzugriffe.
package store

import (
	"database/sql"
	"errors"
)

// ErrNotFound wird geliefert, wenn ein angefragter Datensatz nicht existiert.
var ErrNotFound = errors.New("Datensatz nicht gefunden")

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
