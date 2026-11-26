package cleaner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI — OpenAI-mos /chat/completions mijozi. Ollama (http://localhost:11434/v1), llama.cpp server,
// vLLM, LM Studio — hammasi shu formatni beradi. Kalit ixtiyoriy (lokal serverlar talab qilmaydi).
type OpenAI struct {
	BaseURL   string
	Key       string
	Model     string
	Host      string // ixtiyoriy Host sarlavhasi (colibri faqat o'z nomlarini qabul qiladi: "127.0.0.1")
	MaxTokens int    // javob uzunligi chegarasi (sekin CPU model uchun muhim)
	NoThink   bool   // Qwen3: "/no_think" — fikrlash bosqichini o'chiradi (tezroq)
	HC        *http.Client
}

func NewOpenAI(baseURL, key, model string) *OpenAI {
	return &OpenAI{BaseURL: strings.TrimRight(baseURL, "/"), Key: key, Model: model, MaxTokens: 700,
		HC: &http.Client{Timeout: 10 * time.Minute}} // CPU model (~6 token/s) uchun uzun kutish
}

func (c *OpenAI) Name() string { return c.Model }

func (c *OpenAI) Complete(ctx context.Context, system, user string) (string, error) {
	if c.NoThink {
		user += " /no_think"
	}
	payload := map[string]any{
		"model":       c.Model,
		"temperature": 0.1,
		"stream":      false,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	if c.MaxTokens > 0 {
		payload["max_tokens"] = c.MaxTokens
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Host != "" {
		req.Host = c.Host
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	resp, err := c.HC.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("model %d: %s", resp.StatusCode, data)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("model: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("model bo'sh javob qaytardi")
	}
	return out.Choices[0].Message.Content, nil
}
