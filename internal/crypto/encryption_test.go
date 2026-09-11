package crypto

import (
	"strings"
	"testing"
)

func TestEncryptionRoundTripAndAuthentication(t *testing.T) {
	key := strings.Repeat("ab", 32)
	ciphertext, err := Encrypt([]byte("private credential"), key)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := Decrypt(ciphertext, key)
	if err != nil || string(plaintext) != "private credential" {
		t.Fatalf("round trip: %q %v", plaintext, err)
	}
	second, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	if second == ciphertext {
		t.Fatal("nonce reused")
	}
	if _, err := Decrypt(ciphertext, strings.Repeat("cd", 32)); err == nil {
		t.Fatal("wrong key accepted")
	}
	tampered := "00" + ciphertext[2:]
	if tampered == ciphertext {
		tampered = "01" + ciphertext[2:]
	}
	for _, value := range []string{tampered, "bad hex", "00", ""} {
		if _, err := Decrypt(value, key); err == nil {
			t.Fatalf("invalid ciphertext accepted: %q", value)
		}
	}
}
