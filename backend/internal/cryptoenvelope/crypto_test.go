package cryptoenvelope

import (
	"bytes"
	"testing"
)

func TestSealOpenAndContextBinding(t *testing.T) {
	box, err := New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal([]byte("cloudflare-token"), "org-a")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("cloudflare-token")) {
		t.Fatal("plaintext leaked")
	}
	plain, err := box.Open(sealed, "org-a")
	if err != nil || string(plain) != "cloudflare-token" {
		t.Fatalf("open: %q %v", plain, err)
	}
	if _, err := box.Open(sealed, "org-b"); err == nil {
		t.Fatal("expected context authentication failure")
	}
}
