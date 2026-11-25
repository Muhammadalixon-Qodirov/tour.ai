package diary

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
)

func sampleInput(lang, tone string) StoryInput {
	return StoryInput{Lang: lang, Tone: tone, City: "Buxoro", Date: time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC),
		Interests: []string{"history", "craft"}, WalkingM: 3200,
		Stops: []StopFact{
			{PlaceID: "ark", Name: "Ark qal'asi", Fact: "Qal'a sun'iy tepalik ustida joylashgan."},
			{PlaceID: "suzani", Name: "Suzana ustaxonasi", Novel: true},
			{PlaceID: "lyabi", Name: "Labi Hovuz"},
		}}
}

func TestTemplateStory(t *testing.T) {
	for _, lang := range []string{"uz", "ru", "en"} {
		for _, tone := range Tones {
			in := sampleInput(lang, tone)
			st, err := Template{}.Write(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			if st.Title == "" || st.Body == "" || len(st.Hashtags) == 0 || st.Caption == "" {
				t.Fatalf("%s/%s: bo'sh maydon %+v", lang, tone, st)
			}
			if !strings.Contains(st.Body, "Suzana ustaxonasi") {
				t.Fatalf("%s/%s: yangi joy hikoyada yo'q", lang, tone)
			}
			if st.Highlights[0] != "Suzana ustaxonasi" {
				t.Fatalf("yangi joy birinchi lahza bo'lishi kerak: %v", st.Highlights)
			}
			if err := CheckGrounded(st, in); err != nil {
				t.Fatal(err)
			}
			if len([]rune(st.Caption)) > 300 {
				t.Fatal("caption juda uzun")
			}
		}
	}
	st, _ := Template{}.Write(context.Background(), sampleInput("uz", "warm"))
	if !strings.Contains(st.Body, "3.2 km") || !strings.Contains(st.Body, "sun'iy tepalik") {
		t.Fatalf("masofa yoki fakt yo'q: %s", st.Body)
	}
}

func TestCheckGroundedRejectsInvented(t *testing.T) {
	in := sampleInput("uz", "warm")
	st := Story{Highlights: []string{"Ark qal'asi", "Mavjud bo'lmagan saroy"}}
	if CheckGrounded(st, in) == nil {
		t.Fatal("to'qilgan joy o'tib ketdi")
	}
}

func TestRenderCard(t *testing.T) {
	photo := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for x := range 800 {
		photo.Set(x, x%600, color.RGBA{255, 0, 0, 255})
	}
	for _, f := range []string{"story", "post"} {
		for n := 0; n <= 4; n++ {
			var ph []image.Image
			for range n {
				ph = append(ph, photo)
			}
			b, err := RenderCard(CardInput{Format: f, Title: "Buxoro: Suzana ustaxonasi va boshqa kashfiyotlar — Бухара",
				Highlights: []string{"Suzana ustaxonasi", "Ark qal'asi", "Labi Hovuz"}, City: "Buxoro", Date: "2026-12-02",
				Link: "yoldosh.uz/s/abc", Photos: ph})
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(b))
			if err != nil || cfg.Width != 1080 || (f == "story") != (cfg.Height == 1920) {
				t.Fatalf("%s/%d: %+v %v", f, n, cfg, err)
			}
		}
	}
}
