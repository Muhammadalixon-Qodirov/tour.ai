// Package api — HTTP qatlami: router, middleware, handlerlar. Biznes mantiq planner/domain da.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"yoldosh/internal/auth"
	"yoldosh/internal/diary"
	"yoldosh/internal/i18n"
	"yoldosh/internal/mail"
	"yoldosh/internal/media"
	"yoldosh/internal/platform"
	"yoldosh/internal/store"
	"yoldosh/internal/weather"
)

type Server struct {
	cfg     platform.Config
	store   *store.Store
	signer  *auth.Signer
	limiter *Limiter
	media   media.Store
	weather weather.Source
	story   diary.Writer
	mailer  mail.Sender
	google  auth.GoogleVerifier
	images  fs.FS // katalog rasmlari (/static/places/{file}); nil bo'lsa 404
	now     func() time.Time
}

// SetImages — katalog rasmlari manbasi (cmd/api binar ichidagi static.Places() ni beradi).
func (s *Server) SetImages(f fs.FS) { s.images = f }

func NewServer(cfg platform.Config, st *store.Store) *Server {
	s := &Server{cfg: cfg, store: st, limiter: NewLimiter(),
		media: media.Local{Dir: cfg.UploadDir, BaseURL: cfg.PublicBaseURL}, weather: weather.NewOpenMeteo(), story: diary.Template{}, now: time.Now}
	s.signer = auth.NewSigner(cfg.TokenSecret, func() time.Time { return s.now() })
	s.google = auth.NewGoogleJWKS(cfg.GoogleClientIDs)
	switch {
	case cfg.SMTPHost != "":
		s.mailer = mail.SMTP{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.MailFrom}
	case cfg.MailDevLog:
		s.mailer = mail.DevLog{}
	default:
		s.mailer = mail.SMTP{} // sozlanmagan: /auth/email/start 503 qaytaradi
	}
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	pub := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.rate("general", 120, h)) }
	user := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireUser(s.rate("general", 120, h)))
	}

	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /static/ph/{file}", s.placeholder)
	mux.HandleFunc("GET /static/places/{file}", s.placeImage)

	mux.Handle("POST /api/v1/auth/device", s.rateIP("auth", 30, http.HandlerFunc(s.authDevice)))
	mux.Handle("POST /api/v1/auth/email/start", s.rateIP("email-start", 5, http.HandlerFunc(s.emailStart)))
	mux.Handle("POST /api/v1/auth/email/verify", s.rateIP("email-verify", 10, http.HandlerFunc(s.emailVerify)))
	mux.Handle("POST /api/v1/auth/google", s.rateIP("auth-google", 20, http.HandlerFunc(s.googleLogin)))
	mux.Handle("POST /api/v1/auth/refresh", s.rateIP("auth-refresh", 30, http.HandlerFunc(s.refreshSession)))
	mux.Handle("POST /api/v1/auth/logout", s.rateIP("auth-refresh", 30, http.HandlerFunc(s.logout)))
	user("POST /api/v1/auth/logout-all", s.logoutAll)
	user("DELETE /api/v1/me", s.deleteMe)
	user("GET /api/v1/me", s.getMe)
	user("PATCH /api/v1/me", s.patchMe)

	pub("GET /api/v1/cities", s.listCities)
	pub("GET /api/v1/places", s.listPlaces)
	pub("GET /api/v1/places/{id}", s.getPlace)
	pub("GET /api/v1/places/{id}/reviews", s.placeReviews)
	pub("GET /api/v1/geofences", s.geofences)
	pub("GET /api/v1/prices", s.listPrices)
	mux.Handle("POST /api/v1/prices/reports", s.optionalUser(s.rate("price-report", 10, http.HandlerFunc(s.reportPrice))))
	pub("GET /api/v1/events", s.listEvents)
	pub("GET /api/v1/services", s.listServices)
	pub("GET /api/v1/stays", s.listStays)
	pub("GET /api/v1/stays/{id}", s.getStay)
	pub("GET /api/v1/cities/{id}/emergency", s.emergency)
	mux.Handle("GET /api/v1/cities/{id}/offline-pack", s.rateIP("offline", 30, http.HandlerFunc(s.offlinePack)))

	mux.Handle("POST /api/v1/trips", s.requireUser(s.rate("trips", 6, http.HandlerFunc(s.createTrip))))
	user("GET /api/v1/trips", s.listTrips)
	user("GET /api/v1/trips/{id}", s.getTrip)
	user("DELETE /api/v1/trips/{id}", s.deleteTrip)
	user("POST /api/v1/trips/{id}/stops", s.addStop)
	user("DELETE /api/v1/trips/{id}/stops/{stopId}", s.deleteStop)
	user("POST /api/v1/trips/{id}/stops/{stopId}/swap", s.swapStop)
	user("PATCH /api/v1/trips/{id}/stops/{stopId}", s.patchStop)
	user("POST /api/v1/trips/{id}/stops/{stopId}/move", s.moveStop)
	mux.Handle("POST /api/v1/trips/{id}/replan", s.requireUser(s.rate("replan", 20, http.HandlerFunc(s.replanTrip))))
	user("POST /api/v1/visits", s.addVisit)
	user("GET /api/v1/me/saved", s.listSaved)
	user("PUT /api/v1/me/saved/{placeId}", s.setSaved(true))
	user("DELETE /api/v1/me/saved/{placeId}", s.setSaved(false))
	user("POST /api/v1/groups", s.createGroup)
	user("GET /api/v1/groups", s.listGroups)
	mux.Handle("POST /api/v1/groups/join", s.requireUser(s.rate("group-join", 10, http.HandlerFunc(s.joinGroup))))
	user("GET /api/v1/groups/{id}", s.getGroup)
	user("PATCH /api/v1/groups/{id}/me", s.patchGroupMe)
	user("POST /api/v1/groups/{id}/votes", s.voteGroup)
	user("DELETE /api/v1/groups/{id}/members/me", s.leaveGroup)
	mux.Handle("POST /api/v1/groups/{id}/trip", s.requireUser(s.rate("trips", 6, http.HandlerFunc(s.groupTrip))))

	// sayohat kundaligi
	mux.Handle("POST /api/v1/diary/photos", s.requireUser(s.rate("photos", 60, http.HandlerFunc(s.uploadDiaryPhoto))))
	user("DELETE /api/v1/diary/photos/{id}", s.deleteDiaryPhoto)
	user("GET /api/v1/trips/{id}/days/{idx}/photos", s.dayPhotos)
	mux.Handle("POST /api/v1/trips/{id}/days/{idx}/diary", s.requireUser(s.rate("diary", 20, http.HandlerFunc(s.createDiary))))
	user("GET /api/v1/trips/{id}/days/{idx}/diary", s.getDayDiary)
	user("GET /api/v1/diary/{entryId}", s.getDiary)
	user("PATCH /api/v1/diary/{entryId}", s.patchDiary)
	mux.Handle("POST /api/v1/diary/{entryId}/regenerate", s.requireUser(s.rate("diary", 20, http.HandlerFunc(s.regenerateDiary))))
	user("POST /api/v1/diary/{entryId}/publish", s.publishDiary)
	user("POST /api/v1/diary/{entryId}/shares", s.recordShare)
	user("GET /api/v1/diary/{entryId}/share-card", s.shareCard)
	user("POST /api/v1/me/push-token", s.setPushToken)

	// lenta
	mux.Handle("GET /api/v1/feed", s.optionalUser(s.rate("general", 120, http.HandlerFunc(s.feed))))
	mux.Handle("GET /api/v1/posts/{id}", s.optionalUser(s.rate("general", 120, http.HandlerFunc(s.getPost))))
	user("POST /api/v1/posts/{id}/like", s.likePost(true))
	user("DELETE /api/v1/posts/{id}/like", s.likePost(false))
	mux.Handle("POST /api/v1/posts/{id}/report", s.requireUser(s.rate("report", 10, http.HandlerFunc(s.reportPost))))
	user("DELETE /api/v1/posts/{id}", s.deletePost)
	mux.Handle("GET /s/{slug}", s.rateIP("public", 120, http.HandlerFunc(s.publicPost)))
	mux.Handle("GET /s/{slug}/card.png", s.rateIP("public-card", 60, http.HandlerFunc(s.publicCard)))

	mux.HandleFunc("GET /uploads/{file}", s.serveUpload)
	adm := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireAdmin(s.rate("admin", 300, h))) }
	mux.Handle("POST /api/v1/admin/auth/login", s.rateIP("admin-login", 10, http.HandlerFunc(s.adminLogin)))
	adm("GET /api/v1/admin/me", s.adminMe)
	adm("GET /api/v1/admin/places", s.adminListPlaces)
	adm("POST /api/v1/admin/places", s.adminCreatePlace)
	adm("GET /api/v1/admin/places/{id}", s.adminGetPlace)
	adm("PATCH /api/v1/admin/places/{id}", s.adminPatchPlace)
	adm("POST /api/v1/admin/places/{id}/submit", s.changeStatus("pending", "draft", "rejected"))
	adm("POST /api/v1/admin/places/{id}/publish", s.changeStatus("published", "draft", "pending", "rejected"))
	adm("POST /api/v1/admin/places/{id}/reject", s.changeStatus("rejected", "draft", "pending"))
	adm("POST /api/v1/admin/places/{id}/unpublish", s.changeStatus("draft", "published"))
	adm("PUT /api/v1/admin/places/{id}/rules", s.adminPutRules)
	mux.Handle("POST /api/v1/admin/places/{id}/images", s.requireAdmin(s.rate("admin-upload", 30, http.HandlerFunc(s.adminUploadImage))))
	adm("GET /api/v1/admin/analytics/novelty", s.adminNovelty)
	adm("GET /api/v1/admin/posts", s.adminPosts)
	adm("GET /api/v1/admin/stays", s.adminListStays)
	adm("PATCH /api/v1/admin/stays/{id}", s.adminPatchStay)
	adm("POST /api/v1/admin/services", s.adminCreateService)
	adm("PATCH /api/v1/admin/services/{id}", s.adminPatchService)
	adm("DELETE /api/v1/admin/services/{id}", s.adminDeleteService)
	adm("POST /api/v1/admin/posts/{id}/hide", s.adminPostStatus("hidden"))
	adm("POST /api/v1/admin/posts/{id}/restore", s.adminPostStatus("visible"))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found", false)
	})

	return s.recoverer(s.logRequests(s.cors(s.withLang(mux))))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.DB.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- kontekst ----------

type ctxKey int

const (
	keyLang ctxKey = iota
	keyUser
	keyRole
	keyReqID
)

func langOf(r *http.Request) string {
	if l, ok := r.Context().Value(keyLang).(string); ok {
		return l
	}
	return "en"
}

func userOf(r *http.Request) string {
	u, _ := r.Context().Value(keyUser).(string)
	return u
}

// ---------- middleware ----------

func (s *Server) withLang(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := i18n.FromHeader(r.Header.Get("Accept-Language"))
		w.Header().Add("Vary", "Accept-Language")
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyLang, lang)))
	})
}

func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		tok, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || tok == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		c, err := s.signer.Verify(tok)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		if ok, err := s.store.UserExists(r.Context(), c.Subject); err != nil || !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", false)
			return
		}
		ctx := context.WithValue(r.Context(), keyUser, c.Subject)
		ctx = context.WithValue(ctx, keyRole, c.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) rate(bucket string, perMin int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := userOf(r)
		if key == "" {
			key = s.clientIP(r)
		}
		if wait, ok := s.limiter.Allow(bucket+":"+key, perMin); !ok {
			w.Header().Set("Retry-After", wait)
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", true)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateIP(bucket string, perMin int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wait, ok := s.limiter.Allow(bucket+":"+s.clientIP(r), perMin); !ok {
			w.Header().Set("Retry-After", wait)
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", true)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP — limit kaliti. So'rov ishonchli proksidan (TRUSTED_PROXIES) kelsa X-Forwarded-For o'ngdan chapga
// o'qiladi va birinchi ishonchsiz manzil olinadi: mijoz o'zi yozgan chap qismi bilan limitni aldab bo'lmaydi.
// Aks holda RemoteAddr.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !s.trusted(host) {
		return host
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		h := strings.TrimSpace(hops[i])
		if h == "" {
			continue
		}
		if _, err := netip.ParseAddr(h); err != nil {
			break // buzilgan sarlavha: oxirgi ishonchli hop qoladi
		}
		if !s.trusted(h) {
			return h
		}
		host = h
	}
	return host
}

func (s *Server) trusted(ip string) bool {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, p := range s.cfg.TrustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && slices.Contains(s.cfg.CORSOrigins, o) {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept-Language, If-None-Match")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.Header().Set("Access-Control-Expose-Headers", "ETag, Retry-After, X-Request-Id")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }

// Unwrap — http.ResponseController (Flush, SSE) ichki writer ga yetib borishi uchun.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), keyReqID, id)))
		// Koordinatalar loglarga yozilmaydi: faqat yo'l, so'rov parametrlarisiz.
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"ms", time.Since(start).Milliseconds(), "req", id)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.Error("panic", "err", v, "stack", string(debug.Stack()))
				writeError(w, r, http.StatusInternalServerError, "internal", true)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---------- javob yordamchilari ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Retryable bool              `json:"retryable"`
	Fields    map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, retryable bool) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: i18n.T(langOf(r), code), Retryable: retryable}})
}

// fieldErrors — validatsiya xatolari (maydon → sabab, ingliz tilida, dasturchi uchun).
type fieldErrors map[string]string

func (f fieldErrors) add(field, msg string) { f[field] = msg }

func writeValidation(w http.ResponseWriter, r *http.Request, f fieldErrors) {
	writeJSON(w, http.StatusBadRequest, map[string]apiError{"error": {
		Code: "validation_failed", Message: i18n.T(langOf(r), "validation_failed"), Fields: f}})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	id, _ := r.Context().Value(keyReqID).(string)
	slog.Error("internal", "err", err, "path", r.URL.Path, "req", id)
	writeError(w, r, http.StatusInternalServerError, "internal", true)
}

// decodeJSON — tana hajmi cheklangan, noma'lum maydonlar rad etiladi.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeValidation(w, r, fieldErrors{"body": err.Error()})
		return false
	}
	return true
}

func isNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
