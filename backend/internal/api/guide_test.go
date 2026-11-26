package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"yoldosh/internal/assistant"
)

type guideResp struct {
	ConversationID string            `json:"conversationId"`
	Mode           string            `json:"mode"`
	Text           string            `json:"text"`
	Places         []placeSummaryDTO `json:"places"`
}

func TestGuideFallback(t *testing.T) {
	testAPI.limiter = NewLimiter()
	old := testAPI.ai
	testAPI.ai = nil
	defer func() { testAPI.ai = old }()
	c := newClient(t, "guide-device-0001")
	var res guideResp
	if code := c.do("POST", "/api/v1/guide/chat?stream=false", map[string]any{"message": "Samarqandda arzon osh yeyish mumkin bo'lgan joy"}, &res); code != 200 {
		t.Fatalf("chat: %d", code)
	}
	if res.Mode != "fallback" || len(res.Places) == 0 || !strings.Contains(res.Text, "•") {
		t.Fatalf("javob: %+v", res)
	}
	for _, p := range res.Places {
		if p.CityID != "samarkand" || (p.Category != "restaurant" && p.Category != "cafe") || p.PriceLevel > 1 {
			t.Fatalf("filtr ishlamadi: %+v", p)
		}
	}
	// bazada yo'q narsa — to'qimaydi
	var none guideResp
	c.do("POST", "/api/v1/guide/chat?stream=false", map[string]any{"message": "salom", "conversationId": res.ConversationID}, &none)
	if !strings.Contains(none.Text, "tekshirilgan ma'lumotim yo'q") || none.ConversationID != res.ConversationID {
		t.Fatalf("bo'sh javob: %+v", none)
	}
	var conv struct {
		Messages []map[string]any `json:"messages"`
	}
	if code := c.do("GET", "/api/v1/guide/conversations/"+res.ConversationID, nil, &conv); code != 200 || len(conv.Messages) != 4 {
		t.Fatalf("tarix: %d %d", code, len(conv.Messages))
	}
	other := newClient(t, "guide-device-0002")
	if code := other.do("GET", "/api/v1/guide/conversations/"+res.ConversationID, nil, nil); code != 404 {
		t.Fatalf("begona suhbat: %d", code)
	}
}

// fakeAnthropic — Messages API ning SSE oqimini taqlid qiladi: 1-so'rovda search_places asbobini chaqiradi,
// 2-so'rovda asbob natijasidagi birinchi joy nomi bilan javob beradi.
func fakeAnthropic(t *testing.T) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		var body struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Tools []map[string]any `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if len(body.Tools) != 4 || body.Tools[0]["eager_input_streaming"] != true {
			t.Errorf("asboblar: %+v", body.Tools)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(ev string, data string) { fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev, data) }
		send("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":1}}}`)
		if n == 1 {
			send("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"search_places","input":{}}}`)
			send("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"samarkand\",\"categories\":[\"museum\"]}"}}`)
			send("content_block_stop", `{"type":"content_block_stop","index":0}`)
			send("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":5}}`)
		} else {
			// asbob natijasidan birinchi joy nomini olamiz
			last := body.Messages[len(body.Messages)-1].Content
			var blocks []struct {
				Content any `json:"content"`
			}
			json.Unmarshal(last, &blocks)
			raw := fmt.Sprint(blocks[0].Content)
			name := ""
			if i := strings.Index(raw, `"name":"`); i >= 0 {
				rest := raw[i+8:]
				name = rest[:strings.Index(rest, `"`)]
			}
			txt, _ := json.Marshal("Tavsiyam: " + name)
			send("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
			send("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+string(txt)+`}}`)
			send("content_block_stop", `{"type":"content_block_stop","index":0}`)
			send("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`)
		}
		send("message_stop", `{"type":"message_stop"}`)
	}))
	return srv, &calls
}

func TestGuideAI(t *testing.T) {
	testAPI.limiter = NewLimiter()
	fake, calls := fakeAnthropic(t)
	defer fake.Close()
	old := testAPI.ai
	testAPI.ai = assistant.NewClaude("test-key", "claude-opus-5-5", "", option.WithBaseURL(fake.URL), option.WithMaxRetries(0))
	defer func() { testAPI.ai = old }()

	c := newClient(t, "guide-device-0003")
	b, _ := json.Marshal(map[string]any{"message": "Samarqanddagi muzeylar"})
	req, _ := http.NewRequest("POST", testSrv.URL+"/api/v1/guide/chat", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("sse: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	events := map[string][]string{}
	sc := bufio.NewScanner(resp.Body)
	var ev string
	for sc.Scan() {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "event: "); ok {
			ev = v
		} else if v, ok := strings.CutPrefix(line, "data: "); ok {
			events[ev] = append(events[ev], v)
		}
	}
	if calls.Load() != 2 || len(events["delta"]) == 0 || len(events["done"]) != 1 || len(events["places"]) != 1 {
		t.Fatalf("hodisalar (%d chaqiruv): %v", calls.Load(), events)
	}
	var places struct {
		Places []placeSummaryDTO `json:"places"`
	}
	json.Unmarshal([]byte(events["places"][0]), &places)
	if len(places.Places) != 1 || places.Places[0].Category != "museum" || !strings.Contains(events["delta"][0], places.Places[0].Name) {
		t.Fatalf("kartochka: %+v / %v", places, events["delta"])
	}
	if !strings.Contains(events["done"][0], `"mode":"ai"`) {
		t.Fatalf("done: %v", events["done"])
	}
}
