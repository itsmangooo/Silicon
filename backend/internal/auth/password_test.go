package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected Argon2id hash, got %q", hash)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected correct password to verify")
	}
	if VerifyPassword(hash, "incorrect horse battery staple") {
		t.Fatal("incorrect password verified")
	}
}

func TestPasswordLength(t *testing.T) {
	if _, err := HashPassword("too-short"); err == nil {
		t.Fatal("expected short password to be rejected")
	}
}

func TestMalformedHashesFailClosed(t *testing.T) {
	values := []string{"", "plaintext", "$argon2id$v=19$m=999999999,t=1,p=1$bad$bad"}
	for _, value := range values {
		if VerifyPassword(value, "password") {
			t.Fatalf("malformed hash %q verified", value)
		}
	}
}
