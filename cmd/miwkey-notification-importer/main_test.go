package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestRunTokenPrintsSignedJWT(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runWithIO(context.Background(), []string{"token", "--secret", "secret"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWithIO() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}

	tokenString := strings.TrimSpace(stdout.String())
	if tokenString == "" {
		t.Fatal("token output is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			t.Fatalf("method = %v", token.Method)
		}
		return []byte("secret"), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(time.Now))
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
}

func TestRunTokenRequiresSecret(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runWithIO(context.Background(), []string{"token"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runWithIO() error = nil")
	}
	if !strings.Contains(err.Error(), "--secret is required") {
		t.Fatalf("error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
