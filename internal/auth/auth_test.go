package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenValidation(t *testing.T) {
	secret := strings.Repeat("s", 32)
	token, _, err := GenerateToken(1, "admin", secret)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateToken(token, secret)
	if err != nil || claims.UserID != 1 {
		t.Fatalf("valid token: %v", err)
	}
	if _, err := ValidateToken(token, "wrong secret"); err == nil {
		t.Fatal("wrong key accepted")
	}
	for _, tc := range []struct {
		name, issuer string
		expiry       *jwt.NumericDate
		method       jwt.SigningMethod
	}{
		{"no expiry", "opsup", nil, jwt.SigningMethodHS256},
		{"expired", "opsup", jwt.NewNumericDate(time.Now().Add(-time.Minute)), jwt.SigningMethodHS256},
		{"wrong issuer", "another-app", jwt.NewNumericDate(time.Now().Add(time.Hour)), jwt.SigningMethodHS256},
		{"wrong method", "opsup", jwt.NewNumericDate(time.Now().Add(time.Hour)), jwt.SigningMethodHS512},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tc.method, Claims{UserID: 1, RegisteredClaims: jwt.RegisteredClaims{Issuer: tc.issuer, ExpiresAt: tc.expiry}}).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateToken(token, secret); err == nil {
				t.Fatal("invalid claims accepted")
			}
		})
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("strong-password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "strong-password" || !CheckPassword("strong-password", hash) || CheckPassword("wrong", hash) {
		t.Fatal("invalid bcrypt behavior")
	}
}
