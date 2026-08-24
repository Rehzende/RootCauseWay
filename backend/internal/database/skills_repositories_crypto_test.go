package database

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Rehzende/RootCauseway/backend/internal/crypto"
)

func newDBTestCipher(t *testing.T) crypto.Cipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	c, err := crypto.New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("crypto.New error: %v", err)
	}
	return c
}

func TestEncryptJSONFieldRoundTrip(t *testing.T) {
	c := newDBTestCipher(t)
	original := json.RawMessage(`{"api_key":"sk-super-secret","token":"abc"}`)

	enc, err := encryptJSONField(c, original)
	if err != nil {
		t.Fatalf("encryptJSONField error: %v", err)
	}
	// Stored value must be valid JSON (JSONB column) and must not contain the secret.
	var asString string
	if err := json.Unmarshal(enc, &asString); err != nil {
		t.Fatalf("encrypted field is not a JSON string: %v (%s)", err, enc)
	}
	if !crypto.IsEncrypted(asString) {
		t.Errorf("encrypted field missing enc: envelope: %q", asString)
	}
	if strings.Contains(string(enc), "sk-super-secret") {
		t.Error("encrypted field leaks plaintext secret")
	}

	dec, err := decryptJSONField(c, enc)
	if err != nil {
		t.Fatalf("decryptJSONField error: %v", err)
	}
	if string(dec) != string(original) {
		t.Errorf("round trip mismatch: got %s, want %s", dec, original)
	}
}

func TestEncryptJSONFieldEmpty(t *testing.T) {
	c := newDBTestCipher(t)
	enc, err := encryptJSONField(c, nil)
	if err != nil || enc != nil {
		t.Errorf("encryptJSONField(nil) = %s, %v; want nil, nil", enc, err)
	}
	dec, err := decryptJSONField(c, nil)
	if err != nil || dec != nil {
		t.Errorf("decryptJSONField(nil) = %s, %v; want nil, nil", dec, err)
	}
}

func TestDecryptJSONFieldLegacyPlaintextPassthrough(t *testing.T) {
	c := newDBTestCipher(t)
	for _, legacy := range []string{
		`{"vault_addr":"https://vault.example.com","role":"rootcauseway"}`,
		`[]`,
		`"just a plain json string"`,
		`{}`,
	} {
		dec, err := decryptJSONField(c, json.RawMessage(legacy))
		if err != nil {
			t.Fatalf("decryptJSONField(%s) error: %v", legacy, err)
		}
		if string(dec) != legacy {
			t.Errorf("legacy passthrough mismatch: got %s, want %s", dec, legacy)
		}
	}
}

func TestEncryptJSONFieldNoopKeepsValidJSON(t *testing.T) {
	noop, err := crypto.New("")
	if err != nil {
		t.Fatalf("crypto.New(\"\") error: %v", err)
	}
	original := json.RawMessage(`{"api_key":"plain"}`)
	enc, err := encryptJSONField(noop, original)
	if err != nil {
		t.Fatalf("encryptJSONField error: %v", err)
	}
	if string(enc) != string(original) {
		t.Errorf("noop mode altered JSON: got %s, want %s", enc, original)
	}
	dec, err := decryptJSONField(noop, enc)
	if err != nil || string(dec) != string(original) {
		t.Errorf("noop decrypt mismatch: got %s, %v", dec, err)
	}
}

func TestDecryptJSONFieldWrongKeyFails(t *testing.T) {
	c1 := newDBTestCipher(t)
	c2 := newDBTestCipher(t)
	enc, err := encryptJSONField(c1, json.RawMessage(`{"secret":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decryptJSONField(c2, enc); err == nil {
		t.Error("decryptJSONField with wrong key succeeded, want error")
	}
}

func TestPickCipherExplicitAndDefault(t *testing.T) {
	explicit := newDBTestCipher(t)
	if got := pickCipher([]crypto.Cipher{explicit}); got != explicit {
		t.Error("pickCipher did not return the explicitly provided cipher")
	}

	// Default path: no env key -> no-op cipher (plaintext passthrough).
	t.Setenv(crypto.EnvKeyName, "")
	c := pickCipher(nil)
	enc, err := c.Encrypt("plain")
	if err != nil || enc != "plain" {
		t.Errorf("default cipher without env key should be no-op, got %q, %v", enc, err)
	}

	// Default path: env key set -> encrypting cipher.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Setenv(crypto.EnvKeyName, base64.StdEncoding.EncodeToString(key))
	c = pickCipher(nil)
	enc, err = c.Encrypt("plain")
	if err != nil {
		t.Fatal(err)
	}
	if !crypto.IsEncrypted(enc) {
		t.Errorf("default cipher with env key should encrypt, got %q", enc)
	}
}
