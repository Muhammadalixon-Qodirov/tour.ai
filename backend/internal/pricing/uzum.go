package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Uzum — eng katta market. Ochiq API yo'q: web ilova GraphQL (graphql.uzum.uz, makeSearch) ni anonim Bearer token
// bilan chaqiradi. Token ~12 soatda eskiradi, shuning uchun sozlamadan (UZUM_BEARER) olinadi; bo'sh bo'lsa Uzum
// o'tkazib yuboriladi. So'rov market-fetch yordamchisi orqali (brauzer TLS izi + O'zbekiston IP).

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

// UzumReady — token o'rnatilganmi (admin/diagnostika uchun).
func UzumReady() bool {
	uzumCfg.mu.RLock()
	defer uzumCfg.mu.RUnlock()
	return uzumCfg.bearer != ""
}

const uzumQuery = `query getMakeSearch($queryInput: MakeSearchQueryInput!) {
  makeSearch(query: $queryInput) {
    total
    items {
      catalogCard {
        minSellPrice
        discovery {
          title
          ... on DiscoveryProductCard { id }
          ... on DiscoverySkuCard { id productId }
          ... on DiscoverySkuGroupCard { id productId }
        }
      }
    }
  }
}`

func uzumMarket() Market {
	return Market{
		ID: "uzum", Name: "Uzum",
		SearchURL: func(q string) string {
			return "https://uzum.uz/uz/search?query=" + strings.ReplaceAll(strings.TrimSpace(q), " ", "%20")
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
					"text": q, "pagination": map[string]int{"offset": 0, "limit": 15},
					"showAdultContent": "NONE", "sort": "BY_RELEVANCE_DESC", "filters": []any{}}},
			})
			headers := map[string]string{
				"Authorization":                "Bearer " + bearer,
				"Apollographql-Client-Name":    name,
				"Apollographql-Client-Version": ver,
				"x-iid":                        "safar-price-safar",
				"Origin":                       "https://uzum.uz",
				"Referer":                      "https://uzum.uz/",
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
								MinSellPrice any `json:"minSellPrice"`
								Discovery    struct {
									Title     string `json:"title"`
									ID        any    `json:"id"`
									ProductID any    `json:"productId"`
								} `json:"discovery"`
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
			seen := map[string]bool{}
			var out []Listing
			for _, it := range d.Data.MakeSearch.Items {
				cc := it.CatalogCard
				pid := idStr(firstNonNil(cc.Discovery.ProductID, cc.Discovery.ID))
				if pr := num(cc.MinSellPrice); pr > 0 && cc.Discovery.Title != "" && pid != "" && !seen[pid] {
					seen[pid] = true
					out = append(out, Listing{Market: "uzum", Title: cc.Discovery.Title, Price: pr,
						URL: "https://uzum.uz/uz/product/" + pid})
				}
			}
			return out, nil
		},
	}
}

// idStr — JSON raqamini (float64) butun son satriga; satr bo'lsa o'zicha.
func idStr(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case json.Number:
		return x.String()
	case string:
		return x
	case nil:
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func firstNonNil(xs ...any) any {
	for _, x := range xs {
		if x != nil {
			return x
		}
	}
	return nil
}
