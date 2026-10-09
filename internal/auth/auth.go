// Package auth kapselt Passwort-Hashing (Argon2id, ältere bcrypt-Hashes
// werden weiter erkannt), Sitzungs-Tokens (JWT, HS256) und die daraus
// abgeleiteten CSRF-Tokens, die Sperrliste abgemeldeter Sitzungen und die
// Bremse für Fehlanmeldungen.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"

	"iperf3-tracker/internal/model"
)

// DefaultSessionTTL ist die gleitende Gültigkeit einer Sitzung ohne
// Aktivität, wenn auth.session_ttl nicht gesetzt ist.
const DefaultSessionTTL = 12 * time.Hour

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

// MinPasswordLength ist die Mindestlänge neuer Passwörter in Zeichen.
// Bestehende kürzere Passwörter bleiben gültig, geprüft wird nur beim Setzen.
const MinPasswordLength = 10

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

var (
	dummyOnce sync.Once
	dummyVal  string
)

// DummyHash liefert einen Argon2id-Hash mit den aktuellen Parametern, gegen
// den bei unbekanntem Benutzer geprüft wird. So dauert eine Fehlanmeldung
// gleich lang wie bei einem vorhandenen Konto (kein Benutzer-Orakel über die
// Antwortzeit). Er wird erst beim ersten Bedarf berechnet.
func DummyHash() string {
	dummyOnce.Do(func() {
		h, err := HashPassword("iperf3-tracker-dummy")
		if err != nil {
			h = "$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}
		dummyVal = h
	})
	return dummyVal
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

// Claims ist der Inhalt eines Sitzungs-Tokens.
type Claims struct {
	// Username ist der angemeldete Benutzer (sub).
	Username string
	// Version ist die Token-Version des Benutzers beim Ausstellen (ver). Ein
	// Passwortwechsel oder „überall abmelden“ erhöht sie in der Datenbank und
	// macht damit alle älteren Tokens des Benutzers ungültig.
	Version int64
	// SessionID kennzeichnet die Sitzung (jti). Sie bleibt beim Verlängern
	// erhalten; an ihr hängen die Sperrliste und das CSRF-Token.
	SessionID string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// jwtClaims ist die JWT-Darstellung von Claims.
type jwtClaims struct {
	jwt.RegisteredClaims
	Version int64 `json:"ver"`
}

// Tokens erstellt und prüft Sitzungs-Tokens. Payload:
// {"sub": <username>, "ver": <token_version>, "jti": <sitzung>, "iat": <unix>, "exp": <unix>}.
type Tokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewTokens erstellt einen Token-Dienst mit dem angegebenen Signaturschlüssel
// und der gleitenden Sitzungsdauer ttl (<= 0: DefaultSessionTTL).
func NewTokens(secret string, ttl time.Duration) *Tokens {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &Tokens{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// TTL liefert die Sitzungsdauer.
func (t *Tokens) TTL() time.Duration { return t.ttl }

// Create stellt ein Token für eine neue Sitzung von username aus.
func (t *Tokens) Create(username string, version int64) (string, Claims, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", Claims{}, err
	}
	return t.Renew(Claims{Username: username, Version: version, SessionID: hex.EncodeToString(b)})
}

// Renew stellt für die Sitzung c ein Token mit voller Laufzeit aus
// (gleiche SessionID, Version aus c).
func (t *Tokens) Renew(c Claims) (string, Claims, error) {
	now := t.now().Truncate(time.Second)
	c.IssuedAt, c.ExpiresAt = now, now.Add(t.ttl)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.Username,
			ID:        c.SessionID,
			IssuedAt:  jwt.NewNumericDate(c.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(c.ExpiresAt),
		},
		Version: c.Version,
	}).SignedString(t.secret)
	return token, c, err
}

// NeedsRenewal meldet, ob mehr als die Hälfte der Laufzeit von c verstrichen
// ist. Dann verlängert der Server die Sitzung (gleitende Gültigkeit).
func (t *Tokens) NeedsRenewal(c Claims) bool {
	return t.now().After(c.IssuedAt.Add(t.ttl / 2))
}

// Parse prüft Signatur und Ablauf eines Tokens und liefert seinen Inhalt.
// Ob die Sitzung gesperrt oder die Version veraltet ist, prüft der Aufrufer.
func (t *Tokens) Parse(token string) (Claims, error) {
	var jc jwtClaims
	_, err := jwt.ParseWithClaims(token, &jc,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(t.now),
	)
	// Tokens ohne Sitzungskennung (frühere Versionen) gelten nicht mehr.
	if err != nil || jc.Subject == "" || jc.ID == "" || jc.IssuedAt == nil {
		return Claims{}, ErrInvalidToken
	}
	return Claims{
		Username:  jc.Subject,
		Version:   jc.Version,
		SessionID: jc.ID,
		IssuedAt:  jc.IssuedAt.Time,
		ExpiresAt: jc.ExpiresAt.Time,
	}, nil
}

// CSRF leitet das CSRF-Token einer Sitzung ab. Es ist an die Sitzungskennung
// gebunden, bleibt also beim Verlängern gleich, und muss bei ändernden
// Anfragen im Header X-CSRF-Token stehen.
func (t *Tokens) CSRF(sessionID string) string {
	m := hmac.New(sha256.New, t.secret)
	m.Write([]byte("csrf\x00"))
	m.Write([]byte(sessionID))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// CheckCSRF vergleicht ein übermitteltes CSRF-Token in konstanter Zeit.
func (t *Tokens) CheckCSRF(sessionID, got string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(t.CSRF(sessionID)), []byte(got)) == 1
}
