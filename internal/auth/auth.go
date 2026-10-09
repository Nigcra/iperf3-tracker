// Package auth kapselt Passwort-Hashing (Argon2id, ältere bcrypt-Hashes
// werden weiter erkannt), Sitzungs-Tokens (JWT, HS256) und die daraus
// abgeleiteten CSRF-Tokens.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"

	"iperf3-tracker/internal/model"
)

// TokenTTL ist die Gültigkeitsdauer einer Sitzung.
const TokenTTL = 24 * time.Hour

// Benutzername und E-Mail des ersten Admins, der angelegt wird, solange kein
// Benutzer existiert. Das Passwort wird zufällig erzeugt (GeneratePassword).
const (
	DefaultAdminUsername = "admin"
	DefaultAdminEmail    = "admin@iperf-tracker.local"
)

// LegacyDefaultPassword ist das feste Standardpasswort früherer Versionen.
// Ein Admin, der es noch nutzt, muss es bei der nächsten Anmeldung ändern.
const LegacyDefaultPassword = "admin123"

// MaxPasswordBytes begrenzt die Passwortlänge (Schutz vor sehr langen Eingaben).
const MaxPasswordBytes = 256

// ErrInvalidToken wird für fehlerhafte, abgelaufene oder fremd signierte Tokens geliefert.
var ErrInvalidToken = errors.New("ungültiges Token")

// Argon2id-Parameter nach RFC 9106, Abschnitt 4 (zweite Empfehlung):
// 64 MiB Speicher, 3 Durchläufe, 4 Threads.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// HashPassword erzeugt einen Argon2id-Hash im PHC-Format
// ($argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>).
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword prüft password gegen einen Argon2id- oder (ältere Konten)
// bcrypt-Hash.
func CheckPassword(hash, password string) bool {
	if strings.HasPrefix(hash, "$argon2id$") {
		p, err := parseArgon2id(hash)
		if err != nil {
			return false
		}
		key := argon2.IDKey([]byte(password), p.salt, p.time, p.memory, p.threads, uint32(len(p.key)))
		return subtle.ConstantTimeCompare(key, p.key) == 1
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NeedsRehash meldet, ob hash nicht mit den aktuellen Argon2id-Parametern
// erzeugt wurde (z. B. bcrypt) und nach erfolgreicher Anmeldung ersetzt werden sollte.
func NeedsRehash(hash string) bool {
	p, err := parseArgon2id(hash)
	if err != nil {
		return true
	}
	return p.time != argonTime || p.memory != argonMemory || p.threads != argonThreads || len(p.key) != argonKeyLen
}

type argonParams struct {
	time, memory uint32
	threads      uint8
	salt, key    []byte
}

func parseArgon2id(hash string) (argonParams, error) {
	var p argonParams
	parts := strings.Split(hash, "$")
	// "", "argon2id", "v=19", "m=…,t=…,p=…", salt, key
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, errors.New("kein Argon2id-Hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, errors.New("Argon2-Version nicht unterstützt")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return p, err
	}
	var err error
	if p.salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return p, err
	}
	if p.key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return p, err
	}
	if p.time == 0 || p.threads == 0 || len(p.key) == 0 {
		return p, errors.New("ungültige Argon2id-Parameter")
	}
	return p, nil
}

// passwordAlphabet enthält keine leicht verwechselbaren Zeichen (0/O, 1/l/I).
const passwordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GeneratePassword erzeugt ein zufälliges Passwort mit 18 Zeichen (rund 104 Bit).
func GeneratePassword() string {
	const n = 18
	out := make([]byte, n)
	buf := make([]byte, 1)
	for i := 0; i < n; {
		if _, err := rand.Read(buf); err != nil {
			panic("Zufallsgenerator nicht verfügbar: " + err.Error())
		}
		// Verwerfen statt Modulo, damit alle Zeichen gleich wahrscheinlich sind.
		if int(buf[0]) >= 256-256%len(passwordAlphabet) {
			continue
		}
		out[i] = passwordAlphabet[int(buf[0])%len(passwordAlphabet)]
		i++
	}
	return string(out)
}

// NewAdmin baut einen Admin mit dem angegebenen Passwort (noch nicht
// gespeichert). mustChange erzwingt den Passwortwechsel bei der ersten Anmeldung.
func NewAdmin(username, email, password string, mustChange bool) (*model.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return &model.User{
		Username:           username,
		Email:              email,
		HashedPassword:     hash,
		IsActive:           true,
		IsAdmin:            true,
		MustChangePassword: mustChange,
	}, nil
}

// Tokens erstellt und prüft Sitzungs-Tokens. Payload:
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

// CSRF leitet das CSRF-Token einer Sitzung ab. Es ist an das Sitzungs-Token
// gebunden und muss bei ändernden Anfragen im Header X-CSRF-Token stehen.
func (t *Tokens) CSRF(sessionToken string) string {
	m := hmac.New(sha256.New, t.secret)
	m.Write([]byte("csrf\x00"))
	m.Write([]byte(sessionToken))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// CheckCSRF vergleicht ein übermitteltes CSRF-Token in konstanter Zeit.
func (t *Tokens) CheckCSRF(sessionToken, got string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(t.CSRF(sessionToken)), []byte(got)) == 1
}
