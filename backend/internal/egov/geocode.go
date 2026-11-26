package egov

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

// Geocoder — OpenStreetMap Nominatim orqali manzilni koordinataga aylantiradi (bepul, ochiq).
// Nominatim foydalanish qoidasi: ≤ 1 so'rov/sek, aniq User-Agent. Shaxsiy ma'lumot yuborilmaydi.
type Geocoder struct {
	HC        *http.Client
	UserAgent string
	Delay     time.Duration
	Email     string // ixtiyoriy: Nominatim operatorlari uchun generik aloqa (shaxsiy emas)
	last      time.Time
	cache     map[string]*domain.LatLng
}

func NewGeocoder() *Geocoder {
	return &Geocoder{
		HC:        &http.Client{Timeout: 30 * time.Second},
		UserAgent: "SafarBot/1.0 (+https://safar-api.e-investment.uz)",
		Delay:     1100 * time.Millisecond, // > 1 s
		cache:     map[string]*domain.LatLng{},
	}
}

// UZ chegaralari (taxminiy bbox) — natija mamlakat ichida ekanligini tekshirish uchun.
const (
	uzMinLat, uzMaxLat = 37.0, 45.7
	uzMinLng, uzMaxLng = 55.9, 73.3
)

func inUzbekistan(p domain.LatLng) bool {
	return p.Lat >= uzMinLat && p.Lat <= uzMaxLat && p.Lng >= uzMinLng && p.Lng <= uzMaxLng
}

type nominatimHit struct {
	Lat         string   `json:"lat"`
	Lon         string   `json:"lon"`
	Importance  float64  `json:"importance"`
	Class       string   `json:"class"`
	Type        string   `json:"type"`
	PlaceRank   int      `json:"place_rank"`
	BoundingBox []string `json:"boundingbox"`
}

// Match — geokodlash natijasi.
type Match struct {
	Loc     domain.LatLng
	Precise bool // uy/ko'cha darajasida (place_rank yuqori) — ishonchli
	Class   string
}

func (g *Geocoder) throttle(ctx context.Context) error {
	if d := time.Since(g.last); d < g.Delay {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(g.Delay - d):
		}
	}
	g.last = time.Now()
	return nil
}

// Geocode — erkin matnli so'rovni (manzil) koordinataga aylantiradi. Topilmasa (nil, nil).
func (g *Geocoder) Geocode(ctx context.Context, query string) (*Match, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if v, ok := g.cache[query]; ok {
		if v == nil {
			return nil, nil
		}
		return &Match{Loc: *v, Precise: true}, nil
	}
	if err := g.throttle(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("format", "jsonv2")
	q.Set("limit", "1")
	q.Set("countrycodes", "uz")
	q.Set("accept-language", "uz,ru,en")
	if g.Email != "" {
		q.Set("email", g.Email)
	}
	u := "https://nominatim.openstreetmap.org/search?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", g.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := g.HC.Do(req)
	if err != nil {
		return nil, err
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("nominatim %d: %.150s", resp.StatusCode, data)
	}
	var hits []nominatimHit
	if err := json.Unmarshal(data, &hits); err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		g.cache[query] = nil
		return nil, nil
	}
	h := hits[0]
	lat, err1 := strconv.ParseFloat(h.Lat, 64)
	lng, err2 := strconv.ParseFloat(h.Lon, 64)
	if err1 != nil || err2 != nil {
		g.cache[query] = nil
		return nil, nil
	}
	loc := domain.LatLng{Lat: lat, Lng: lng}
	if !inUzbekistan(loc) {
		g.cache[query] = nil
		return nil, nil
	}
	g.cache[query] = &loc
	// place_rank: 30 = uy, 26+ = ko'cha. Shundan past bo'lsa shahar/tuman markazi (taxminiy).
	return &Match{Loc: loc, Precise: h.PlaceRank >= 26, Class: h.Class}, nil
}

// GeocodeAddress — manzilni tuman, so'ng viloyat markaziga geokodlaydi.
//
// Diqqat: O'zbekiston manzillaridagi ko'cha darajasidagi geokodlash Nominatim'da ishonchsiz —
// bir xil ko'cha nomi bir necha tumanda uchraydi va noto'g'ri tumandagi nuqta qaytadi (markazdan ham yomon).
// Shu bois faqat tuman/viloyat markazini olamiz: to'g'ri hudud, taxminiy nuqta (Match.Precise=false).
// Aniq koordinata keyin admin yoki OSM moslashtiruvi orqali qo'yiladi; aniq manzil matni Data'da saqlanadi.
func (g *Geocoder) GeocodeAddress(ctx context.Context, street, district, region string) (*Match, error) {
	_ = street // ko'cha darajasi ataylab ishlatilmaydi (yuqoridagi izohga qarang)
	region = strings.TrimSpace(region)
	district = strings.TrimSpace(district)
	var tries []string
	if district != "" {
		tries = append(tries, district+", "+region+", O‘zbekiston")
	}
	if region != "" {
		tries = append(tries, region+", O‘zbekiston")
	}
	for _, q := range tries {
		m, err := g.Geocode(ctx, q)
		if err != nil {
			return nil, err
		}
		if m != nil {
			m.Precise = false // hudud markazi — aniq emas
			return m, nil
		}
	}
	return nil, nil
}
