package api

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"yoldosh/internal/auth"
	"yoldosh/internal/mail"
)

type fakeGoogle struct{ id auth.GoogleIdentity }

func (f fakeGoogle) Verify(_ context.Context, tok string) (auth.GoogleIdentity, error) {
	if tok != "valid-google-token" {
		return auth.GoogleIdentity{}, errors.New("bad token")
	}
	return f.id, nil
}

var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

func lastCode(t *testing.T, c *mail.Capture, to string) string {
	t.Helper()
	for i := len(c.Sent) - 1; i >= 0; i-- {
		if c.Sent[i].To == to {
			if m := codeRe.FindStringSubmatch(c.Sent[i].Text); m != nil {
				return m[1]
			}
		}
	}
	t.Fatalf("%s ga kod yuborilmadi", to)
	return ""
}

type session struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	IsNewUser    bool   `json:"isNewUser"`
	User         meDTO  `json:"user"`
}

func TestEmailAndGoogleAuth(t *testing.T) {
	box := &mail.Capture{}
	origMail, origGoogle := testAPI.mailer, testAPI.google
	defer func() { testAPI.mailer, testAPI.google = origMail, origGoogle }()
	testAPI.mailer = box
	testAPI.limiter = NewLimiter() // testlar bitta IP dan — IP limitini yangilaymiz
	var e struct{ Error apiError }

	// 1. Anonim turist marshrut tuzadi, keyin email bilan kiradi — marshrut hisobda qoladi.
	anon := newClient(t, "auth-device-0001")
	mkTrip(t, anon, map[string]any{"cityId": "bukhara", "startDate": "2026-12-02", "days": 1})
	var me meDTO
	anon.do("GET", "/api/v1/me", nil, &me)
	anonID := me.ID

	pub := &client{t: t, lang: "uz"}
	if code := pub.do("POST", "/api/v1/auth/email/start", map[string]string{"email": "noto'g'ri"}, &e); code != 400 {
		t.Fatalf("noto'g'ri email: %d", code)
	}
	if code := pub.do("POST", "/api/v1/auth/email/start", map[string]string{"email": "Turist@Example.com"}, nil); code != 202 {
		t.Fatalf("start: %d", code)
	}
	code := lastCode(t, box, "turist@example.com")

	if c := pub.do("POST", "/api/v1/auth/email/verify", map[string]string{"email": "turist@example.com", "code": "000000"}, &e); c != 400 || e.Error.Code != "code_invalid" {
		t.Fatalf("noto'g'ri kod: %d %+v", c, e)
	}
	var s1 session
	if c := anon.do("POST", "/api/v1/auth/email/verify", map[string]string{"email": "TURIST@example.com", "code": code, "deviceId": "auth-device-0001"}, &s1); c != 200 {
		t.Fatalf("verify: %d", c)
	}
	if s1.User.ID != anonID || !s1.User.IsRegistered || *s1.User.Email != "turist@example.com" || s1.RefreshToken == "" {
		t.Fatalf("anonim hisob ro'yxatdan o'tgan hisobga aylanishi kerak: %+v", s1)
	}
	if c := pub.do("POST", "/api/v1/auth/email/verify", map[string]string{"email": "turist@example.com", "code": code}, &e); c != 400 {
		t.Fatalf("kod ikki marta ishladi: %d", c)
	}
	acct := &client{t: t, lang: "uz", token: s1.Token}
	var trips []tripListDTO
	acct.do("GET", "/api/v1/trips", nil, &trips)
	if len(trips) != 1 {
		t.Fatalf("anonim marshrut yo'qoldi: %d", len(trips))
	}

	// 2. Faqat deviceId bilan endi hisobga kirib bo'lmaydi — yangi anonim sessiya beriladi.
	var dev struct{ UserID string }
	pub.do("POST", "/api/v1/auth/device", map[string]string{"deviceId": "auth-device-0001", "platform": "android"}, &dev)
	if dev.UserID == s1.User.ID {
		t.Fatal("deviceId orqali ro'yxatdan o'tgan hisobga kirildi")
	}

	// 3. Boshqa telefondagi anonim ma'lumot + Google (xuddi shu tasdiqlangan email) → bitta hisobga qo'shiladi.
	testAPI.google = fakeGoogle{auth.GoogleIdentity{Sub: "g-123", Email: "turist@example.com", EmailVerified: true, Name: "Turist"}}
	anon2 := newClient(t, "auth-device-0002")
	mkTrip(t, anon2, map[string]any{"cityId": "samarkand", "startDate": "2026-12-03", "days": 1})
	if c := anon2.do("POST", "/api/v1/auth/google", map[string]string{"idToken": "bad"}, &e); c != 401 {
		t.Fatalf("yomon google token: %d", c)
	}
	var s2 session
	if c := anon2.do("POST", "/api/v1/auth/google", map[string]string{"idToken": "valid-google-token", "deviceId": "auth-device-0002"}, &s2); c != 200 {
		t.Fatalf("google: %d", c)
	}
	if s2.User.ID != s1.User.ID || len(s2.User.AuthProviders) != 2 {
		t.Fatalf("google xuddi shu hisobga ulanishi kerak: %+v", s2.User)
	}
	acct.token = s2.Token
	acct.do("GET", "/api/v1/trips", nil, &trips)
	if len(trips) != 2 {
		t.Fatalf("ikki qurilmadagi marshrutlar birlashmadi: %d", len(trips))
	}

	// Tasdiqlanmagan Google email boshqa birovning hisobini egallay olmaydi.
	testAPI.google = fakeGoogle{auth.GoogleIdentity{Sub: "g-evil", Email: "turist@example.com", EmailVerified: false}}
	var s3 session
	pub.do("POST", "/api/v1/auth/google", map[string]string{"idToken": "valid-google-token"}, &s3)
	if s3.User.ID == s1.User.ID || s3.User.Email != nil {
		t.Fatal("tasdiqlanmagan email orqali begona hisobga kirildi")
	}

	// 4. Refresh: almashtiriladi; eski token qayta ishlatilsa — butun oila bekor qilinadi.
	var r1 session
	if c := pub.do("POST", "/api/v1/auth/refresh", map[string]string{"refreshToken": s2.RefreshToken}, &r1); c != 200 || r1.RefreshToken == s2.RefreshToken {
		t.Fatalf("refresh: %d", c)
	}
	if c := pub.do("POST", "/api/v1/auth/refresh", map[string]string{"refreshToken": s2.RefreshToken}, &e); c != 401 {
		t.Fatalf("eski refresh qayta ishladi: %d", c)
	}
	if c := pub.do("POST", "/api/v1/auth/refresh", map[string]string{"refreshToken": r1.RefreshToken}, &e); c != 401 {
		t.Fatalf("o'g'irlik aniqlangach yangi token ham bekor bo'lishi kerak edi: %d", c)
	}
	if c := pub.do("POST", "/api/v1/auth/refresh", map[string]string{"refreshToken": s1.RefreshToken}, &e); c != 401 {
		t.Fatalf("birinchi qurilma sessiyasi ham yopilishi kerak edi: %d", c)
	}

	// 5. Hisobni o'chirish.
	acct.token = r1.Token
	if c := acct.do("DELETE", "/api/v1/me", nil, nil); c != 204 {
		t.Fatalf("delete me: %d", c)
	}
	if c := acct.do("GET", "/api/v1/me", nil, &e); c != 401 {
		t.Fatalf("o'chirilgan hisob tokeni ishladi: %d", c)
	}
}

func TestEmailCodeLimits(t *testing.T) {
	box := &mail.Capture{}
	orig := testAPI.mailer
	defer func() { testAPI.mailer = orig }()
	testAPI.mailer = box
	testAPI.limiter = NewLimiter()
	pub := &client{t: t, lang: "en"}
	var e struct{ Error apiError }

	pub.do("POST", "/api/v1/auth/email/start", map[string]string{"email": "brute@example.com"}, nil)
	code := lastCode(t, box, "brute@example.com")
	wrong := "111111"
	if wrong == code {
		wrong = "222222"
	}
	for range 5 {
		pub.do("POST", "/api/v1/auth/email/verify", map[string]string{"email": "brute@example.com", "code": wrong}, &e)
	}
	if c := pub.do("POST", "/api/v1/auth/email/verify", map[string]string{"email": "brute@example.com", "code": code}, &e); c != 429 || e.Error.Code != "code_locked" {
		t.Fatalf("5 ta xatodan keyin to'g'ri kod ham bloklanishi kerak: %d %+v", c, e)
	}
	pub.do("POST", "/api/v1/auth/email/start", map[string]string{"email": "brute@example.com"}, nil)
	pub.do("POST", "/api/v1/auth/email/start", map[string]string{"email": "brute@example.com"}, nil)
	if c := pub.do("POST", "/api/v1/auth/email/start", map[string]string{"email": "brute@example.com"}, &e); c != 429 {
		t.Fatalf("10 daqiqada 3 tadan ko'p kod: %d", c)
	}

	testAPI.mailer = mail.SMTP{}
	if c := pub.do("POST", "/api/v1/auth/email/start", map[string]string{"email": "nomail@example.com"}, &e); c != 503 || e.Error.Code != "email_unavailable" {
		t.Fatalf("SMTP sozlanmagan: %d %+v", c, e)
	}
	origG := testAPI.google
	defer func() { testAPI.google = origG }()
	testAPI.google = auth.NewGoogleJWKS(nil)
	if c := pub.do("POST", "/api/v1/auth/google", map[string]string{"idToken": "x"}, &e); c != 503 || e.Error.Code != "google_unavailable" {
		t.Fatalf("google sozlanmagan: %d %+v", c, e)
	}
}
