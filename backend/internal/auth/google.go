package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GoogleIdentity — tekshirilgan Google ID token mazmuni.
type GoogleIdentity struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (GoogleIdentity, error)
}

var ErrGoogleNotConfigured = errors.New("google client id lar sozlanmagan")

// GoogleJWKS — Google ID tokenini Google ning ochiq kalitlari bilan tekshiradi (client secret kerak emas).
// Kalitlar Cache-Control max-age bo'yicha keshlanadi.
type GoogleJWKS struct {
	ClientIDs []string // ruxsat etilgan aud: web, android, ios client id lar
	CertsURL  string
	HTTP      *http.Client
	Now       func() time.Time

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

func NewGoogleJWKS(clientIDs []string) *GoogleJWKS {
	return &GoogleJWKS{ClientIDs: clientIDs, CertsURL: "https://www.googleapis.com/oauth2/v3/certs",
		HTTP: &http.Client{Timeout: 5 * time.Second}, Now: time.Now}
}

type googleClaims struct {
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // Google ba'zan "true" satr sifatida beradi
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	jwt.RegisteredClaims
}

func (g *GoogleJWKS) Verify(ctx context.Context, idToken string) (GoogleIdentity, error) {
	if len(g.ClientIDs) == 0 {
		return GoogleIdentity{}, ErrGoogleNotConfigured
	}
	c := &googleClaims{}
	_, err := jwt.ParseWithClaims(idToken, c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return g.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithExpirationRequired(), jwt.WithTimeFunc(g.Now),
		jwt.WithLeeway(30*time.Second))
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("google token: %w", err)
	}
	if c.Issuer != "accounts.google.com" && c.Issuer != "https://accounts.google.com" {
		return GoogleIdentity{}, errors.New("google token: noto'g'ri iss")
	}
	if !slices.ContainsFunc(c.Audience, func(a string) bool { return slices.Contains(g.ClientIDs, a) }) {
		return GoogleIdentity{}, errors.New("google token: aud bizning client id emas")
	}
	if c.Subject == "" {
		return GoogleIdentity{}, errors.New("google token: sub yo'q")
	}
	verified := false
	switch v := c.EmailVerified.(type) {
	case bool:
		verified = v
	case string:
		verified = v == "true"
	}
	return GoogleIdentity{Sub: c.Subject, Email: strings.ToLower(strings.TrimSpace(c.Email)), EmailVerified: verified,
		Name: c.Name, Picture: c.Picture}, nil
}

func (g *GoogleJWKS) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if k, ok := g.keys[kid]; ok && g.Now().Before(g.expires) {
		return k, nil
	}
	// noma'lum kid yoki kesh eskirgan — qayta yuklash (Google kalitlarni almashtirib turadi)
	if err := g.refresh(ctx); err != nil {
		return nil, err
	}
	if k, ok := g.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("google kaliti topilmadi: %q", kid)
}

func (g *GoogleJWKS) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.CertsURL, nil)
	if err != nil {
		return err
	}
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google certs: status %d", resp.StatusCode)
	}
	var body struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range body.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return errors.New("google certs: kalit yo'q")
	}
	ttl := time.Hour
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		for _, part := range strings.Split(cc, ",") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(part), "max-age="); ok {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					ttl = time.Duration(n) * time.Second
				}
			}
		}
	}
	g.keys, g.expires = keys, g.Now().Add(ttl)
	return nil
}
