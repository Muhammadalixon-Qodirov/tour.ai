// Package pricing — "Adolatli narx": rasmdagi buyumni AI (Groq vision) bilan aniqlaydi, O'zbekistondagi onlayn
// marketlardan (OLX, Texnomart, Asaxiy, Olcha, Idea, Uzum) joriy narxlarni qidiradi, mos kelmaganlarini (aksessuar,
// boshqa model) ajratadi va o'rtacha narx + oraliqni hisoblaydi.
//
// Marketlarning ochiq (avtorizatsiyasiz) qidiruv sahifalari/endpointlari ishlatiladi, so'rov kam (foydalanuvchi
// so'roviga bitta) va natija 6 soat keshlanadi. Avtorizatsiya talab qiladigan marketlar (masalan Uzum) ulanmagan.
package pricing

import (
	"bytes"
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
	"sync"
	"time"
)

// Listing — bitta marketdagi bitta taklif.
type Listing struct {
	Market string `json:"market"`
	Title  string `json:"title"`
	Price  int64  `json:"price"` // so'm
	URL    string `json:"url"`
	Used   bool   `json:"used,omitempty"` // OLX: ishlatilgan
}

// Market — qidiruv adapteri.
type Market struct {
	ID        string // olx, texnomart, ...
	Name      string // ko'rsatiladigan nom
	SearchURL func(q string) string
	Fetch     func(ctx context.Context, c *Client, q string) ([]Listing, error)
}

// doFn — bitta so'rov (GET yoki POST). Transport: to'g'ridan-to'g'ri, proksi yoki market-fetch yordamchisi.
type doFn func(ctx context.Context, method, u, accept string, body []byte, headers map[string]string) ([]byte, error)

// Client — market adapterlari uchun: Get (ko'pchilik) va PostJSON (Uzum GraphQL). Faqat 200 da bayt qaytaradi.
type Client struct{ do doFn }

func (c *Client) Get(ctx context.Context, u, accept string) ([]byte, error) {
	return c.do(ctx, "GET", u, accept, nil, nil)
}

func (c *Client) PostJSON(ctx context.Context, u string, body []byte, headers map[string]string) ([]byte, error) {
	return c.do(ctx, "POST", u, "application/json", body, headers)
}

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36"

func httpDo(ctx context.Context, hc *http.Client, method, u, accept string, body []byte, headers map[string]string) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "uz,ru;q=0.9")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: %d", u, resp.StatusCode)
	}
	return data, nil
}

func num(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return int64(n)
	case json.Number:
		n, _ := x.Float64()
		return int64(n)
	}
	return 0
}

var Markets = []Market{
	{
		ID: "olx", Name: "OLX",
		SearchURL: func(q string) string {
			return "https://www.olx.uz/list/q-" + url.PathEscape(strings.ReplaceAll(q, " ", "-")) + "/"
		},
		Fetch: func(ctx context.Context, c *Client, q string) ([]Listing, error) {
			b, err := c.Get(ctx, "https://www.olx.uz/api/v1/offers/?offset=0&limit=30&query="+url.QueryEscape(q), "application/json")
			if err != nil {
				return nil, err
			}
			var d struct {
				Data []struct {
					Title  string `json:"title"`
					URL    string `json:"url"`
					Params []struct {
						Key   string         `json:"key"`
						Value map[string]any `json:"value"`
					} `json:"params"`
				} `json:"data"`
			}
			if err := json.Unmarshal(b, &d); err != nil {
				return nil, err
			}
			var out []Listing
			for _, o := range d.Data {
				l := Listing{Market: "olx", Title: o.Title, URL: o.URL}
				for _, p := range o.Params {
					switch p.Key {
					case "price":
						// "у.е." (dollar) bo'lsa OLX o'zi so'mga o'girib beradi (converted_value)
						if cv := num(p.Value["converted_value"]); cv > 0 && p.Value["converted_currency"] == "UZS" {
							l.Price = cv
						} else if p.Value["currency"] == "UZS" {
							l.Price = num(p.Value["value"])
						}
					case "state":
						l.Used = p.Value["key"] == "used"
					}
				}
				if l.Price > 0 {
					out = append(out, l)
				}
			}
			return out, nil
		},
	},
	{
		ID: "texnomart", Name: "Texnomart",
		SearchURL: func(q string) string { return "https://texnomart.uz/uz/search?q=" + url.QueryEscape(q) },
		Fetch: func(ctx context.Context, c *Client, q string) ([]Listing, error) {
			b, err := c.Get(ctx, "https://gw.texnomart.uz/api/common/v1/search/result?q="+url.QueryEscape(q), "application/json")
			if err != nil {
				return nil, err
			}
			var d struct {
				Data struct {
					Products []struct {
						ID        any    `json:"id"`
						Name      string `json:"name"`
						SalePrice any    `json:"sale_price"`
					} `json:"products"`
				} `json:"data"`
			}
			if err := json.Unmarshal(b, &d); err != nil {
				return nil, err
			}
			var out []Listing
			for _, p := range d.Data.Products {
				if pr := num(p.SalePrice); pr > 0 {
					out = append(out, Listing{Market: "texnomart", Title: p.Name, Price: pr,
						URL: fmt.Sprintf("https://texnomart.uz/uz/product/detail/%v", p.ID)})
				}
			}
			return out, nil
		},
	},
	{
		ID: "asaxiy", Name: "Asaxiy",
		SearchURL: func(q string) string { return "https://asaxiy.uz/product?key=" + url.QueryEscape(q) },
		Fetch: func(ctx context.Context, c *Client, q string) ([]Listing, error) {
			b, err := c.Get(ctx, "https://asaxiy.uz/product?key="+url.QueryEscape(q), "text/html")
			if err != nil {
				return nil, err
			}
			var out []Listing
			for _, c := range strings.Split(string(b), `class="product__item d-flex`)[1:] {
				pm := asaxiyPrice.FindStringSubmatch(c)
				hm := asaxiyHref.FindStringSubmatch(c)
				tm := asaxiyTitle.FindStringSubmatch(c)
				if pm == nil || tm == nil {
					continue
				}
				pr, _ := strconv.ParseInt(pm[1], 10, 64)
				l := Listing{Market: "asaxiy", Title: strings.TrimSpace(html.UnescapeString(tm[1])), Price: pr}
				if hm != nil {
					l.URL = "https://asaxiy.uz" + hm[1]
				}
				if pr > 0 {
					out = append(out, l)
				}
			}
			return out, nil
		},
	},
	{
		ID: "olcha", Name: "Olcha",
		SearchURL: func(q string) string { return "https://olcha.uz/uz/search?q=" + url.QueryEscape(q) },
		Fetch: func(ctx context.Context, c *Client, q string) ([]Listing, error) {
			b, err := c.Get(ctx, "https://api.olcha.uz/api/v2/products?q="+url.QueryEscape(q), "application/json")
			if err != nil {
				return nil, err
			}
			var d struct {
				Data struct {
					Products []struct {
						NameUz     string `json:"name_uz"`
						NameRu     string `json:"name_ru"`
						Alias      string `json:"alias"`
						TotalPrice any    `json:"total_price"`
					} `json:"products"`
				} `json:"data"`
			}
			if err := json.Unmarshal(b, &d); err != nil {
				return nil, err
			}
			var out []Listing
			for _, p := range d.Data.Products {
				name := p.NameUz
				if name == "" {
					name = p.NameRu
				}
				if pr := num(p.TotalPrice); pr > 0 {
					out = append(out, Listing{Market: "olcha", Title: name, Price: pr, URL: "https://olcha.uz/uz/product/view/" + p.Alias})
				}
			}
			return out, nil
		},
	},
	{
		ID: "idea", Name: "Idea",
		SearchURL: func(q string) string { return "https://idea.uz/uz/search?q=" + url.QueryEscape(q) },
		Fetch: func(ctx context.Context, c *Client, q string) ([]Listing, error) {
			b, err := c.Get(ctx, "https://api.idea.uz/api/v2/products?search="+url.QueryEscape(q), "application/json")
			if err != nil {
				return nil, err
			}
			var d struct {
				Data []struct {
					Name         string `json:"name"`
					Slug         string `json:"slug"`
					CurrentPrice any    `json:"current_price"`
				} `json:"data"`
			}
			if err := json.Unmarshal(b, &d); err != nil {
				return nil, err
			}
			var out []Listing
			for _, p := range d.Data {
				if pr := num(p.CurrentPrice); pr > 0 {
					out = append(out, Listing{Market: "idea", Title: p.Name, Price: pr, URL: "https://idea.uz/uz/product/" + p.Slug})
				}
			}
			return out, nil
		},
	},
}

func init() { Markets = append(Markets, uzumMarket()) }

var (
	asaxiyPrice = regexp.MustCompile(`data-actual-price="(\d+)"`)
	asaxiyHref  = regexp.MustCompile(`<a href="(/product/[^"]+)"`)
	asaxiyTitle = regexp.MustCompile(`class="product__item__info-title[^"]*"[^>]*>([^<]+)`)
)

// MarketResult — bitta market qidiruvi natijasi (xato bo'lsa Err).
type MarketResult struct {
	Market   Market
	Listings []Listing
	Err      error
}

type cacheEntry struct {
	at  time.Time
	res []MarketResult
}

// Searcher — marketlarda parallel qidiradi, natijani keshlaydi.
type Searcher struct {
	client   *Client // asosiy yo'l (market-fetch yordamchisi, proksi yoki to'g'ridan-to'g'ri)
	fallback *Client // asosiy ishlamasa (odatda to'g'ridan-to'g'ri); nil — yo'q
	Markets  []Market
	TTL      time.Duration
	mu       sync.Mutex
	cache    map[string]cacheEntry
}

// NewSearcher — market so'rovlarini qanday yuborishni tanlaydi:
//   - fetchURL (market-fetch yordamchisi, masalan "http://market-fetch:8890"): yordamchi curl bilan oladi
//     (brauzer TLS izi — OLX, Uzum kabi himoyalangan saytlar uchun), O'zbekiston proksisi orqali;
//   - fetchURL bo'sh, proxyURL bor: Go mijozi proksi orqali (ba'zi saytlar bloklashi mumkin);
//   - ikkalasi bo'sh: to'g'ridan-to'g'ri.
//
// Asosiy yo'l ishlamasa to'g'ridan-to'g'ri (fallback) bilan qayta urinadi.
func NewSearcher(proxyURL, fetchURL string) *Searcher {
	direct := &http.Client{Timeout: 10 * time.Second}
	directClient := &Client{do: func(ctx context.Context, m, u, a string, b []byte, h map[string]string) ([]byte, error) {
		return httpDo(ctx, direct, m, u, a, b, h)
	}}
	s := &Searcher{Markets: Markets, TTL: 6 * time.Hour, cache: map[string]cacheEntry{}, client: directClient}
	switch {
	case strings.TrimSpace(fetchURL) != "":
		base := strings.TrimRight(fetchURL, "/")
		fh := &http.Client{Timeout: 20 * time.Second}
		s.client = &Client{do: func(ctx context.Context, m, u, a string, b []byte, h map[string]string) ([]byte, error) {
			return viaFetch(ctx, fh, base, m, u, a, b, h)
		}}
		s.fallback = directClient
	case strings.TrimSpace(proxyURL) != "":
		if pu, err := url.Parse(strings.TrimSpace(proxyURL)); err == nil && pu.Host != "" {
			hc := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu),
				MaxIdleConnsPerHost: 4, IdleConnTimeout: 60 * time.Second}}
			s.client = &Client{do: func(ctx context.Context, m, u, a string, b []byte, h map[string]string) ([]byte, error) {
				return httpDo(ctx, hc, m, u, a, b, h)
			}}
			s.fallback = directClient
		}
	}
	return s
}

// viaFetch — market-fetch yordamchisidan sahifa so'raydi (POST {url, method, accept, headers, body} -> {status, body}).
func viaFetch(ctx context.Context, hc *http.Client, base, method, u, accept string, body []byte, headers map[string]string) ([]byte, error) {
	spec := map[string]any{"url": u, "accept": accept, "method": method}
	if body != nil {
		spec["body"] = string(body)
	}
	if len(headers) > 0 {
		spec["headers"] = headers
	}
	reqBody, _ := json.Marshal(spec)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/fetch", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("market-fetch %d: %.120s", resp.StatusCode, data)
	}
	var out struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("market-fetch: %s", out.Error)
	}
	if out.Status != 200 {
		return nil, fmt.Errorf("%s: %d", u, out.Status)
	}
	return []byte(out.Body), nil
}

// Search — barcha marketlarda parallel qidiruv (har biri ≤ 8 s); bitta market ishlamasa qolganlari qaytadi.
func (s *Searcher) Search(ctx context.Context, q string) []MarketResult {
	key := strings.ToLower(strings.TrimSpace(q))
	s.mu.Lock()
	if e, ok := s.cache[key]; ok && time.Since(e.at) < s.TTL {
		s.mu.Unlock()
		return e.res
	}
	s.mu.Unlock()
	res := make([]MarketResult, len(s.Markets))
	var wg sync.WaitGroup
	for i, m := range s.Markets {
		wg.Add(1)
		go func(i int, m Market) {
			defer wg.Done()
			ls, err := m.Fetch(ctx, s.client, q)
			if err != nil && s.fallback != nil && ctx.Err() == nil { // asosiy yo'l ishlamadi — to'g'ridan-to'g'ri
				if ls2, err2 := m.Fetch(ctx, s.fallback, q); err2 == nil {
					ls, err = ls2, nil
				}
			}
			res[i] = MarketResult{Market: m, Listings: ls, Err: err}
		}(i, m)
	}
	wg.Wait()
	ok := 0
	for _, r := range res {
		if r.Err == nil {
			ok++
		}
	}
	if ok > 0 {
		s.mu.Lock()
		if len(s.cache) > 2000 { // oddiy chegara
			s.cache = map[string]cacheEntry{}
		}
		s.cache[key] = cacheEntry{time.Now(), res}
		s.mu.Unlock()
	}
	return res
}
