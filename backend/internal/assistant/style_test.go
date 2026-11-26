package assistant

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := []struct{ lang, in, want string }{
		{"uz", "Registon — eng mashhur maydon", "Registon, eng mashhur maydon"},
		{"uz", "• Registon — maydon, kirish bor", "• Registon: maydon, kirish bor"},
		{"uz", "• Registon — maydon — kechqurun chiroyli", "• Registon: maydon, kechqurun chiroyli"},
		{"uz", "Narx ≈ 50 000 so'm", "Narx taxminan 50 000 so'm"},
		{"uz", "Narx ~50 000 so'm", "Narx taxminan 50 000 so'm"},
		{"uz", "≈ 20 daqiqa piyoda", "Taxminan 20 daqiqa piyoda"},
		{"uz", "Narx taxminan ~50 000", "Narx taxminan 50 000"},
		{"ru", "Такси ≈ 15 000 сум", "Такси примерно 15 000 сум"},
		{"en", "Taxi (~15 min)", "Taxi (about 15 min)"},
		{"uz", "45 000–60 000 so'm", "45 000–60 000 so'm"},
		{"uz", "Ochiq – 09:00 dan", "Ochiq, 09:00 dan"},
		{"uz", "— birinchi qator\nikkinchi —", "birinchi qator\nikkinchi"},
		{"uz", "home~dir", "home~dir"},
		{"uz", "Toshkent—Samarqand", "Toshkent, Samarqand"},
		{"uz", "Oddiy matn.", "Oddiy matn."},
	}
	for _, c := range cases {
		if got := Sanitize(c.in, c.lang); got != c.want {
			t.Errorf("Sanitize(%q, %s) = %q, want %q", c.in, c.lang, got, c.want)
		}
	}
}

// Oqim: bo'laklar har qanday joyda bo'linsa ham natija Sanitize(to'liq) bilan bir xil.
func TestSanitizerStreaming(t *testing.T) {
	texts := []string{
		"Bazada topildi:\n• Registon — maydon (≈ 20 daq)\n• Shohizinda — maqbaralar, narx ~40 000 so'm",
		"Такси ≈ 15 000 сум — по счётчику",
		"45 000–60 000 so'm, ochiq – 09:00 dan",
	}
	for _, text := range texts {
		want := Sanitize(text, "uz")
		rs := []rune(text)
		for size := 1; size <= 7; size++ {
			s := NewSanitizer("uz")
			var b strings.Builder
			for i := 0; i < len(rs); i += size {
				b.WriteString(s.Push(string(rs[i:min(i+size, len(rs))])))
			}
			b.WriteString(s.Flush())
			if b.String() != want {
				t.Fatalf("bo'lak %d: %q, want %q", size, b.String(), want)
			}
			if strings.ContainsAny(b.String(), "—≈") {
				t.Fatalf("tozalanmadi: %q", b.String())
			}
		}
	}
}

func TestSanitizeEmit(t *testing.T) {
	var got strings.Builder
	events := []string{}
	emit, flush := SanitizeEmit("ru", func(ev string, data any) {
		events = append(events, ev)
		if ev == "delta" {
			got.WriteString(data.(map[string]string)["text"])
		}
	})
	for _, d := range []string{"Цена ", "~", "5", "0 000 сум —", " недорого"} {
		emit("delta", map[string]string{"text": d})
	}
	emit("places", map[string]any{})
	flush()
	if got.String() != "Цена примерно 50 000 сум, недорого" || !strings.Contains(strings.Join(events, ","), "places") {
		t.Fatalf("%q %v", got.String(), events)
	}
}

// Prompt va zaxira shablonlari yozish qoidalariga mos: birinchi shaxs, "—", "≈", "~" yo'q.
func TestWritingRulesInTemplates(t *testing.T) {
	for key, m := range fbText {
		for lang, v := range m {
			if strings.ContainsAny(v, "—≈~→") {
				t.Errorf("fbText[%s][%s] taqiqlangan belgi: %q", key, lang, v)
			}
			low := " " + strings.ToLower(v) + " "
			for _, w := range []string{" men ", "ma'lumotim", "topganlarim", " я ", "нашёл", "нашел", " i ", "i don't", "i found"} {
				if strings.Contains(low, w) {
					t.Errorf("fbText[%s][%s] birinchi shaxs (%q): %q", key, lang, w, v)
				}
			}
		}
	}
	for _, rule := range []string{"first person", "—", "taxminan", "примерно", "Bazada topildi", "tekshirilgan ma'lumot yo'q",
		"get_trip", "search_events", "get_intercity", "get_services", "get_weather"} {
		if !strings.Contains(systemPrompt, rule) {
			t.Errorf("systemPrompt da yo'q: %q", rule)
		}
	}
	// prompt o'zi namuna sifatida first-person ibora bermasin
	for _, bad := range []string{"ma'lumotim yo'q", "У меня пока нет", "Вот что я нашёл", "Tavsiyam"} {
		if strings.Contains(systemPrompt, bad) {
			t.Errorf("systemPrompt da birinchi shaxs namunasi: %q", bad)
		}
	}
}
