package assistant

import (
	"bufio"
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

// Groq — OpenAI-mos Chat Completions API (https://api.groq.com/openai/v1). Asboblar va oqim Claude bilan bir xil:
// fakt faqat asbob natijalaridan, matn bo'laklari emit("delta") orqali.
type Groq struct {
	key, model, baseURL string
	backup              string // limit (429) tugasa — alohida limitli kichikroq model
	hc                  *http.Client
}

const groqBaseURL = "https://api.groq.com/openai/v1"

// NewGroq — kalit bo'sh bo'lsa nil. model bo'sh — openai/gpt-oss-120b (Groq da asbob chaqira oladigan eng kuchli model).
func NewGroq(key, model string) *Groq {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	if model == "" || strings.HasPrefix(model, "claude") {
		model = "openai/gpt-oss-120b"
	}
	g := &Groq{key: key, model: model, baseURL: groqBaseURL, hc: &http.Client{Timeout: 90 * time.Second}}
	if model == "openai/gpt-oss-120b" {
		g.backup = "openai/gpt-oss-20b"
	}
	return g
}

// WithBaseURL — testlar uchun.
func (g *Groq) WithBaseURL(u string) *Groq { g.baseURL = strings.TrimRight(u, "/"); return g }

func (g *Groq) Name() string { return "groq:" + g.model }

type oaMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

var openAITools = func() []map[string]any {
	var out []map[string]any
	for _, ts := range toolSpecs {
		params := map[string]any{"type": "object", "properties": ts.Props}
		if len(ts.Required) > 0 {
			params["required"] = ts.Required
		}
		out = append(out, map[string]any{"type": "function", "function": map[string]any{"name": ts.Name, "description": ts.Desc, "parameters": params}})
	}
	return out
}()

func (g *Groq) Answer(ctx context.Context, req Request, t *Tools, emit Emit) (string, error) {
	msgs := []oaMessage{{Role: "system", Content: systemPrompt}}
	for _, h := range req.History {
		if strings.TrimSpace(h.Text) == "" {
			continue
		}
		role := "user"
		if h.Role == "assistant" {
			role = "assistant"
		}
		msgs = append(msgs, oaMessage{Role: role, Content: h.Text})
	}
	msgs = append(msgs, oaMessage{Role: "user", Content: contextLine(req) + req.Message})
	var full strings.Builder
	streamed := false
	for iter := 0; iter < 6; iter++ {
		text, calls, finish, err := g.stream(ctx, msgs, func(s string) {
			full.WriteString(s)
			emit("delta", map[string]string{"text": s})
			streamed = true
		})
		if err != nil {
			if !streamed {
				return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
			}
			return full.String(), err
		}
		if finish == "length" { // kesilgan — asboblarni ishga tushirmaymiz
			if full.Len() == 0 {
				return "", fmt.Errorf("%w: length", ErrUnavailable)
			}
			return full.String(), nil
		}
		if len(calls) == 0 {
			return full.String(), nil
		}
		msgs = append(msgs, oaMessage{Role: "assistant", Content: text, ToolCalls: calls})
		for _, c := range calls {
			out, _ := t.run(ctx, c.Function.Name, c.Function.Arguments)
			msgs = append(msgs, oaMessage{Role: "tool", ToolCallID: c.ID, Content: out})
		}
		if full.Len() > 0 && !strings.HasSuffix(full.String(), "\n") {
			full.WriteString("\n")
			emit("delta", map[string]string{"text": "\n"})
		}
	}
	return full.String(), nil
}

// stream — bitta so'rov: matn bo'laklarini onText ga beradi, asbob chaqiruvlarini (index bo'yicha) yig'adi.
func (g *Groq) stream(ctx context.Context, msgs []oaMessage, onText func(string)) (string, []oaToolCall, string, error) {
	model := g.model
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		payload := map[string]any{"model": model, "messages": msgs, "tools": openAITools, "stream": true,
			"temperature": 0.2, "max_completion_tokens": 1500}
		if strings.HasPrefix(model, "openai/gpt-oss") {
			payload["reasoning_effort"] = "low" // fikrlash tokenlari kam — tez va limitga sig'adi
		}
		body, _ := json.Marshal(payload)
		hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", nil, "", err
		}
		hreq.Header.Set("Authorization", "Bearer "+g.key)
		hreq.Header.Set("Content-Type", "application/json")
		resp, err = g.hc.Do(hreq)
		if err != nil {
			return "", nil, "", err
		}
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < 3 {
			// Groq bepul tarif: daqiqasiga token limiti. "try again in 1.9s" / Retry-After ni kutamiz (≤ 8 s),
			// ikkinchi 429 da alohida limitli zaxira modelga o'tamiz.
			wait := retryAfter(resp)
			resp.Body.Close()
			if resp.StatusCode == 429 && attempt >= 1 && g.backup != "" && model != g.backup {
				model = g.backup
				continue
			}
			if wait > 8*time.Second {
				return "", nil, "", fmt.Errorf("groq: limit, %s kutish kerak", wait)
			}
			select {
			case <-ctx.Done():
				return "", nil, "", ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		break
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		return "", nil, "", fmt.Errorf("groq %d: %s", resp.StatusCode, b)
	}
	var text strings.Builder
	calls := map[int]*oaToolCall{}
	var order []int
	finish := ""
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue
		}
		if line == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return text.String(), nil, "", fmt.Errorf("groq: %s", chunk.Error.Message)
		}
		for _, ch := range chunk.Choices {
			if c := ch.Delta.Content; c != "" { // reasoning (fikrlash) maydoni ko'rsatilmaydi
				text.WriteString(c)
				onText(c)
			}
			for _, tc := range ch.Delta.ToolCalls {
				c := calls[tc.Index]
				if c == nil {
					c = &oaToolCall{Type: "function"}
					calls[tc.Index] = c
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					c.ID = tc.ID
				}
				if tc.Function.Name != "" {
					c.Function.Name = tc.Function.Name
				}
				c.Function.Arguments += tc.Function.Arguments
			}
			if ch.FinishReason != nil {
				finish = *ch.FinishReason
			}
		}
	}
	if err := sc.Err(); err != nil {
		return text.String(), nil, "", err
	}
	var out []oaToolCall
	for _, i := range order {
		c := calls[i]
		if c.Function.Arguments == "" {
			c.Function.Arguments = "{}"
		}
		out = append(out, *c)
	}
	return text.String(), out, finish, nil
}

// retryAfter — Retry-After sarlavhasi (soniya) yoki 1 s.
func retryAfter(resp *http.Response) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return time.Duration(f*1000)*time.Millisecond + 200*time.Millisecond
		}
	}
	return time.Second
}
