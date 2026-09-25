package auth

import (
	"bytes"
	"testing"
)

func TestSessionTokensAreRandomAndHashable(t *testing.T) {
	first, firstHash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || bytes.Equal(firstHash, secondHash) {
		t.Fatal("tokens must be unique")
	}
	if !bytes.Equal(firstHash, HashToken(first)) {
		t.Fatal("token hash is not stable")
	}
}
