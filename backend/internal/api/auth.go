package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"yoldosh/internal/auth"
	mailer "yoldosh/internal/mail"
	"yoldosh/internal/store"
)

// Hisob: Google yoki email kod orqali kirish. Anonim qurilma sessiyasi (POST /auth/device) saqlanadi —
// kirishda Bearer da anonim token yuborilsa, anonim ma'lumotlar hisobga ko'chiriladi.

const (
	codeTTL        = 10 * time.Minute
	codesPerWindow = 3 // shu email ga 10 daqiqada
	refreshTTL     = 60 * 24 * time.Hour
)

type authResp struct {
	Token            string `json:"token"`
	ExpiresAt        string `json:"expiresAt"`
	RefreshToken     string `json:"refreshToken"`
	RefreshExpiresAt string `json:"refreshExpiresAt"`
	IsNewUser        bool   `json:"isNewUser"`
	User             meDTO  `json:"user"`
}

func normalizeEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || len(s) > 254 || !strings.Contains(s[strings.LastIndex(s, "@"):], ".") {
		return "", false
	}
	return s, true
}

func (s *Server) codeHash(email, code string) string {
	h := sha256.Sum256([]byte(string(s.cfg.TokenSecret) + "|" + email + "|" + code))
	return hex.EncodeToString(h[:])
}

func tokenHash(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n.Int64())
}

// anonFromBearer — so'rovda anonim turist tokeni bo'lsa uning id si (ma'lumotlarni hisobga ko'chirish uchun).
func (s *Server) anonFromBearer(r *http.Request) string {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	c, err := s.signer.Verify(tok)
	if err != nil || auth.IsAdminRole(c.Role) {
		return ""
	}
	return c.Subject
}

// issueSession — kirish tokeni (1 soat) + refresh token (60 kun).
func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, u store.User, deviceID string, isNew bool) {
	now := s.now()
	tok, exp, err := s.signer.IssueTTL(u.ID, u.Role, now, auth.AccountTokenTTL)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	refresh := randomToken()
	rexp := now.Add(refreshTTL)
	if err := s.store.IssueRefresh(r.Context(), u.ID, deviceID, tokenHash(refresh), rexp); err != nil {
		s.internalError(w, r, err)
		return
	}
	me, err := s.meFor(r, u)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authResp{Token: tok, ExpiresAt: exp.UTC().Format(time.RFC3339), RefreshToken: refresh,
		RefreshExpiresAt: rexp.UTC().Format(time.RFC3339), IsNewUser: isNew, User: me})
}

// ---------- Email kod ----------

func (s *Server) emailStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeValidation(w, r, fieldErrors{"email": "invalid email"})
		return
	}
	ctx := r.Context()
	n, err := s.store.CountRecentCodes(ctx, email, s.now().Add(-codeTTL))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if n >= codesPerWindow {
		w.Header().Set("Retry-After", "600")
		writeError(w, r, http.StatusTooManyRequests, "rate_limited", true)
		return
	}
	code := randomCode()
	if err := s.store.CreateAuthCode(ctx, email, s.codeHash(email, code), s.now().Add(codeTTL)); err != nil {
		s.internalError(w, r, err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.mailer.Send(sendCtx, codeEmail(langOf(r), email, code)); err != nil {
		if errors.Is(err, mailer.ErrNotConfigured) {
			writeError(w, r, http.StatusServiceUnavailable, "email_unavailable", false)
			return
		}
		slog.Error("kod xati yuborilmadi", "err", err)
		writeError(w, r, http.StatusServiceUnavailable, "email_unavailable", true)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"sent": true, "expiresInSec": int(codeTTL.Seconds())})
}

func codeEmail(lang, to, code string) mailer.Message {
	t := map[string][3]string{
		"uz": {"Yo'ldosh AI: kirish kodi %s", "Kirish kodingiz: %s\n\nKod 10 daqiqa amal qiladi. Agar kodni siz so'ramagan bo'lsangiz, bu xatni e'tiborsiz qoldiring.", "Kirish kodingiz"},
		"ru": {"Yo'ldosh AI: код входа %s", "Ваш код входа: %s\n\nКод действует 10 минут. Если вы его не запрашивали, просто проигнорируйте это письмо.", "Ваш код входа"},
		"en": {"Yo'ldosh AI: sign-in code %s", "Your sign-in code: %s\n\nThe code is valid for 10 minutes. If you didn't request it, you can ignore this email.", "Your sign-in code"},
	}
	m, ok := t[lang]
	if !ok {
		m = t["en"]
	}
	text := fmt.Sprintf(m[1], code)
	html := fmt.Sprintf(`<div style="font-family:system-ui,sans-serif;max-width:420px;margin:auto;padding:24px;color:#102a2e">
<p style="color:#0f8f7e;font-weight:700;font-size:18px">Yo'ldosh AI</p><p>%s:</p>
<p style="font-size:32px;font-weight:700;letter-spacing:6px;margin:16px 0">%s</p>
<p style="color:#55706c;font-size:14px">%s</p></div>`, m[2], code, strings.ReplaceAll(strings.SplitN(text, "\n\n", 2)[1], "\n", "<br>"))
	return mailer.Message{To: to, Subject: fmt.Sprintf(m[0], code), Text: text, HTML: html}
}

func (s *Server) emailVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		DeviceID string `json:"deviceId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)
	if !ok || len(code) != 6 {
		writeValidation(w, r, fieldErrors{"email/code": "email and 6-digit code required"})
		return
	}
	switch err := s.store.ConsumeAuthCode(r.Context(), email, s.codeHash(email, code), s.now()); {
	case errors.Is(err, store.ErrCodeInvalid):
		writeError(w, r, http.StatusBadRequest, "code_invalid", false)
		return
	case errors.Is(err, store.ErrCodeLocked):
		writeError(w, r, http.StatusTooManyRequests, "code_locked", false)
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	u, isNew, err := s.store.Login(r.Context(), store.Identity{Email: &email}, s.anonFromBearer(r), req.DeviceID, langOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.issueSession(w, r, u, req.DeviceID, isNew)
}

// ---------- Google ----------

func (s *Server) googleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDToken  string `json:"idToken"`
		DeviceID string `json:"deviceId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IDToken == "" || len(req.IDToken) > 8192 {
		writeValidation(w, r, fieldErrors{"idToken": "required"})
		return
	}
	g, err := s.google.Verify(r.Context(), req.IDToken)
	if errors.Is(err, auth.ErrGoogleNotConfigured) {
		writeError(w, r, http.StatusServiceUnavailable, "google_unavailable", false)
		return
	}
	if err != nil {
		slog.Info("google token rad etildi", "err", err)
		writeError(w, r, http.StatusUnauthorized, "google_token_invalid", false)
		return
	}
	id := store.Identity{GoogleSub: &g.Sub, Name: g.Name, AvatarURL: g.Picture}
	if g.EmailVerified && g.Email != "" {
		id.Email = &g.Email // faqat Google tasdiqlagan email (aks holda begona email egallab olinishi mumkin)
	}
	u, isNew, err := s.store.Login(r.Context(), id, s.anonFromBearer(r), req.DeviceID, langOf(r))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.issueSession(w, r, u, req.DeviceID, isNew)
}

// ---------- Refresh / logout / o'chirish ----------

func (s *Server) refreshSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	now := s.now()
	next := randomToken()
	rexp := now.Add(refreshTTL)
	userID, err := s.store.RotateRefresh(r.Context(), tokenHash(req.RefreshToken), tokenHash(next), now, rexp)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrTokenReuse) {
		if errors.Is(err, store.ErrTokenReuse) {
			slog.Warn("refresh token qayta ishlatildi — foydalanuvchining barcha sessiyalari yopildi", "user", userID)
		}
		writeError(w, r, http.StatusUnauthorized, "refresh_invalid", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	u, err := s.store.User(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "refresh_invalid", false)
		return
	}
	tok, exp, err := s.signer.IssueTTL(u.ID, u.Role, now, auth.AccountTokenTTL)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	me, err := s.meFor(r, u)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authResp{Token: tok, ExpiresAt: exp.UTC().Format(time.RFC3339), RefreshToken: next,
		RefreshExpiresAt: rexp.UTC().Format(time.RFC3339), User: me})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.RevokeRefresh(r.Context(), tokenHash(req.RefreshToken)); err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RevokeAllRefresh(r.Context(), userOf(r)); err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteMe — hisobni va unga tegishli barcha ma'lumotlarni o'chirish (App Store / Google Play talabi).
func (s *Server) deleteMe(w http.ResponseWriter, r *http.Request) {
	files, err := s.store.DeleteUser(r.Context(), userOf(r))
	if isNotFound(err) {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	for _, f := range files {
		if err := s.media.Delete(f); err != nil {
			slog.Warn("rasm o'chirilmadi", "file", f, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
