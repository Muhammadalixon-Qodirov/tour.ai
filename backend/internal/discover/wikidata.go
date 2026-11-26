package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"yoldosh/internal/domain"
)

// Wikidata — CC0 litsenziya: ma'lumotni o'z bazamizda saqlash va o'zgartirish mumkin.
// Rasmlar (Commons) har xil litsenziyada — avtomatik qo'yilmaydi, faqat admin uchun havola saqlanadi.

// Found — manbadan kelgan bitta joy (manbaga bog'liq bo'lmagan ko'rinish).
type Found struct {
	Source  string            // wikidata | osm | user | research
	Ref     string            // manbadagi id
	Names   map[string]string // uz/ru/en
	Loc     domain.LatLng
	Signals Signals
	Desc    map[string]string // qisqa tavsif tillar bo'yicha
	Data    map[string]any    // admin uchun xom ma'lumot (rasm, havola, izoh ...)
	UserID  string            // foydalanuvchi taklifi bo'lsa
	Trusted bool              // manba ishonchli — ishonch yetarli bo'lsa avtomatik chop etiladi
	// ForceCategory — rasmiy reyestr (masalan egov sertifikatlangan mehmonxonalar) kategoriyani aniq biladi;
	// ehtimollik asosidagi Classify o'rniga shu kategoriya ishlatiladi (ishonch ≥ AutoPublish). Ixtiyoriy.
	ForceCategory string
}

const wikidataEndpoint = "https://query.wikidata.org/sparql"

// UserAgent — Wikimedia tavsiya qilganidek tavsiflovchi UA (shaxsiy ma'lumotsiz). Konfiguratsiyada almashtiriladi.
var UserAgent = "tour-data-sync/1.0 (open-data importer)"

// FetchWikidata — O'zbekistondagi (P17=Q265) koordinatali turistik obyektlar.
func FetchWikidata(ctx context.Context, hc *http.Client) ([]Found, error) {
	var vals strings.Builder
	for _, q := range WikidataClasses() {
		vals.WriteString(" wd:" + q)
	}
	query := `SELECT ?item ?coord ?class ?sl ?uz ?ru ?en ?duz ?dru ?den ?image ?inception ?heritage ?site WHERE {
  ?item wdt:P17 wd:Q265; wdt:P625 ?coord; wdt:P31 ?class; wikibase:sitelinks ?sl.
  VALUES ?class {` + vals.String() + ` }
  OPTIONAL { ?item rdfs:label ?uz FILTER(lang(?uz) = "uz") }
  OPTIONAL { ?item rdfs:label ?ru FILTER(lang(?ru) = "ru") }
  OPTIONAL { ?item rdfs:label ?en FILTER(lang(?en) = "en") }
  OPTIONAL { ?item schema:description ?duz FILTER(lang(?duz) = "uz") }
  OPTIONAL { ?item schema:description ?dru FILTER(lang(?dru) = "ru") }
  OPTIONAL { ?item schema:description ?den FILTER(lang(?den) = "en") }
  OPTIONAL { ?item wdt:P18 ?image }
  OPTIONAL { ?item wdt:P571 ?inception }
  OPTIONAL { ?item wdt:P1435 ?heritage }
  OPTIONAL { ?item wdt:P856 ?site }
}`
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, wikidataEndpoint, strings.NewReader(url.Values{"query": {query}}.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/sparql-results+json")
		req.Header.Set("User-Agent", UserAgent)
		resp, err = hc.Do(req)
		if err != nil {
			return nil, err
		}
		// 429/503 — Wikidata yuklamasi: Retry-After (yoki 30 s) kutib 3 martagacha qayta urinamiz
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) && attempt < 3 {
			wait := 30 * time.Second
			if n, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && n > 0 && n <= 300 {
				wait = time.Duration(n) * time.Second
			}
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		break
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, fmt.Errorf("wikidata: %d %s", resp.StatusCode, b)
	}
	return parseWikidata(resp.Body, time.Now())
}

type sparqlResp struct {
	Results struct {
		Bindings []map[string]struct {
			Value string `json:"value"`
		} `json:"bindings"`
	} `json:"results"`
}

// parseWikidata — bir obyekt bir nechta qatorda keladi (har klass/rasm uchun) — Q bo'yicha birlashtiriladi.
func parseWikidata(r io.Reader, now time.Time) ([]Found, error) {
	var sr sparqlResp
	if err := json.NewDecoder(io.LimitReader(r, 64<<20)).Decode(&sr); err != nil {
		return nil, err
	}
	byQ := map[string]*Found{}
	var order []string
	for _, b := range sr.Results.Bindings {
		q := strings.TrimPrefix(b["item"].Value, "http://www.wikidata.org/entity/")
		if !strings.HasPrefix(q, "Q") {
			continue
		}
		f := byQ[q]
		if f == nil {
			loc, ok := parsePoint(b["coord"].Value)
			if !ok {
				continue
			}
			sl, _ := strconv.Atoi(b["sl"].Value)
			f = &Found{Source: "wikidata", Ref: q, Loc: loc, Names: map[string]string{}, Desc: map[string]string{},
				Data: map[string]any{"url": "https://www.wikidata.org/wiki/" + q}, Trusted: true}
			f.Signals.Sitelinks = sl
			byQ[q] = f
			order = append(order, q)
		}
		for _, l := range []string{"uz", "ru", "en"} {
			if v := b[l].Value; v != "" && !isQID(v) && !strings.HasPrefix(v, "Category:") && !strings.HasPrefix(v, "Turkum:") {
				f.Names[l] = v
			}
			if v := b["d"+l].Value; v != "" {
				f.Desc[l] = v
			}
		}
		if c := strings.TrimPrefix(b["class"].Value, "http://www.wikidata.org/entity/"); c != "" && !contains(f.Signals.WDClasses, c) {
			f.Signals.WDClasses = append(f.Signals.WDClasses, c)
		}
		if v := b["image"].Value; v != "" {
			f.Data["image"] = v // Commons fayli — litsenziyasini admin tekshiradi
		}
		if v := b["site"].Value; v != "" {
			f.Data["website"] = v
		}
		if b["heritage"].Value != "" {
			f.Signals.Heritage = f.Signals.Heritage || strings.HasSuffix(b["heritage"].Value, "/Q9259") // UNESCO
			f.Data["heritage"] = true
		}
		if v := b["inception"].Value; len(v) >= 4 {
			if y, err := strconv.Atoi(strings.TrimPrefix(v, "+")[:4]); err == nil && y >= now.Year()-3 {
				f.Signals.NewSince = true
			}
		}
	}
	out := make([]Found, 0, len(order))
	for _, q := range order {
		f := byQ[q]
		if f.Names["uz"] == "" && f.Names["en"] == "" && f.Names["ru"] == "" {
			continue // nomsiz obyekt foydasiz
		}
		f.Signals.Names = f.Names
		f.Signals.Text = f.Desc["en"] + " " + f.Desc["ru"] + " " + f.Desc["uz"]
		out = append(out, *f)
	}
	return out, nil
}

// parsePoint — "Point(66.97 39.65)" (avval uzunlik).
func parsePoint(s string) (domain.LatLng, bool) {
	s = strings.TrimSuffix(strings.TrimPrefix(s, "Point("), ")")
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return domain.LatLng{}, false
	}
	lng, err1 := strconv.ParseFloat(parts[0], 64)
	lat, err2 := strconv.ParseFloat(parts[1], 64)
	p := domain.LatLng{Lat: lat, Lng: lng}
	return p, err1 == nil && err2 == nil && p.Valid()
}

func isQID(s string) bool {
	if len(s) < 2 || s[0] != 'Q' {
		return false
	}
	_, err := strconv.Atoi(s[1:])
	return err == nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
