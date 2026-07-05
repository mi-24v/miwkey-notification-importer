package auth

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

type StaticBearer string

func (s StaticBearer) Token(_ context.Context) (string, error) {
	return string(s), nil
}

type JWTSource struct {
	Secret string
	Now    func() time.Time
}

func (s JWTSource) Token(_ context.Context) (string, error) {
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "miwkey-notification-importer",
		Subject:   "notification-extension",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	})

	return token.SignedString([]byte(s.Secret))
}
