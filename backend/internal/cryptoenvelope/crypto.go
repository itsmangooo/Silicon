package cryptoenvelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const version byte = 1

var ErrKeyUnavailable = errors.New("integration encryption is not configured")

type Box struct{ key []byte }

func New(key []byte) (Box, error) {
	if len(key) != 32 {
		return Box{}, ErrKeyUnavailable
	}
	return Box{key: append([]byte(nil), key...)}, nil
}

func (b Box) Seal(plaintext []byte, context string) ([]byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	result := append([]byte{version}, nonce...)
	return gcm.Seal(result, nonce, plaintext, []byte(context)), nil
}

func (b Box) Open(ciphertext []byte, context string) ([]byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < 1+gcm.NonceSize() || ciphertext[0] != version {
		return nil, fmt.Errorf("invalid encrypted value")
	}
	nonce := ciphertext[1 : 1+gcm.NonceSize()]
	return gcm.Open(nil, nonce, ciphertext[1+gcm.NonceSize():], []byte(context))
}
