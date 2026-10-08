// Package auth kapselt Passwort-Hashing (bcrypt) und Login-Tokens (JWT, HS256).
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"iperf3-tracker/internal/model"
)

// TokenTTL ist die Gültigkeitsdauer eines Login-Tokens.
const TokenTTL = 24 * time.Hour

// Zugangsdaten des Standard-Admins, der angelegt wird, solange kein Benutzer existiert.
const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin123"
	DefaultAdminEmail    = "admin@iperf-tracker.local"
)

// ErrInvalidToken wird für fehlerhafte, abgelaufene oder fremd signierte Tokens geliefert.
var ErrInvalidToken = errors.New("ungültiges Token")

// HashPassword erzeugt einen bcrypt-Hash. bcrypt verarbeitet höchstens 72 Byte.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword prüft password gegen einen bcrypt-Hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// DefaultAdmin baut den Standard-Admin inkl. Passwort-Hash (noch nicht gespeichert).
func DefaultAdmin() (*model.User, error) {
	hash, err := HashPassword(DefaultAdminPassword)
	if err != nil {
		return nil, err
	}
	return &model.User{
		Username:       DefaultAdminUsername,
		Email:          DefaultAdminEmail,
		HashedPassword: hash,
		IsActive:       true,
		IsAdmin:        true,
	}, nil
}

// Tokens erstellt und prüft Login-Tokens. Payload wie im Python-Backend:
// {"sub": <username>, "exp": <unix>}.
type Tokens struct {
	secret []byte
}

// NewTokens erstellt einen Token-Dienst mit dem angegebenen Signaturschlüssel.
func NewTokens(secret string) *Tokens { return &Tokens{secret: []byte(secret)} }

// Create stellt ein Token für username aus.
func (t *Tokens) Create(username string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   username,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenTTL)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

// Parse prüft ein Token und liefert den enthaltenen Benutzernamen.
func (t *Tokens) Parse(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
