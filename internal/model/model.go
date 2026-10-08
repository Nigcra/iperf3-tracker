// Package model enthält die Datenstrukturen der Anwendung. Die JSON-Tags
// entsprechen 1:1 den Feldnamen der bisherigen Python-API (snake_case);
// optionale Felder sind Pointer und werden als null serialisiert.
package model

import "time"

// User ist ein Benutzerkonto.
type User struct {
	ID             int64      `json:"id"`
	Username       string     `json:"username"`
	Email          string     `json:"email"`
	HashedPassword string     `json:"-"`
	IsActive       bool       `json:"is_active"`
	IsAdmin        bool       `json:"is_admin"`
	CreatedAt      time.Time  `json:"created_at"`
	LastLogin      *time.Time `json:"last_login"`
}
