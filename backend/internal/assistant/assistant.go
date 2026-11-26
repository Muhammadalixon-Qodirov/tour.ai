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
	Profile     string    // "byudjet: tejamkor; qiziqishlar: tarix, ovqat" — ixtiyoriy
	Trip        *TripInfo // so'rovda tripId bo'lsa (egasi tekshirilgan)
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

const systemPrompt = `You are the travel assistant inside the Safar app for tourists in Uzbekistan.

Ground rules (most important):
- Every concrete fact you state (a place, price level, ticket price, opening hours, address, hotel price, rules, history,
  dates, events, routes between cities, service locations, weather) must come from the tool results in this conversation
  or from the trip context. Never invent places, prices, phone numbers, hours, dates or history from memory.
- If the tools return nothing relevant, say plainly that there is no verified information on it in the base, in the
  user's language (uz: "Bu haqda tekshirilgan ma'lumot yo'q.", ru: "Проверенной информации об этом нет.",
  en: "There is no verified information on this yet."), then say what the app can help with instead.
- Questions outside the data (visa and registration rules, customs, laws, medical advice, history or facts that get_place
  did not return): do not answer from general knowledge. Say there is no verified information in the base and suggest
  checking an official source such as the embassy. Short etiquette tips that are common sense (dress modestly at a mosque,
  bargaining at a bazaar is normal) may be given as general advice, without numbers.
- trust "sample", price_is_estimate=true or estimate=true means approximate data: say "taxminan" / "примерно" / "about".
- open_now null means hours are unknown: do not claim a place is open.
- Only name places, stays, events and services that appeared in a tool result or the trip context, with the exact returned
  name. Never add one you remember or guess.
- price_level is a 0 to 3 scale, NOT a price: say "bepul"/"arzon"/"o'rtacha"/"qimmat", never "0 so'm". State amounts in
  so'm only when they come from entry_fee, search_prices, search_stays, get_intercity, search_events or the trip.
- Do not embellish: no "most popular", "great reviews", "central location" unless the data says so.
- A null or missing value means "unknown": say there is no verified info on it; never write "null".
- Never show internal field names or values like "price_level", "open_now true", "trust", ids.

Voice and writing (strict):
- Neutral, impersonal voice. Never speak in the first person and never talk about yourself: no "men", "menimcha",
  "tavsiya qilaman", "topdim", "bilmayman", no "я", "рекомендую", "я нашёл", no "I", "I recommend", "I found".
  Start with the subject instead: "Arzon osh uchun:", "Bazada topildi:", "Ertangi reja:", "Для недорогого плова:",
  "В базе найдено:", "For cheap plov:", "Found in the base:".
- Do not mention AI, models or "the assistant".
- Never use the long dash "—" (use a comma, a colon or a full stop), never use "≈" or "~" (write "taxminan" /
  "примерно" / "about"), never use arrows like "→" (write "dan ... gacha" / "от ... до" / "from ... to").
  A plain hyphen between numbers is fine: "45 000-60 000 so'm".
- No emoji, no markdown headings, bold or tables. Plain lines; "•" bullets are fine. Be concise: one short intro line and
  a list. Plain, simple words that any tourist understands.
- Reply in the language the user's message is written in (Russian question, Russian answer), even if app_language
  differs; Uzbek in Latin script by default.

How to work:
- Call search_places with the filters the request implies: categories (restaurant, cafe, museum, monument, mosque, bazaar,
  workshop, gallery, park, viewpoint, ecovillage, theatre), words for dishes or features (e.g. ["osh","plov","palov"]),
  family=true for kids, max_price_level 1 for cheap, open_now for "now", near + radius_m when the user shares a location.
  City ids: tashkent, samarkand, bukhara, khiva, fergana, namangan, kokand, margilan, andijan, nukus, urgench, shahrisabz,
  termez, navoi, qarshi, jizzakh, bostanliq. If a search is too narrow, retry with fewer filters (drop words first).
- get_place: details (hours, fees, rules, history with sources) of the places you name. search_stays: hotels, hostels.
  search_prices: fair prices (taxi, food, souvenirs, tickets). search_events: concerts, festivals, fairs in a city and
  date range. get_intercity: trains, buses, shared taxis and flights between two cities. get_services: nearest ATM,
  pharmacy, currency exchange, hospital, police and the emergency numbers. get_weather: daily forecast for a city.
- Recommend 3 to 5 options, best first, each with one short reason grounded in the data (price level, family friendly,
  open now, little-known, distance). Mention each name exactly as returned so the app can link it.

The user's trip (when the context line has trip_id):
- The context line carries a short plan summary; call get_trip for the full plan (stops, times, transport, costs, start
  point, arrival, preparation). Day numbers for the user start at 1 ("2-kun" = day 2).
- "ertaga" / "завтра" / "tomorrow" and "bugun" / "today" are relative to today in the context line.
- "tushlik qayerda?" / "где обед?": the meal stop of that day.
- Requests to change the plan ("2-kunni yengillashtir", "make day 2 lighter"): the chat cannot edit the plan. Suggest
  concrete changes from the trip data (which stops to drop or move, what the day would then look like) and say the change
  is made in the plan with "Kunni moslash" (day) or the stop's actions button; every change there can be undone.`

// toolSpecs — asboblar tavsifi bitta joyda; Claude va OpenAI-mos (Groq) formatlariga o'giriladi.
var toolSpecs = []toolSpec{
	spec("search_places", "Search published places (sights, restaurants, cafes, bazaars, workshops...) in the Safar database. Returns brief cards with price_level (0 free..3 expensive), family_friendly, open_now (null = unknown hours), rating, distance.",
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
	spec("get_place", "Full details of one place by id: description, address, opening hours, entry fee, rules, best time, crowd.",
		map[string]any{"id": map[string]any{"type": "string"}}, []string{"id"}),
	spec("search_stays", "Search hotels, hostels and guesthouses. Prices in UZS per night; price_is_estimate=true means approximate.",
		map[string]any{
			"city":          map[string]any{"type": "string"},
			"near":          map[string]any{"type": "object", "properties": map[string]any{"lat": map[string]any{"type": "number"}, "lng": map[string]any{"type": "number"}}},
			"kinds":         map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"hotel", "boutique", "guesthouse", "hostel", "motel", "apartment"}}},
			"max_price_uzs": map[string]any{"type": "integer", "description": "per night in Uzbek so'm (1 USD is about 12 000 so'm). Typical: hostel 100 000-250 000, guesthouse 250 000-500 000, hotel 500 000+. Omit unless the user gave a budget"},
			"limit":         map[string]any{"type": "integer", "minimum": 1, "maximum": 10},
		}, nil),
	spec("search_prices", "Fair price ranges in UZS (taxi, food, souvenir, market, ticket, transport) for a city.",
		map[string]any{
			"city":     map[string]any{"type": "string"},
			"category": map[string]any{"type": "string", "enum": []string{"taxi", "food", "souvenir", "market", "ticket", "transport"}},
			"q":        map[string]any{"type": "string"},
		}, []string{"city"}),
	spec("get_trip", "The user's current trip plan when the context line has trip_id: days with dates, stops (arrival time, name, place_id, duration, how to get there, status, reason), cost per person per day, start point, arrival and preparation. No input.",
		map[string]any{}, nil),
	spec("search_events", "Events (festivals, concerts, fairs, exhibitions, sport) in a city between two dates. Default: from today for 14 days.",
		map[string]any{
			"city": map[string]any{"type": "string"},
			"from": map[string]any{"type": "string", "description": "YYYY-MM-DD"},
			"to":   map[string]any{"type": "string", "description": "YYYY-MM-DD"},
		}, nil),
	spec("get_intercity", "Ways to travel between two cities (train, Afrosiyob high-speed train, bus, shared taxi, flight): duration, price from (UZS), estimate flag, source and update date.",
		map[string]any{
			"from": map[string]any{"type": "string", "description": "city id, e.g. tashkent"},
			"to":   map[string]any{"type": "string", "description": "city id, e.g. samarkand"},
		}, []string{"from", "to"}),
	spec("get_services", "Nearest ATMs, pharmacies, currency exchange, hospitals, police for a city or a point, plus the city's emergency phone numbers.",
		map[string]any{
			"city":  map[string]any{"type": "string"},
			"near":  map[string]any{"type": "object", "properties": map[string]any{"lat": map[string]any{"type": "number"}, "lng": map[string]any{"type": "number"}}},
			"kinds": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"atm", "pharmacy", "exchange", "hospital", "clinic", "police", "embassy"}}},
		}, nil),
	spec("get_weather", "Daily weather forecast for a city (today and up to 15 days ahead): max temperature, precipitation, rain probability.",
		map[string]any{
			"city": map[string]any{"type": "string"},
			"date": map[string]any{"type": "string", "description": "YYYY-MM-DD, default today"},
		}, nil),
}

type toolSpec struct {
	Name, Desc string
	Props      map[string]any
	Required   []string
}

func spec(name, desc string, props map[string]any, required []string) toolSpec {
	return toolSpec{name, desc, props, required}
}

// claudeTools — oqimli so'rov + o'z asboblarimiz: eager_input_streaming. Kirish qat'iy tekshiriladi (run).
var claudeTools = func() []anthropic.ToolUnionParam {
	var out []anthropic.ToolUnionParam
	for _, ts := range toolSpecs {
		t := anthropic.ToolParam{Name: ts.Name, Description: anthropic.String(ts.Desc), EagerInputStreaming: anthropic.Bool(true),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: ts.Props, Required: ts.Required}}
		out = append(out, anthropic.ToolUnionParam{OfTool: &t})
	}
	return out
}()

// Model — javob beruvchi til modeli (Claude yoki Groq). nil — Fallback.
type Model interface {
	Answer(ctx context.Context, req Request, t *Tools, emit Emit) (string, error)
	Name() string
}

func (c *Claude) Name() string { return c.model }

// New — sozlamaga qarab model: Anthropic kaliti bo'lsa Claude, aks holda Groq kaliti bo'lsa Groq, bo'lmasa nil.
func New(anthropicKey, groqKey, model, effort string) Model {
	if c := NewClaude(anthropicKey, model, effort); c != nil {
		return c
	}
	if g := NewGroq(groqKey, model); g != nil {
		return g
	}
	return nil
}

func (t *Tools) run(ctx context.Context, name string, raw string) (string, bool) {
	if t.Debug != nil {
		defer func() { t.Debug(name, raw) }()
	}
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
	case "get_trip":
		res, err = t.GetTrip(ctx)
	case "search_events":
		var in SearchEventsInput
		if err = dec(&in); err == nil {
			res, err = t.SearchEvents(ctx, in)
		}
	case "get_intercity":
		var in GetIntercityInput
		if err = dec(&in); err == nil {
			res, err = t.GetIntercity(ctx, in)
		}
	case "get_services":
		var in GetServicesInput
		if err = dec(&in); err == nil {
			res, err = t.GetServices(ctx, in)
		}
	case "get_weather":
		var in GetWeatherInput
		if err = dec(&in); err == nil {
			res, err = t.GetWeather(ctx, in)
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
		Tools:     claudeTools,
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
	if req.Trip != nil {
		parts = append(parts, req.Trip.Brief())
	}
	return "[" + strings.Join(parts, "; ") + "]\n"
}
