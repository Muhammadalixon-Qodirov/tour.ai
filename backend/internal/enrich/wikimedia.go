// Package enrich — joylarni ochiq manbalardan boyitadi: Wikimedia Commons rasmlari (ochiq litsenziya, muallif va
// manba bilan) va Vikipediya qisqa tavsiflari (CC BY-SA, manba ko'rsatiladi). Jamoa qo'lda qo'ygan rasm yoki
// tavsif hech qachon ustidan yozilmaydi.
package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/assistant"
)

// Wiki — Wikidata / Commons / Vikipediya API mijozi.
type Wiki struct {
	HC        *http.Client
	UserAgent string
	Delay     time.Duration // so'rovlar orasida (API ga hurmat)
	last      time.Time
}

func NewWiki(ua string) *Wiki {
	return &Wiki{HC: &http.Client{Timeout: 30 * time.Second}, UserAgent: ua, Delay: 400 * time.Millisecond}
}

func (w *Wiki) getJSON(ctx context.Context, u string, v any) error {
	if d := time.Since(w.last); d < w.Delay {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.Delay - d):
		}
	}
	w.last = time.Now()
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", w.UserAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := w.HC.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < 6 {
			// Wikimedia: Retry-After ga amal qilamiz (≤ 5 daq), bo'lmasa 15, 30, 60 ... s; keyingi so'rovlar ham sekinlashadi
			wait := time.Duration(15<<attempt) * time.Second
			if n, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && n > 0 {
				wait = time.Duration(n) * time.Second
			}
			wait = min(wait, 5*time.Minute)
			w.Delay = max(w.Delay, 1*time.Second)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if resp.StatusCode == 404 {
			return errNotFound
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("%s: %d", u, resp.StatusCode)
		}
		return json.Unmarshal(body, v)
	}
}

var errNotFound = fmt.Errorf("not found")

// Entity — Wikidata obyektidan kerakli qism.
type Entity struct {
	Image     string            // P18 fayl nomi ("Sher-Dor Madrasah.jpg")
	Sitelinks map[string]string // "uz" -> maqola nomi
}

// Entities — 50 tagacha QID bitta so'rovda.
func (w *Wiki) Entities(ctx context.Context, ids []string) (map[string]Entity, error) {
	out := map[string]Entity{}
	for i := 0; i < len(ids); i += 25 {
		chunk := ids[i:min(i+25, len(ids))]
		u := "https://www.wikidata.org/w/api.php?action=wbgetentities&props=claims%7Csitelinks&format=json&ids=" +
			url.QueryEscape(strings.Join(chunk, "|"))
		var d struct {
			Entities map[string]struct {
				Claims map[string][]struct {
					Mainsnak struct {
						Datavalue struct {
							Value any `json:"value"`
						} `json:"datavalue"`
					} `json:"mainsnak"`
				} `json:"claims"`
				Sitelinks map[string]struct {
					Title string `json:"title"`
				} `json:"sitelinks"`
			} `json:"entities"`
		}
		if err := w.getJSON(ctx, u, &d); err != nil {
			return out, err
		}
		for q, e := range d.Entities {
			en := Entity{Sitelinks: map[string]string{}}
			if cl := e.Claims["P18"]; len(cl) > 0 {
				if s, ok := cl[0].Mainsnak.Datavalue.Value.(string); ok {
					en.Image = s
				}
			}
			for _, l := range []string{"uz", "ru", "en"} {
				if sl, ok := e.Sitelinks[l+"wiki"]; ok {
					en.Sitelinks[l] = sl.Title
				}
			}
			out[q] = en
		}
	}
	return out, nil
}

// Photo — Commons rasmi litsenziya ma'lumoti bilan.
type Photo struct {
	File      string
	URL       string // 960 px eskiz
	PageURL   string // Commons sahifasi
	License   string
	Credit    string
	Open      bool // qayta ishlatish mumkin (CC0 / PD / CC BY / CC BY-SA)
	MissingOK bool
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

func plain(s string) string {
	s = html.UnescapeString(tagRe.ReplaceAllString(s, ""))
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:117]) + "..."
	}
	return s
}

// OpenLicense — tijoratda qayta ishlatish mumkin bo'lgan litsenziyami (NC/ND va fair use — yo'q).
func OpenLicense(l string) bool {
	x := strings.ToLower(strings.TrimSpace(l))
	if x == "" || strings.Contains(x, "fair use") || strings.Contains(x, "-nc") || strings.Contains(x, " nc") ||
		strings.Contains(x, "-nd") || strings.Contains(x, " nd") || strings.Contains(x, "noncommercial") {
		return false
	}
	return strings.HasPrefix(x, "cc0") || strings.HasPrefix(x, "public domain") || strings.HasPrefix(x, "pd") ||
		strings.HasPrefix(x, "cc by") || strings.HasPrefix(x, "cc-by") || strings.HasPrefix(x, "attribution")
}

// Photos — 50 tagacha fayl uchun eskiz URL va litsenziya.
func (w *Wiki) Photos(ctx context.Context, files []string) (map[string]Photo, error) {
	out := map[string]Photo{}
	for i := 0; i < len(files); i += 50 {
		chunk := files[i:min(i+50, len(files))]
		titles := make([]string, len(chunk))
		for j, f := range chunk {
			titles[j] = "File:" + strings.TrimPrefix(f, "File:")
		}
		u := "https://commons.wikimedia.org/w/api.php?action=query&prop=imageinfo&iiprop=url%7Cextmetadata&iiurlwidth=960&format=json&titles=" +
			url.QueryEscape(strings.Join(titles, "|"))
		var d struct {
			Query struct {
				Normalized []struct{ From, To string } `json:"normalized"`
				Pages      map[string]struct {
					Title     string `json:"title"`
					Imageinfo []struct {
						ThumbURL       string `json:"thumburl"`
						DescriptionURL string `json:"descriptionurl"`
						Extmetadata    map[string]struct {
							Value any `json:"value"`
						} `json:"extmetadata"`
					} `json:"imageinfo"`
				} `json:"pages"`
			} `json:"query"`
		}
		if err := w.getJSON(ctx, u, &d); err != nil {
			return out, err
		}
		back := map[string]string{} // normallashgan nom -> so'ralgan nom
		for _, t := range titles {
			back[t] = t
		}
		for _, n := range d.Query.Normalized {
			back[n.To] = n.From
		}
		for _, p := range d.Query.Pages {
			if len(p.Imageinfo) == 0 {
				continue
			}
			ii := p.Imageinfo[0]
			meta := func(k string) string {
				if v, ok := ii.Extmetadata[k]; ok {
					if s, ok := v.Value.(string); ok {
						return s
					}
				}
				return ""
			}
			lic := plain(meta("LicenseShortName"))
			credit := plain(meta("Artist"))
			if credit == "" {
				credit = plain(meta("Credit"))
			}
			if credit == "" {
				credit = "Wikimedia Commons"
			}
			orig := strings.TrimPrefix(back[p.Title], "File:")
			if orig == "" {
				orig = strings.TrimPrefix(p.Title, "File:")
			}
			thumb := ii.ThumbURL
			if i := strings.IndexByte(thumb, '?'); i > 0 { // utm_ kuzatuv parametrlarisiz
				thumb = thumb[:i]
			}
			ii.ThumbURL = thumb
			out[orig] = Photo{File: orig, URL: ii.ThumbURL, PageURL: ii.DescriptionURL, License: lic, Credit: credit,
				Open: OpenLicense(lic) && ii.ThumbURL != ""}
		}
	}
	return out, nil
}

// NearbyFiles — koordinata atrofidagi (radius m) Commons fayllari.
func (w *Wiki) NearbyFiles(ctx context.Context, lat, lng float64, radius int) ([]string, error) {
	u := fmt.Sprintf("https://commons.wikimedia.org/w/api.php?action=query&list=geosearch&gscoord=%.6f%%7C%.6f&gsradius=%d&gsnamespace=6&gslimit=30&format=json",
		lat, lng, radius)
	var d struct {
		Query struct {
			Geosearch []struct {
				Title string `json:"title"`
			} `json:"geosearch"`
		} `json:"query"`
	}
	if err := w.getJSON(ctx, u, &d); err != nil {
		return nil, err
	}
	var out []string
	for _, g := range d.Query.Geosearch {
		out = append(out, strings.TrimPrefix(g.Title, "File:"))
	}
	return out, nil
}

// Summary — Vikipediya maqolasining qisqa tavsifi (REST API).
type Summary struct {
	Extract string
	URL     string
}

func (w *Wiki) Summary(ctx context.Context, lang, title string) (Summary, error) {
	u := "https://" + lang + ".wikipedia.org/api/rest_v1/page/summary/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	var d struct {
		Type        string `json:"type"`
		Extract     string `json:"extract"`
		ContentURLs struct {
			Desktop struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
	}
	if err := w.getJSON(ctx, u, &d); err != nil {
		return Summary{}, err
	}
	if d.Type == "disambiguation" {
		return Summary{}, errNotFound
	}
	return Summary{Extract: assistant.Sanitize(stripAccents(shortExtract(d.Extract)), lang), URL: d.ContentURLs.Desktop.Page}, nil
}

// shortExtract — birinchi 2 gap, ≤ 420 belgi.
func shortExtract(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	var b strings.Builder
	sentences := 0
	rs := []rune(s)
	for i, r := range rs {
		b.WriteRune(r)
		if (r == '.' || r == '!' || r == '?') && (i+1 == len(rs) || rs[i+1] == ' ') {
			sentences++
			if sentences == 2 {
				break
			}
		}
		if b.Len() > 420 {
			break
		}
	}
	out := strings.TrimSpace(b.String())
	if r := []rune(out); len(r) > 420 {
		out = string(r[:417]) + "..."
	}
	return out
}

// stripAccents — ruscha Vikipediyadagi urg'u belgilari (combining acute U+0301) olib tashlanadi: "Чагата́й" -> "Чагатай".
func stripAccents(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\u0301' || r == '\u0300' {
			return -1
		}
		return r
	}, s)
}
