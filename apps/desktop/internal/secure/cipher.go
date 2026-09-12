package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "advance-hris"
	keyringUser    = "desktop-local-data-key"
)

type Cipher struct {
	aead cipher.AEAD
}

func NewKeyringCipher() (*Cipher, error) {
	encoded, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		if !errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("read local encryption key: %w", err)
		}
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("generate local encryption key: %w", err)
		}
		encoded = base64.RawStdEncoding.EncodeToString(key)
		if err := keyring.Set(keyringService, keyringUser, encoded); err != nil {
			return nil, fmt.Errorf("store local encryption key: %w", err)
		}
	}

	key, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode local encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid local encryption key length: %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	nonceSize := c.aead.NonceSize()
	if len(payload) < nonceSize {
		return nil, errors.New("encrypted payload is too short")
	}
	return c.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
}
