package api

import (
	"context"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"yoldosh/internal/i18n"
	"yoldosh/internal/store"
)

// optionalUser — token bo'lsa foydalanuvchini aniqlaydi, bo'lmasa anonim davom etadi (lenta ochiq).
func (s *Server) optionalUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && tok != "" {
			if c, err := s.signer.Verify(tok); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), keyUser, c.Subject))
			}
		}
		next.ServeHTTP(w, r)
	})
}

type postDTO struct {
	ID         string            `json:"id"`
	Slug       string            `json:"slug"`
	URL        string            `json:"url"`
	CardURL    string            `json:"cardUrl"`
	Author     map[string]string `json:"author"`
	CityID     string            `json:"cityId"`
	Date       string            `json:"date"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Highlights []string          `json:"highlights"`
	Hashtags   []string          `json:"hashtags"`
	Photos     []photoDTO        `json:"photos"`
	Places     []placeSummaryDTO `json:"places"` // bosilsa — joy sahifasi → "marshrutimga qo'shish"
	LikesCount int               `json:"likesCount"`
	LikedByMe  bool              `json:"likedByMe"`
	ViewsCount int               `json:"viewsCount"`
	IsMine     bool              `json:"isMine"`
	Visibility string            `json:"visibility"`
	CreatedAt  string            `json:"createdAt"`
}

func authorName(p store.Post, lang string) string {
	if p.AuthorName != nil && *p.AuthorName != "" {
		return *p.AuthorName
	}
	return map[string]string{"uz": "Sayohatchi", "ru": "Путешественник", "en": "Traveller"}[lang]
}

func (s *Server) postDTO(r *http.Request, p store.Post) (postDTO, error) {
	e := p.Entry
	places, err := s.placesFor(r, e.PlaceIDs)
	if err != nil {
		return postDTO{}, err
	}
	photos, err := s.photosFor(r, e.PhotoIDs)
	if err != nil {
		return postDTO{}, err
	}
	return postDTO{ID: p.ID.String(), Slug: p.Slug, URL: s.postURL(p.Slug), CardURL: s.postURL(p.Slug) + "/card.png",
		Author: map[string]string{"name": authorName(p, langOf(r))}, CityID: p.CityID, Date: e.Date.Format(time.DateOnly),
		Title: e.Title, Body: e.Body, Highlights: nonNil(e.Highlights), Hashtags: nonNil(e.Hashtags), Photos: photos,
		Places: places, LikesCount: p.LikesCount, LikedByMe: p.LikedByMe, ViewsCount: p.ViewsCount,
		IsMine: p.UserID == userOf(r), Visibility: p.Visibility, CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339)}, nil
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 20
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			writeValidation(w, r, fieldErrors{"limit": "1..50"})
			return
		}
		limit = n
	}
	off, ok := decodeCursor(q.Get("cursor"))
	if !ok {
		writeValidation(w, r, fieldErrors{"cursor": "invalid"})
		return
	}
	posts, err := s.store.Feed(r.Context(), userOf(r), q.Get("city"), limit, off)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	items := make([]postDTO, 0, len(posts))
	for _, p := range posts {
		d, err := s.postDTO(r, p)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		items = append(items, d)
	}
	var next *string
	if len(posts) == limit {
		next = encodeCursor(off + limit)
	}
	writeJSON(w, http.StatusOK, page[postDTO]{Items: items, NextCursor: next})
}

func (s *Server) loadPost(w http.ResponseWriter, r *http.Request) (store.Post, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "post_not_found", false)
		return store.Post{}, false
	}
	p, err := s.store.Post(r.Context(), userOf(r), id)
	if isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "post_not_found", false)
		return p, false
	}
	if err != nil {
		s.internalError(w, r, err)
		return p, false
	}
	return p, true
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadPost(w, r)
	if !ok {
		return
	}
	d, err := s.postDTO(r, p)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) likePost(liked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.loadPost(w, r)
		if !ok {
			return
		}
		n, err := s.store.SetLike(r.Context(), p.ID, userOf(r), liked)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"likesCount": n, "likedByMe": liked})
	}
}

var reportReasons = []string{"spam", "offensive", "false_info", "privacy", "other"}

func (s *Server) reportPost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.loadPost(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !slices.Contains(reportReasons, req.Reason) {
		writeValidation(w, r, fieldErrors{"reason": strings.Join(reportReasons, " | ")})
		return
	}
	if p.UserID == userOf(r) {
		writeValidation(w, r, fieldErrors{"post": "can't report your own post"})
		return
	}
	if _, err := s.store.ReportPost(r.Context(), p.ID, userOf(r), req.Reason); err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "post_not_found", false)
		return
	}
	if err := s.store.DeletePost(r.Context(), id, userOf(r)); isNotFound(err) {
		writeError(w, r, http.StatusNotFound, "post_not_found", false)
		return
	} else if err != nil {
		s.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Ochiq sahifa /s/{slug} ----------

var publicPage = template.Must(template.New("post").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} — Yo'ldosh AI</title>
<meta name="description" content="{{.Desc}}">
<meta property="og:type" content="article">
<meta property="og:site_name" content="Yo'ldosh AI">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Desc}}">
<meta property="og:image" content="{{.Card}}">
<meta property="og:image:width" content="1080"><meta property="og:image:height" content="1350">
<meta property="og:url" content="{{.URL}}">
<meta name="twitter:card" content="summary_large_image">
<style>
:root{--bg:#f4f6f5;--fg:#102a2e;--muted:#55706c;--accent:#0f8f7e;--card:#fff}
@media (prefers-color-scheme:dark){:root{--bg:#0d1d20;--fg:#e8efed;--muted:#9fb5b1;--accent:#3cc4b1;--card:#132a2e}}
body{margin:0;background:var(--bg);color:var(--fg);font:16px/1.6 system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:640px;margin:0 auto;padding:24px 16px 48px}
img.card{width:100%;border-radius:14px;display:block}
h1{font-size:1.6rem;line-height:1.25;margin:20px 0 4px}
.meta{color:var(--muted);font-size:.9rem;margin-bottom:16px}
.tags{color:var(--accent)}
ul{padding-left:20px}
.cta{display:block;text-align:center;background:var(--accent);color:#fff;text-decoration:none;font-weight:600;padding:14px;border-radius:12px;margin-top:28px}
footer{color:var(--muted);font-size:.85rem;text-align:center;margin-top:16px}
</style></head>
<body><main>
<img class="card" src="{{.Card}}" alt="{{.Title}}">
<h1>{{.Title}}</h1>
<div class="meta">{{.Author}} · {{.City}} · {{.Date}}</div>
<p>{{.Body}}</p>
{{if .Places}}<ul>{{range .Places}}<li>{{.}}</li>{{end}}</ul>{{end}}
<p class="tags">{{.Tags}}</p>
<a class="cta" href="{{.AppURL}}">{{.CTA}}</a>
<footer>Yo'ldosh AI — {{.Tagline}}</footer>
</main></body></html>`))

func (s *Server) publicPost(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.PostBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	e := p.Entry
	lang := e.Lang
	city, _ := s.store.City(r.Context(), p.CityID, i18n.Chain(lang))
	places, _ := s.store.PlacesByIDs(r.Context(), e.PlaceIDs, i18n.Chain(lang))
	var names []string
	for _, id := range e.PlaceIDs {
		if pl, ok := places[id]; ok && pl.Status == "published" {
			names = append(names, pl.Name)
		}
	}
	desc := e.Body
	if rs := []rune(desc); len(rs) > 200 {
		desc = string(rs[:197]) + "..."
	}
	cta := map[string]string{"uz": "Yo'ldosh AI da o'z marshrutingizni tuzing", "ru": "Постройте свой маршрут в Yo'ldosh AI", "en": "Plan your own trip with Yo'ldosh AI"}
	tagline := map[string]string{"uz": "sayohatchining aqlli hamrohi", "ru": "умный спутник путешественника", "en": "your smart travel companion"}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	_ = publicPage.Execute(w, map[string]any{
		"Lang": lang, "Title": e.Title, "Desc": desc, "Body": e.Body, "Card": s.postURL(p.Slug) + "/card.png",
		"URL": s.postURL(p.Slug), "Author": authorName(p, lang), "City": city.Name, "Date": e.Date.Format("02.01.2006"),
		"Places": names, "Tags": strings.Join(e.Hashtags, " "), "AppURL": s.cfg.PublicBaseURL + "/",
		"CTA": cta[lang], "Tagline": tagline[lang],
	})
}

func (s *Server) publicCard(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.PostBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	e := *p.Entry
	e.Post = &p
	s.renderCard(w, r, e, "post")
}

// ---------- Admin: postlar ----------

func (s *Server) adminPosts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "reported" {
		status = ""
	}
	if status != "" && !slices.Contains([]string{"visible", "hidden", "removed"}, status) {
		writeValidation(w, r, fieldErrors{"status": "reported | visible | hidden | removed"})
		return
	}
	off, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeValidation(w, r, fieldErrors{"cursor": "invalid"})
		return
	}
	posts, err := s.store.AdminPosts(r.Context(), status, 50, off)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	type item struct {
		postDTO
		Status  string   `json:"status"`
		Reports int      `json:"reports"`
		Reasons []string `json:"reasons"`
	}
	items := make([]item, 0, len(posts))
	for _, rp := range posts {
		d, err := s.postDTO(r, rp.Post)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		items = append(items, item{d, rp.Status, rp.Reports, nonNil(rp.Reasons)})
	}
	var next *string
	if len(posts) == 50 {
		next = encodeCursor(off + 50)
	}
	writeJSON(w, http.StatusOK, page[item]{Items: items, NextCursor: next})
}

func (s *Server) adminPostStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, r, http.StatusNotFound, "post_not_found", false)
			return
		}
		if err := s.store.AdminSetPostStatus(r.Context(), adminOf(r).ID, id, status); isNotFound(err) {
			writeError(w, r, http.StatusNotFound, "post_not_found", false)
			return
		} else if err != nil {
			s.internalError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": status})
	}
}
