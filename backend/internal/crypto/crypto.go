// Package crypto provides AES-256-GCM envelope encryption for secrets stored
// at rest (credential provider configs, credential paths, etc.).
//
// Wire format (versioned for future key rotation):
//
//	enc:v1:<base64(nonce)>:<base64(ciphertext)>
//
// Values that do not start with the "enc:" prefix are treated as legacy
// plaintext and passed through Decrypt unchanged, so pre-existing rows keep
// working; they are encrypted the next time they are written.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

// EnvKeyName is the environment variable holding the base64-encoded 32-byte key.
// Generate one with: openssl rand -base64 32
const EnvKeyName = "ROOTCAUSEWAY_ENCRYPTION_KEY"

const (
	envelopePrefix = "enc:"
	v1Prefix       = "enc:v1:"
)

// Cipher encrypts and decrypts string values for storage at rest.
type Cipher interface {
	// Encrypt returns the value in the versioned envelope format.
	Encrypt(plaintext string) (string, error)
	// Decrypt reverses Encrypt. Values without the "enc:" prefix are
	// returned unchanged (legacy plaintext passthrough).
	Decrypt(value string) (string, error)
}

// New returns an AES-256-GCM Cipher for the given base64-encoded 32-byte key.
// If base64Key is empty, encryption is disabled: a no-op Cipher is returned
// and a warning is logged.
func New(base64Key string) (Cipher, error) {
	if base64Key == "" {
		log.Printf("WARNING: %s is not set — credential encryption at rest is DISABLED; secrets will be stored in plaintext. Generate a key with: openssl rand -base64 32", EnvKeyName)
		return noopCipher{}, nil
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64-encoded: %w", EnvKeyName, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%s must decode to 32 bytes for AES-256, got %d bytes (generate with: openssl rand -base64 32)", EnvKeyName, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to init AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to init GCM: %w", err)
	}
	return &aesGCMCipher{aead: aead}, nil
}

// NewFromEnv builds a Cipher from the ROOTCAUSEWAY_ENCRYPTION_KEY environment variable.
func NewFromEnv() (Cipher, error) {
	return New(os.Getenv(EnvKeyName))
}

// IsEncrypted reports whether a stored value carries the encryption envelope prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, envelopePrefix)
}

// --- AES-256-GCM cipher ---

type aesGCMCipher struct{ aead cipher.AEAD }

func (c *aesGCMCipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return v1Prefix +
		base64.StdEncoding.EncodeToString(nonce) + ":" +
		base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *aesGCMCipher) Decrypt(value string) (string, error) {
	if !IsEncrypted(value) {
		// Legacy plaintext passthrough.
		return value, nil
	}
	if !strings.HasPrefix(value, v1Prefix) {
		return "", fmt.Errorf("unsupported encryption envelope version (expected %q prefix)", v1Prefix)
	}
	rest := strings.TrimPrefix(value, v1Prefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("malformed encryption envelope: expected enc:v1:<nonce>:<ciphertext>")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("malformed encryption envelope nonce: %w", err)
	}
	if len(nonce) != c.aead.NonceSize() {
		return "", fmt.Errorf("malformed encryption envelope: nonce must be %d bytes, got %d", c.aead.NonceSize(), len(nonce))
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("malformed encryption envelope ciphertext: %w", err)
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong key or tampered ciphertext): %w", err)
	}
	return string(plaintext), nil
}

// --- No-op cipher (encryption disabled) ---

type noopCipher struct{}

func (noopCipher) Encrypt(plaintext string) (string, error) { return plaintext, nil }

func (noopCipher) Decrypt(value string) (string, error) {
	if IsEncrypted(value) {
		return "", fmt.Errorf("value is encrypted but %s is not configured", EnvKeyName)
	}
	return value, nil
}
