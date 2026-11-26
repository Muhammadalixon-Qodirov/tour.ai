package cleaner

import (
	"context"
	"strings"
	"testing"
)

type fakeModel struct {
	reply string
	err   error
}

func (f fakeModel) Name() string { return "fake" }
func (f fakeModel) Complete(_ context.Context, _, _ string) (string, error) {
	return f.reply, f.err
}

func TestClean(t *testing.T) {
	cases := []struct {
		name     string
		item     PendingItem
		reply    string
		decision string
		cat      string
	}{
		{"wikidata muzey -> publish", PendingItem{ID: 1, Source: "wikidata", CityID: ptr("samarkand"), Category: ptr("museum")},
			`{"touristic":true,"category":"museum","descriptions":{"uz":"Afrosiyob muzeyi","ru":"Музей","en":"Museum"},"confidence":0.95,"reason":"wikidata museum"}`,
			"publish", "museum"},
		{"junk -> reject", PendingItem{ID: 2, Source: "osm", Category: ptr("workshop")},
			"```json\n{\"touristic\":false,\"category\":\"workshop\",\"confidence\":0.9,\"reason\":\"car repair\"}\n```",
			"reject", "workshop"},
		{"past ishonch -> review", PendingItem{ID: 3, Source: "osm", CityID: ptr("bukhara"), Category: ptr("monument")},
			`{"touristic":true,"category":"monument","confidence":0.5}`, "review", "monument"},
		{"user manba hech qachon publish emas", PendingItem{ID: 4, Source: "user", CityID: ptr("tashkent"), Category: ptr("cafe")},
			`{"touristic":true,"category":"cafe","confidence":0.99}`, "review", "cafe"},
		{"noto'g'ri kategoriya -> joriy", PendingItem{ID: 5, Source: "wikidata", CityID: ptr("khiva"), Category: ptr("mosque")},
			`{"touristic":true,"category":"temple","confidence":0.95}`, "publish", "mosque"},
	}
	for _, c := range cases {
		r, err := Clean(context.Background(), fakeModel{reply: c.reply}, c.item)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if r.Decision != c.decision || r.Category != c.cat {
			t.Errorf("%s: decision=%s cat=%s (kutilgan %s/%s)", c.name, r.Decision, r.Category, c.decision, c.cat)
		}
		if r.ID != c.item.ID {
			t.Errorf("%s: id %d", c.name, r.ID)
		}
	}
	// JSON bo'lmagan javob — xato
	if _, err := Clean(context.Background(), fakeModel{reply: "kechirasiz, tushunmadim"}, PendingItem{Source: "osm"}); err == nil {
		t.Fatal("JSON bo'lmasa xato kutilgan")
	}
	// descriptions faqat uz/ru/en, bo'shlar tashlanadi
	r, _ := Clean(context.Background(), fakeModel{reply: `{"touristic":true,"category":"park","confidence":0.1,"descriptions":{"uz":"Bog'","fr":"x","ru":"  "}}`},
		PendingItem{Source: "osm", CityID: ptr("navoi")})
	if len(r.Descriptions) != 1 || r.Descriptions["uz"] != "Bog'" {
		t.Fatalf("tavsif filtri: %+v", r.Descriptions)
	}
	if !strings.HasPrefix(r.Reasons[0], "ai:") {
		t.Fatalf("sabab: %v", r.Reasons)
	}
}

func ptr(s string) *string { return &s }

func TestParseReplyThink(t *testing.T) {
	r, err := parseReply("<think>\nfoydalanuvchi {muzey} so'radi...\n</think>\n{\"touristic\":true,\"category\":\"museum\",\"confidence\":0.9}")
	if err != nil || r.Category != "museum" || !r.Touristic {
		t.Fatalf("%v %+v", err, r)
	}
}
