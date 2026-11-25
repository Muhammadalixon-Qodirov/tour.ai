// Package auth — imzolangan sessiya tokenlari (JWT HS256).
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	Issuer   = "yoldosh"
	TokenTTL = 30 * 24 * time.Hour
)

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type Signer struct{ secret []byte }

func NewSigner(secret []byte) *Signer { return &Signer{secret: secret} }

// AdminTokenTTL — admin sessiyasi qisqaroq.
const AdminTokenTTL = 12 * time.Hour

func IsAdminRole(role string) bool { return role == "moderator" || role == "admin" }

func (s *Signer) Issue(userID, role string, now time.Time) (string, time.Time, error) {
	ttl := TokenTTL
	if IsAdminRole(role) {
		ttl = AdminTokenTTL
	}
	exp := now.Add(ttl)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID, Issuer: Issuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	signed, err := tok.SignedString(s.secret)
	return signed, exp, err
}

func (s *Signer) Verify(token string) (*Claims, error) {
	c := &Claims{}
	_, err := jwt.ParseWithClaims(token, c, func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(Issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if c.Subject == "" {
		return nil, errors.New("subject yo'q")
	}
	return c, nil
}
