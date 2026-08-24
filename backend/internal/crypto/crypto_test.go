package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func newTestCipher(t *testing.T) Cipher {
	t.Helper()
	c, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	return c
}

func TestRoundTrip(t *testing.T) {
	c := newTestCipher(t)
	cases := []string{
		"secret-token-abc123",
		`{"api_key":"sk-super-secret","region":"us-east-1"}`,
		"path/with/slashes:and:colons",
		"unicode: héllo wörld 日本語 🔐",
		strings.Repeat("x", 4096),
	}
	for _, plaintext := range cases {
		enc, err := c.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt(%q) error: %v", plaintext, err)
		}
		if !strings.HasPrefix(enc, "enc:v1:") {
			t.Errorf("Encrypt(%q) = %q, want enc:v1: prefix", plaintext, enc)
		}
		if strings.Contains(enc, plaintext) && plaintext != "" {
			t.Errorf("ciphertext contains plaintext for %q", plaintext)
		}
		dec, err := c.Decrypt(enc)
		if err != nil {
			t.Fatalf("Decrypt error: %v", err)
		}
		if dec != plaintext {
			t.Errorf("round trip mismatch: got %q, want %q", dec, plaintext)
		}
	}
}

func TestEncryptEmptyStringIsEmpty(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt(\"\") error: %v", err)
	}
	if enc != "" {
		t.Errorf("Encrypt(\"\") = %q, want \"\"", enc)
	}
	dec, err := c.Decrypt("")
	if err != nil || dec != "" {
		t.Errorf("Decrypt(\"\") = %q, %v; want \"\", nil", dec, err)
	}
}

func TestNonceUniqueness(t *testing.T) {
	c := newTestCipher(t)
	a, _ := c.Encrypt("same plaintext")
	b, _ := c.Encrypt("same plaintext")
	if a == b {
		t.Error("two encryptions of the same plaintext produced identical output (nonce reuse)")
	}
}

func TestWireFormat(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt("hello")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(enc, ":")
	if len(parts) != 4 {
		t.Fatalf("expected 4 colon-separated segments (enc:v1:nonce:ct), got %d in %q", len(parts), enc)
	}
	if parts[0] != "enc" || parts[1] != "v1" {
		t.Errorf("wrong envelope header: %s:%s", parts[0], parts[1])
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		t.Errorf("nonce is not valid base64: %v", err)
	}
	if len(nonce) != 12 {
		t.Errorf("nonce length = %d, want 12 (GCM standard)", len(nonce))
	}
	if _, err := base64.StdEncoding.DecodeString(parts[3]); err != nil {
		t.Errorf("ciphertext is not valid base64: %v", err)
	}
}

func TestLegacyPlaintextPassthrough(t *testing.T) {
	c := newTestCipher(t)
	for _, legacy := range []string{
		"plain-old-secret",
		`{"vault_addr":"https://vault.example.com"}`,
		"encoded-but-not-enveloped",
	} {
		got, err := c.Decrypt(legacy)
		if err != nil {
			t.Fatalf("Decrypt(%q) error: %v", legacy, err)
		}
		if got != legacy {
			t.Errorf("Decrypt(%q) = %q, want passthrough", legacy, got)
		}
	}
}

func TestWrongKeyFails(t *testing.T) {
	c1 := newTestCipher(t)
	c2 := newTestCipher(t)
	enc, err := c1.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c2.Decrypt(enc); err == nil {
		t.Error("Decrypt with a different key succeeded, want error")
	}
}

func TestTamperedCiphertextFails(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(enc, ":")
	ct, _ := base64.StdEncoding.DecodeString(parts[3])
	ct[0] ^= 0xff
	tampered := parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + base64.StdEncoding.EncodeToString(ct)
	if _, err := c.Decrypt(tampered); err == nil {
		t.Error("Decrypt of tampered ciphertext succeeded, want error")
	}
}

func TestMalformedEnvelopeFails(t *testing.T) {
	c := newTestCipher(t)
	for _, bad := range []string{
		"enc:v1:only-one-segment",
		"enc:v1:!!!notbase64!!!:AAAA",
		"enc:v1:AAAA:!!!notbase64!!!",
		"enc:v2:AAAA:AAAA", // unknown version must not pass through
		"enc:garbage",
	} {
		if _, err := c.Decrypt(bad); err == nil {
			t.Errorf("Decrypt(%q) succeeded, want error", bad)
		}
	}
}

func TestNoopMode(t *testing.T) {
	c, err := New("")
	if err != nil {
		t.Fatalf("New(\"\") error: %v", err)
	}
	enc, err := c.Encrypt("my-secret")
	if err != nil {
		t.Fatal(err)
	}
	if enc != "my-secret" {
		t.Errorf("noop Encrypt = %q, want plaintext passthrough", enc)
	}
	dec, err := c.Decrypt("my-secret")
	if err != nil || dec != "my-secret" {
		t.Errorf("noop Decrypt = %q, %v; want passthrough", dec, err)
	}
	// An encrypted value with no key configured must error, not leak garbage.
	if _, err := c.Decrypt("enc:v1:AAAA:AAAA"); err == nil {
		t.Error("noop Decrypt of encrypted value succeeded, want error")
	}
}

func TestInvalidKeys(t *testing.T) {
	if _, err := New("not-base64!!!"); err == nil {
		t.Error("New with invalid base64 succeeded, want error")
	}
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := New(short); err == nil {
		t.Error("New with short key succeeded, want error")
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv(EnvKeyName, testKey(t))
	c, err := NewFromEnv()
	if err != nil {
		t.Fatalf("NewFromEnv error: %v", err)
	}
	enc, err := c.Encrypt("x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "enc:v1:") {
		t.Errorf("NewFromEnv did not return an encrypting cipher: %q", enc)
	}
}

func TestIsEncrypted(t *testing.T) {
	if !IsEncrypted("enc:v1:a:b") {
		t.Error("IsEncrypted(enc:v1:...) = false, want true")
	}
	if IsEncrypted("plaintext") {
		t.Error("IsEncrypted(plaintext) = true, want false")
	}
}
