package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mi-24v/miwkey-notification-importer/internal/importer"
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

func TestSplitCommaListTrimsEmptyValues(t *testing.T) {
	result := splitCommaList("pollVote, groupInvited,,reaction ")
	want := []string{"pollVote", "groupInvited", "reaction"}

	if strings.Join(result, ",") != strings.Join(want, ",") {
		t.Fatalf("splitCommaList() = %v, want %v", result, want)
	}
}

func TestPrintResultIncludesSkippedNotifications(t *testing.T) {
	var stderr bytes.Buffer

	printResult(&stderr, importer.Result{
		SuccessCount:     10,
		SkippedCount:     2,
		FailureCount:     1,
		LastSuccessfulID: "ok",
		LastSkippedID:    "skipped",
		FailedID:         "failed",
	})

	output := stderr.String()
	for _, want := range []string{
		"success=10",
		"skipped=2",
		"failure=1",
		`last_successful_id="ok"`,
		`last_skipped_id="skipped"`,
		`failed_id="failed"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("printResult() = %q, missing %q", output, want)
		}
	}
}
