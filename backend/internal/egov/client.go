// Package egov — data.egov.uz Ochiq ma'lumotlar portali mijozi. Rasmiy davlat datasetlarini o'qiydi
// (faqat o'qish, ochiq ma'lumot). API uch bosqich:
//
//	GET  apiClient/Main/GetSphereList                      -> sohalar (Turizm, Madaniyat ...)
//	GET  apiClient/Main/gettable?sphereId=..&limit&offset  -> sohadagi datasetlar ro'yxati
//	GET  apiClient/Main/GetById?id=<structId>              -> dataset metama'lumoti + ustunlar (tableFields)
//	POST apiClient/Main/GetMainData?GuidId=<structId>      -> dataset qatorlari {guidId,fields:{},limit,offset}
package egov

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const base = "https://data.egov.uz/apiClient/Main"

// Sohalar (GetSphereList'dan olingan barqaror guidId'lar).
const (
	SphereTourism = "607ff03a7b6428eee08802b8" // Turizm
	SphereCulture = "607fed667b6428eee08802b3" // Madaniyat
)

type Client struct {
	HC        *http.Client
	UserAgent string
	Delay     time.Duration // so'rovlar orasida (portalga hurmat)
	last      time.Time
}

func New() *Client {
	return &Client{
		HC:        &http.Client{Timeout: 40 * time.Second},
		UserAgent: "SafarBot/1.0 (+https://safar-api.e-investment.uz; ochiq ma'lumot importeri)",
		Delay:     700 * time.Millisecond,
	}
}

// ml — data.egov.uz ko'p tilli matn bloki.
type ml struct {
	Uz  string `json:"uzbText"`
	UzK string `json:"uzbKrText"`
	Ru  string `json:"rusText"`
	En  string `json:"engText"`
}

func (m ml) best() string {
	for _, s := range []string{m.Uz, m.En, m.Ru, m.UzK} {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

func (m ml) langs() map[string]string {
	out := map[string]string{}
	if s := strings.TrimSpace(m.Uz); s != "" {
		out["uz"] = s
	}
	if s := strings.TrimSpace(m.Ru); s != "" {
		out["ru"] = s
	}
	if s := strings.TrimSpace(m.En); s != "" {
		out["en"] = s
	}
	return out
}

// Dataset — gettable ro'yxatidagi bitta dataset.
type Dataset struct {
	StructID string `json:"structId"`
	Name     string `json:"name"` // "2-001-0394"
	DataName ml     `json:"dataName"`
	OrgName  ml     `json:"orgName"`
}

func (d Dataset) Title() string { return d.DataName.best() }

// Field — dataset ustuni.
type Field struct {
	Name string `json:"name"` // "address", "viloyat" ...
	Text ml     `json:"text"`
}

func (c *Client) throttle(ctx context.Context) error {
	if d := time.Since(c.last); d < c.Delay {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.Delay - d):
		}
	}
	c.last = time.Now()
	return nil
}

func (c *Client) do(ctx context.Context, method, url string, body []byte, v any) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, r)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Referer", "https://data.egov.uz/")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "https://data.egov.uz")
		}
		resp, err := c.HC.Do(req)
		if err != nil {
			if attempt < 3 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(2<<attempt) * time.Second):
				}
				continue
			}
			return err
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < 4 {
			c.Delay = max(c.Delay, 1500*time.Millisecond)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(3<<attempt) * time.Second):
			}
			continue
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("egov %s: %d: %.200s", url, resp.StatusCode, data)
		}
		return json.Unmarshal(data, v)
	}
}

// Datasets — sohadagi barcha datasetlar.
func (c *Client) Datasets(ctx context.Context, sphereID string) ([]Dataset, error) {
	var out struct {
		Result struct {
			Count int       `json:"count"`
			Data  []Dataset `json:"data"`
		} `json:"result"`
	}
	url := fmt.Sprintf("%s/gettable?limit=500&offset=0&sphereId=%s", base, sphereID)
	if err := c.do(ctx, http.MethodGet, url, nil, &out); err != nil {
		return nil, err
	}
	return out.Result.Data, nil
}

// Fields — datasetning ustunlari (tartibda).
func (c *Client) Fields(ctx context.Context, structID string) ([]Field, error) {
	var out struct {
		Result struct {
			TableFields []Field `json:"tableFields"`
		} `json:"result"`
	}
	if err := c.do(ctx, http.MethodGet, base+"/GetById?id="+structID, nil, &out); err != nil {
		return nil, err
	}
	return out.Result.TableFields, nil
}

// Cell — qator katakchasi. Qiymat .Data (matn) yoki .Text (ko'p tilli) yoki .Number da bo'ladi.
type Cell struct {
	Text    *ml             `json:"text"`
	Number  float64         `json:"number"`
	Data    json.RawMessage `json:"data"`
	Default json.RawMessage `json:"default"`
}

// Str — katakchaning eng yaxshi matnli qiymati (joriy til = uz).
func (c Cell) Str() string {
	if c.Text != nil {
		if s := c.Text.best(); s != "" {
			return s
		}
	}
	if s := rawStr(c.Data); s != "" {
		return s
	}
	if s := rawStr(c.Default); s != "" {
		return s
	}
	return ""
}

// Langs — ko'p tilli qiymat (bo'lsa), aks holda bitta uz.
func (c Cell) Langs() map[string]string {
	if c.Text != nil {
		if m := c.Text.langs(); len(m) > 0 {
			return m
		}
	}
	if s := c.Str(); s != "" {
		return map[string]string{"uz": s}
	}
	return map[string]string{}
}

func rawStr(r json.RawMessage) string {
	if len(r) == 0 || string(r) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var f float64
	if err := json.Unmarshal(r, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return ""
}

// Row — bitta qator: ustun nomi -> katakcha.
type Row struct {
	ID          string          `json:"id"`
	TableColumn map[string]Cell `json:"tableColumn"`
}

// Get — ustun qiymati matn ko'rinishida.
func (r Row) Get(col string) string { return r.TableColumn[col].Str() }

// Rows — datasetdan limit ta qatorni offsetdan boshlab oladi.
func (c *Client) Rows(ctx context.Context, structID string, limit, offset int) ([]Row, int, error) {
	body, _ := json.Marshal(map[string]any{
		"guidId": structID,
		"fields": map[string]any{},
		"limit":  limit,
		"offset": offset,
	})
	var out struct {
		Result *struct {
			Count int   `json:"count"`
			Data  []Row `json:"data"`
		} `json:"result"`
		Error struct {
			En string `json:"engText"`
		} `json:"error"`
	}
	url := base + "/GetMainData?GuidId=" + structID
	if err := c.do(ctx, http.MethodPost, url, body, &out); err != nil {
		return nil, 0, err
	}
	if out.Result == nil {
		return nil, 0, fmt.Errorf("egov GetMainData: %s", out.Error.En)
	}
	return out.Result.Data, out.Result.Count, nil
}

// AllRows — datasetning barcha qatorlari (sahifalab).
func (c *Client) AllRows(ctx context.Context, structID string, pageSize int) ([]Row, error) {
	if pageSize <= 0 {
		pageSize = 200
	}
	var all []Row
	for offset := 0; ; offset += pageSize {
		rows, count, err := c.Rows(ctx, structID, pageSize, offset)
		if err != nil {
			return all, err
		}
		all = append(all, rows...)
		if len(rows) == 0 || len(all) >= count {
			break
		}
	}
	return all, nil
}
