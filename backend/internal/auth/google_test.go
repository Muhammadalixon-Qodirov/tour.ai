package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGoogleJWKS(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		w.Header().Set("Cache-Control", "public, max-age=3600")
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	defer srv.Close()

	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	g := NewGoogleJWKS([]string{"web-client.apps.googleusercontent.com"})
	g.CertsURL, g.Now = srv.URL, func() time.Time { return now }

	sign := func(claims jwt.MapClaims, k *rsa.PrivateKey, kid string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = kid
		s, _ := tok.SignedString(k)
		return s
	}
	good := jwt.MapClaims{"iss": "https://accounts.google.com", "aud": "web-client.apps.googleusercontent.com",
		"sub": "1234567890", "email": "Turist@Gmail.com", "email_verified": true, "name": "Turist",
		"exp": now.Add(time.Hour).Unix(), "iat": now.Unix()}

	id, err := g.Verify(context.Background(), sign(good, key, "k1"))
	if err != nil || id.Sub != "1234567890" || id.Email != "turist@gmail.com" || !id.EmailVerified {
		t.Fatalf("yaxshi token: %+v %v", id, err)
	}
	if _, err := g.Verify(context.Background(), sign(good, key, "k1")); err != nil || fetches != 1 {
		t.Fatalf("kalitlar keshlanmadi: %d", fetches)
	}

	bad := func(name string, mut func(jwt.MapClaims)) {
		c := jwt.MapClaims{}
		for k, v := range good {
			c[k] = v
		}
		mut(c)
		if _, err := g.Verify(context.Background(), sign(c, key, "k1")); err == nil {
			t.Errorf("%s: rad etilishi kerak edi", name)
		}
	}
	bad("begona aud", func(c jwt.MapClaims) { c["aud"] = "boshqa-ilova" })
	bad("begona iss", func(c jwt.MapClaims) { c["iss"] = "https://evil.example" })
	bad("muddati o'tgan", func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Hour).Unix() })

	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := g.Verify(context.Background(), sign(good, other, "k1")); err == nil {
		t.Error("begona kalit bilan imzolangan token o'tib ketdi")
	}
	if _, err := NewGoogleJWKS(nil).Verify(context.Background(), "x"); err != ErrGoogleNotConfigured {
		t.Error("sozlanmagan holat")
	}
}
