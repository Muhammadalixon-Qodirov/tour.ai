// Package weather — kunlik ob-havo prognozi (Open-Meteo, API kalit kerak emas), 1 soatlik kesh bilan.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"yoldosh/internal/domain"
)

type Forecast struct {
	MaxTempC   float64 `json:"maxTempC"`
	PrecipMM   float64 `json:"precipMm"`
	PrecipProb float64 `json:"precipProbability"`
}

// Bad — sayohat uchun noqulay: jazirama yoki yomg'ir.
func (f Forecast) Bad() bool { return f.MaxTempC >= 35 || f.PrecipMM >= 2 || f.PrecipProb >= 60 }

// Reason — "hot" | "rain" | "".
func (f Forecast) Reason() string {
	switch {
	case f.PrecipMM >= 2 || f.PrecipProb >= 60:
		return "rain"
	case f.MaxTempC >= 35:
		return "hot"
	}
	return ""
}

var ErrUnavailable = errors.New("weather unavailable")

type Source interface {
	Daily(ctx context.Context, at domain.LatLng, date time.Time, tz string) (Forecast, error)
}

type OpenMeteo struct {
	BaseURL string
	HTTP    *http.Client
	mu      sync.Mutex
	cache   map[string]cached
}

type cached struct {
	f  Forecast
	at time.Time
}

func NewOpenMeteo() *OpenMeteo {
	return &OpenMeteo{BaseURL: "https://api.open-meteo.com/v1/forecast", HTTP: &http.Client{Timeout: 3 * time.Second},
		cache: map[string]cached{}}
}

func (o *OpenMeteo) Daily(ctx context.Context, at domain.LatLng, date time.Time, tz string) (Forecast, error) {
	day := date.Format(time.DateOnly)
	// Koordinata ~1 km aniqlikkacha yaxlitlanadi: kesh samarali, aniq joylashuv tashqi xizmatga ketmaydi.
	key := fmt.Sprintf("%.2f,%.2f,%s", at.Lat, at.Lng, day)
	o.mu.Lock()
	if c, ok := o.cache[key]; ok && time.Since(c.at) < time.Hour {
		o.mu.Unlock()
		return c.f, nil
	}
	o.mu.Unlock()

	q := url.Values{
		"latitude": {fmt.Sprintf("%.2f", at.Lat)}, "longitude": {fmt.Sprintf("%.2f", at.Lng)},
		"daily":    {"temperature_2m_max,precipitation_sum,precipitation_probability_max"},
		"timezone": {tz}, "start_date": {day}, "end_date": {day},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return Forecast{}, err
	}
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return Forecast{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// masalan, sana prognoz oralig'idan tashqarida (16 kundan uzoq)
		return Forecast{}, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	var body struct {
		Daily struct {
			Temp []*float64 `json:"temperature_2m_max"`
			Prec []*float64 `json:"precipitation_sum"`
			Prob []*float64 `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Forecast{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	first := func(v []*float64) (float64, bool) {
		if len(v) == 0 || v[0] == nil {
			return 0, false
		}
		return *v[0], true
	}
	temp, ok := first(body.Daily.Temp)
	if !ok {
		return Forecast{}, ErrUnavailable
	}
	prec, _ := first(body.Daily.Prec)
	prob, _ := first(body.Daily.Prob)
	f := Forecast{MaxTempC: temp, PrecipMM: prec, PrecipProb: prob}
	o.mu.Lock()
	o.cache[key] = cached{f, time.Now()}
	o.mu.Unlock()
	return f, nil
}
