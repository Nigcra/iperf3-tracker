package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestArgon2id(t *testing.T) {
	h, err := HashPassword("geheim")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("Hash-Format: %s", h)
	}
	if !CheckPassword(h, "geheim") || CheckPassword(h, "Geheim") || CheckPassword(h, "") {
		t.Error("Prüfung falsch")
	}
	if NeedsRehash(h) {
		t.Error("frischer Hash soll nicht neu gehasht werden")
	}
	h2, _ := HashPassword("geheim")
	if h == h2 {
		t.Error("Salz fehlt")
	}
	for _, broken := range []string{"$argon2id$", "$argon2id$v=19$m=1,t=1,p=1$!!$!!", "$argon2id$v=18$m=65536,t=3,p=4$AAAA$AAAA"} {
		if CheckPassword(broken, "geheim") {
			t.Errorf("%q akzeptiert", broken)
		}
	}
}

func TestBcryptStillAccepted(t *testing.T) {
	b, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(string(b), "admin123") || CheckPassword(string(b), "falsch") {
		t.Error("bcrypt-Prüfung falsch")
	}
	if !NeedsRehash(string(b)) {
		t.Error("bcrypt-Hash muss umgehasht werden")
	}
}

func TestGeneratePassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		p := GeneratePassword()
		if len(p) != 18 || strings.ContainsAny(p, "0O1lI") || seen[p] {
			t.Fatalf("Passwort %q", p)
		}
		seen[p] = true
	}
}

func TestCSRF(t *testing.T) {
	tok := NewTokens("s")
	a, b := tok.CSRF("sitzung-a"), tok.CSRF("sitzung-b")
	if a == b || !tok.CheckCSRF("sitzung-a", a) || tok.CheckCSRF("sitzung-a", b) || tok.CheckCSRF("sitzung-a", "") {
		t.Error("CSRF-Prüfung falsch")
	}
	if NewTokens("anders").CheckCSRF("sitzung-a", a) {
		t.Error("CSRF-Token unabhängig vom Schlüssel")
	}
}
