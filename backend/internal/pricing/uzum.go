package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Uzum — eng katta market. Ochiq API yo'q: web ilova GraphQL (graphql.uzum.uz) ni anonim Bearer token bilan chaqiradi.
// Token ~12 soatda eskiradi, shuning uchun u sozlamadan (UZUM_BEARER) olinadi; bo'sh bo'lsa Uzum o'tkazib yuboriladi.
// So'rov market-fetch yordamchisi orqali (brauzer TLS izi + O'zbekiston IP), aks holda to'g'ridan-to'g'ri.

var uzumCfg = struct {
	mu                            sync.RWMutex
	bearer, clientName, clientVer string
}{clientName: "web-customers", clientVer: "1.63.2"}

// SetUzum — anonim Bearer token va Apollo client ma'lumotini o'rnatadi (startupda, env dan).
func SetUzum(bearer, clientName, clientVer string) {
	uzumCfg.mu.Lock()
	defer uzumCfg.mu.Unlock()
	uzumCfg.bearer = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(bearer), "Bearer "))
	if clientName != "" {
		uzumCfg.clientName = clientName
	}
	if clientVer != "" {
		uzumCfg.clientVer = clientVer
	}
}

const uzumQuery = `query getMakeSearch($queryInput: MakeSearchQueryInput!) {
  makeSearch(query: $queryInput) {
    items { catalogCard { productId title minSellPrice } }
  }
}`

func uzumMarket() Market {
	return Market{
		ID: "uzum", Name: "Uzum",
		SearchURL: func(q string) string {
			return "https://uzum.uz/uz/search?query=" + strings.ReplaceAll(q, " ", "%20")
		},
		Fetch: func(ctx context.Context, c *Client, q string) ([]Listing, error) {
			uzumCfg.mu.RLock()
			bearer, name, ver := uzumCfg.bearer, uzumCfg.clientName, uzumCfg.clientVer
			uzumCfg.mu.RUnlock()
			if bearer == "" {
				return nil, nil // token yo'q — Uzum o'tkazib yuboriladi
			}
			body, _ := json.Marshal(map[string]any{
				"operationName": "getMakeSearch",
				"query":         uzumQuery,
				"variables": map[string]any{"queryInput": map[string]any{
					"text": q, "pageSize": 20, "showAdultContent": "NONE", "sort": "BY_RELEVANCE_DESC", "filters": []any{}}},
			})
			headers := map[string]string{
				"Authorization":                "Bearer " + bearer,
				"Apollographql-Client-Name":    name,
				"Apollographql-Client-Version": ver,
				"x-iid":                        "safar-price",
			}
			data, err := c.PostJSON(ctx, "https://graphql.uzum.uz/", body, headers)
			if err != nil {
				return nil, err
			}
			var d struct {
				Data struct {
					MakeSearch struct {
						Items []struct {
							CatalogCard struct {
								ProductID    any    `json:"productId"`
								Title        string `json:"title"`
								MinSellPrice any    `json:"minSellPrice"`
							} `json:"catalogCard"`
						} `json:"items"`
					} `json:"makeSearch"`
				} `json:"data"`
				Errors []struct {
					Message string `json:"message"`
				} `json:"errors"`
			}
			if err := json.Unmarshal(data, &d); err != nil {
				return nil, err
			}
			if len(d.Errors) > 0 {
				return nil, fmt.Errorf("uzum: %s", d.Errors[0].Message)
			}
			var out []Listing
			for _, it := range d.Data.MakeSearch.Items {
				cc := it.CatalogCard
				if pr := num(cc.MinSellPrice); pr > 0 && cc.Title != "" {
					out = append(out, Listing{Market: "uzum", Title: cc.Title, Price: pr,
						URL: fmt.Sprintf("https://uzum.uz/uz/product/%v", cc.ProductID)})
				}
			}
			return out, nil
		},
	}
}
