// guide-try — AI yordamchini terminalda sinash: DATABASE_URL va GROQ_API_KEY (yoki ANTHROPIC_API_KEY) kerak.
//
//	go run ./cmd/guide-try "Samarqandda arzon osh qayerda?"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"yoldosh/internal/assistant"
	"yoldosh/internal/platform"
	"yoldosh/internal/store"
)

func main() {
	ctx := context.Background()
	db, err := platform.OpenDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	m := assistant.New(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("GROQ_API_KEY"), os.Getenv("AI_MODEL"), os.Getenv("AI_EFFORT"))
	q := strings.Join(os.Args[1:], " ")
	req := assistant.Request{Message: q, Lang: assistant.DetectLang(q, "uz")}
	t := assistant.NewTools(store.New(db), req.Lang, nil)
	if m == nil {
		ans, err := assistant.Fallback(ctx, req, t)
		fmt.Println(ans, err)
		return
	}
	fmt.Println("model:", m.Name())
	ans, err := m.Answer(ctx, req, t, func(string, any) {})
	fmt.Println(ans)
	if err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
	}
	for _, p := range t.FoundOrdered() {
		fmt.Println("  [topildi]", p.ID, p.Name)
	}
}
