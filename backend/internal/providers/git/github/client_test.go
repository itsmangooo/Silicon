package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifySignature("secret", body, sig) {
		t.Fatal("valid signature rejected")
	}
	if VerifySignature("secret", body, sig+"00") {
		t.Fatal("invalid signature accepted")
	}
}

func TestParseRef(t *testing.T) {
	if branch, ok := ParseRef("refs/heads/feature/test"); !ok || branch != "feature/test" {
		t.Fatalf("got %q %v", branch, ok)
	}
	if _, ok := ParseRef("refs/tags/v1"); ok {
		t.Fatal("tag accepted")
	}
}
