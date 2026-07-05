package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mi-24v/miwkey-notification-importer/internal/auth"
)

func TestStaticBearerReturnsToken(t *testing.T) {
	source := auth.StaticBearer("abc")

	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token != "abc" {
		t.Fatalf("token = %q", token)
	}
}

func TestJWTSourceSignsRegisteredClaims(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	source := auth.JWTSource{
		Secret: "secret",
		Now: func() time.Time {
			return now
		},
	}

	tokenString, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tokenString == "" {
		t.Fatal("token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			t.Fatalf("method = %v", token.Method)
		}
		return []byte("secret"), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(func() time.Time {
		return now
	}))
	if err != nil {
		t.Fatalf("ParseWithClaims() error = %v", err)
	}
	if !token.Valid {
		t.Fatal("token is invalid")
	}

	claims := token.Claims.(*jwt.RegisteredClaims)
	if claims.Issuer != "miwkey-notification-importer" {
		t.Fatalf("Issuer = %q", claims.Issuer)
	}
	if claims.Subject != "notification-extension" {
		t.Fatalf("Subject = %q", claims.Subject)
	}
	if claims.IssuedAt == nil || !claims.IssuedAt.Time.Equal(now) {
		t.Fatalf("IssuedAt = %v", claims.IssuedAt)
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.Time.Equal(now.Add(time.Minute)) {
		t.Fatalf("ExpiresAt = %v", claims.ExpiresAt)
	}
}
