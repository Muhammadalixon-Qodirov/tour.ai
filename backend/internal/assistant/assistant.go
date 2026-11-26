package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// Turn — suhbatdagi oldingi xabar (faqat matn; asbob natijalari saqlanmaydi — har safar bazadan yangidan olinadi).
type Turn struct {
	Role string // user | assistant
	Text string
}

// Request — bitta savol.
type Request struct {
	Message     string
	History     []Turn
	Lang        string
	LangGuessed bool   // Accept-Language yuborilmagan — savol tilidan aniqlanadi
	CityID      string // ilova tanlagan shahar (ixtiyoriy)
	PlaceID     string // joy sahifasidan so'ralsa
	Location    *[2]float64
	Profile     string // "byudjet: tejamkor; qiziqishlar: tarix, ovqat" — ixtiyoriy
}

// Emit — oqim hodisasi: "delta" {text} va h.k. Handler SSE ga yozadi.
type Emit func(event string, data any)

type Claude struct {
	client anthropic.Client
	model  string
	effort anthropic.OutputConfigEffort
}

// NewClaude — kalit bo'sh bo'lsa nil (yordamchi Fallback rejimida ishlaydi).
func NewClaude(apiKey, model, effort string, opts ...option.RequestOption) *Claude {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	if model == "" {
		model = "claude-opus-5-5"
	}
	c := &Claude{client: anthropic.NewClient(append([]option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(2)}, opts...)...), model: model}
	switch effort {
	case "low", "medium", "high":
		c.effort = anthropic.OutputConfigEffort(effort)
	case "":
		c.effort = anthropic.OutputConfigEffortLow // suhbat: tez javob muhim
	}
	return c
}

const systemPrompt = `You are "Safar", a friendly travel assistant for tourists in Uzbekistan, inside the Safar app.

Ground rules (most important):
- Every concrete fact you state — a place, its price level or ticket price, opening hours, address, hotel price, rules — must come
  from the tool results in this conversation. Never invent places, prices, phone numbers or hours from memory.
- If the tools return nothing relevant, say plainly that the app has no verified information on that yet, in the user's language
  (Uzbek: "Bu haqda tekshirilgan ma'lumotim yo'q."), and suggest what you *can* help with. General travel etiquette or how
  things work in Uzbekistan (e.g. bargaining at bazaars, dress code at mosques) may come from general knowledge, phrased as general advice.
- trust "sample" or price_is_estimate=true means approximate data: say "taxminan" / "approximately".
- open_now null means hours are unknown — do not claim a place is open.

How to work:
- Call search_places with the filters the request implies: categories (restaurant, cafe, museum, monument, mosque, bazaar,
  workshop, gallery, park, viewpoint, ecovillage, theatre), words for dishes or features (e.g. ["osh","plov","palov"]),
  family=true for kids, max_price_level 1 for cheap, open_now for "now", near + radius_m when the user shares a location.
  City ids: tashkent, samarkand, bukhara, khiva, fergana, namangan, kokand, margilan, andijan, nukus, urgench, shahrisabz,
  termez, navoi, qarshi, jizzakh, bostanliq. If a search is too narrow, retry with fewer filters (drop words first).
- Use get_place for details (hours, fees, rules) of the places you recommend. search_stays for hotels/hostels, search_prices
  for fair prices (taxi, food, souvenirs, tickets).
- Recommend 3–5 options, best first, each with one short reason grounded in the data (price level, family friendly, open now,
  little-known, distance). Mention the place name exactly as returned so the app can link it.
- Reply in the user's language (uz/ru/en; default Uzbek, Latin script). Be concise: a short intro line and a list. No markdown
  headings or tables; plain lines with "•" bullets are fine.`

var tools = []anthropic.ToolUnionParam{
	tool("search_places", "Search published places (sights, restaurants, cafes, bazaars, workshops...) in the Safar database. Returns brief cards with price_level (0 free..3 expensive), family_friendly, open_now (null = unknown hours), rating, distance.",
		map[string]any{
			"city":            map[string]any{"type": "string", "description": "city id, e.g. samarkand"},
			"categories":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"words":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "word stems matched at word start in name/tagline/description, any of them (e.g. osh, plov, somsa, ceramics)"},
			"family":          map[string]any{"type": "boolean", "description": "only places marked good for children"},
			"wheelchair":      map[string]any{"type": "boolean"},
			"max_price_level": map[string]any{"type": "integer", "minimum": 0, "maximum": 3},
			"novelty":         map[string]any{"type": "string", "enum": []string{"iconic", "new", "hidden", "novel"}},
			"open_now":        map[string]any{"type": "boolean"},
			"near":            map[string]any{"type": "object", "properties": map[string]any{"lat": map[string]any{"type": "number"}, "lng": map[string]any{"type": "number"}}},
			"radius_m":        map[string]any{"type": "number"},
			"limit":           map[string]any{"type": "integer", "minimum": 1, "maximum": 15},
		}, nil),
	tool("get_place", "Full details of one place by id: description, address, opening hours, entry fee, rules, best time, crowd.",
		map[string]any{"id": map[string]any{"type": "string"}}, []string{"id"}),
	tool("search_stays", "Search hotels, hostels and guesthouses. Prices in UZS per night; price_is_estimate=true means approximate.",
		map[string]any{
			"city":          map[string]any{"type": "string"},
			"near":          map[string]any{"type": "object", "properties": map[string]any{"lat": map[string]any{"type": "number"}, "lng": map[string]any{"type": "number"}}},
			"kinds":         map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"hotel", "boutique", "guesthouse", "hostel", "motel", "apartment"}}},
			"max_price_uzs": map[string]any{"type": "integer"},
			"limit":         map[string]any{"type": "integer", "minimum": 1, "maximum": 10},
		}, nil),
	tool("search_prices", "Fair price ranges in UZS (taxi, food, souvenir, market, ticket, transport) for a city.",
		map[string]any{
			"city":     map[string]any{"type": "string"},
			"category": map[string]any{"type": "string", "enum": []string{"taxi", "food", "souvenir", "market", "ticket", "transport"}},
			"q":        map[string]any{"type": "string"},
		}, []string{"city"}),
}

func tool(name, desc string, props map[string]any, required []string) anthropic.ToolUnionParam {
	// Oqimli so'rov + o'z asboblarimiz: eager_input_streaming. Kirish qat'iy json.Unmarshal bilan tekshiriladi (run).
	t := anthropic.ToolParam{Name: name, Description: anthropic.String(desc), EagerInputStreaming: anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{Properties: props, Required: required}}
	return anthropic.ToolUnionParam{OfTool: &t}
}

func (t *Tools) run(ctx context.Context, name string, raw string) (string, bool) {
	var res any
	var err error
	dec := func(v any) error { // qat'iy: noto'g'ri yoki kesilgan JSON — xato natija, model qayta urinadi
		d := json.NewDecoder(strings.NewReader(raw))
		d.DisallowUnknownFields()
		return d.Decode(v)
	}
	switch name {
	case "search_places":
		var in SearchPlacesInput
		if err = dec(&in); err == nil {
			res, err = t.SearchPlaces(ctx, in)
		}
	case "get_place":
		var in GetPlaceInput
		if err = dec(&in); err == nil {
			res, err = t.GetPlace(ctx, in)
		}
	case "search_stays":
		var in SearchStaysInput
		if err = dec(&in); err == nil {
			res, err = t.SearchStays(ctx, in)
		}
	case "search_prices":
		var in SearchPricesInput
		if err = dec(&in); err == nil {
			res, err = t.SearchPrices(ctx, in)
		}
	default:
		err = fmt.Errorf("unknown tool %s", name)
	}
	if err != nil {
		return `{"error":"` + strings.ReplaceAll(err.Error(), `"`, `'`) + `"}`, true
	}
	b, _ := json.Marshal(res)
	return string(b), false
}

// ErrUnavailable — model vaqtincha ishlamayapti (handler Fallback ga o'tadi).
var ErrUnavailable = errors.New("assistant unavailable")

// Answer — model + asboblar sikli; matn bo'laklari emit("delta") orqali oqimda yuboriladi. Qaytadi: to'liq javob matni.
func (c *Claude) Answer(ctx context.Context, req Request, t *Tools, emit Emit) (string, error) {
	var msgs []anthropic.MessageParam
	for _, h := range req.History {
		if strings.TrimSpace(h.Text) == "" {
			continue
		}
		if h.Role == "assistant" {
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(h.Text)))
		} else {
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(h.Text)))
		}
	}
	msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(contextLine(req)+req.Message)))
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 4000,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()}},
		Tools:     tools,
	}
	if c.effort != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: c.effort}
	}
	var full strings.Builder
	streamed := false
	for iter := 0; iter < 6; iter++ {
		params.Messages = msgs
		stream := c.client.Messages.NewStreaming(ctx, params)
		msg := anthropic.Message{}
		for stream.Next() {
			ev := stream.Current()
			if err := msg.Accumulate(ev); err != nil {
				return full.String(), err
			}
			if d, ok := ev.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				if td, ok := d.Delta.AsAny().(anthropic.TextDelta); ok && td.Text != "" {
					full.WriteString(td.Text)
					emit("delta", map[string]string{"text": td.Text})
					streamed = true
				}
			}
		}
		if err := stream.Err(); err != nil {
			if !streamed {
				return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
			}
			return full.String(), err
		}
		// max_tokens/refusal: asbob kirishi kesilgan bo'lishi mumkin — asboblarni ishga tushirmaymiz
		if msg.StopReason == anthropic.StopReasonMaxTokens || msg.StopReason == anthropic.StopReasonRefusal {
			if full.Len() == 0 {
				return "", fmt.Errorf("%w: %s", ErrUnavailable, msg.StopReason)
			}
			return full.String(), nil
		}
		if msg.StopReason != anthropic.StopReasonToolUse {
			return full.String(), nil
		}
		msgs = append(msgs, msg.ToParam())
		var results []anthropic.ContentBlockParamUnion
		for _, b := range msg.Content {
			if tu, ok := b.AsAny().(anthropic.ToolUseBlock); ok {
				out, isErr := t.run(ctx, tu.Name, tu.JSON.Input.Raw())
				results = append(results, anthropic.NewToolResultBlock(tu.ID, out, isErr))
			}
		}
		if full.Len() > 0 && !strings.HasSuffix(full.String(), "\n") {
			full.WriteString("\n")
			emit("delta", map[string]string{"text": "\n"})
		}
		msgs = append(msgs, anthropic.NewUserMessage(results...))
	}
	return full.String(), nil
}

func contextLine(req Request) string {
	var parts []string
	parts = append(parts, "today="+time.Now().In(tashkent).Format("2006-01-02 15:04 Mon"))
	if req.Lang != "" {
		parts = append(parts, "app_language="+req.Lang)
	}
	if req.CityID != "" {
		parts = append(parts, "current_city="+req.CityID)
	}
	if req.PlaceID != "" {
		parts = append(parts, "viewing_place_id="+req.PlaceID)
	}
	if req.Location != nil {
		parts = append(parts, fmt.Sprintf("user_location=%.5f,%.5f", req.Location[0], req.Location[1]))
	}
	if req.Profile != "" {
		parts = append(parts, "profile: "+req.Profile)
	}
	return "[" + strings.Join(parts, "; ") + "]\n"
}
