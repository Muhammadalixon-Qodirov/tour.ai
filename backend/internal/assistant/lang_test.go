package assistant

import (
	"strings"
	"testing"
)

func TestDetectLangOK(t *testing.T) {
	cases := []struct {
		msg, lang string
		ok        bool
	}{
		{"Man kimman", "uz", true},
		{"San kimsan", "uz", true},
		{"Salom", "uz", true},
		{"rahmat", "uz", true},
		{"Samarqandda osh qayerda?", "uz", true},
		{"Toshkentda nima ko'rish mumkin", "uz", true},
		{"Кто я?", "ru", true},
		{"Где поесть плов в Самарканде", "ru", true},
		{"Ўзбекистонда қаерга бориш мумкин", "uz", true},
		{"Who am I?", "en", true},
		{"Where can I eat plov in Samarkand?", "en", true},
		{"hello", "en", true},
		{"Registan", "", false},
		{"Samarqanddagi muzeylar", "uz", true},
		{"Buxoroda nima bor", "uz", true},
		{"ok", "", false},
		{"123", "", false},
	}
	for _, c := range cases {
		l, ok := DetectLangOK(c.msg)
		if ok != c.ok || (ok && l != c.lang) {
			t.Errorf("%q: %q %v (kutilgan %q %v)", c.msg, l, ok, c.lang, c.ok)
		}
	}
}

func TestSanitizeMarkdown(t *testing.T) {
	cases := map[string]string{
		"• **Oshqand osh markazi**: arzon": "• Oshqand osh markazi: arzon",
		"* birinchi\n* ikkinchi":           "• birinchi\n• ikkinchi",
		"### Arzon osh\nmatn":              "Arzon osh\nmatn",
		"narx 5 * 2 emas, #1 joy":          "narx 5  2 emas, #1 joy",
	}
	for in, want := range cases {
		if got := Sanitize(in, "uz"); got != want {
			t.Errorf("%q -> %q (kutilgan %q)", in, got, want)
		}
	}
	// oqimda bo'laklarga bo'lingan "**"
	s := NewSanitizer("uz")
	out := s.Push("Bu **Osh") + s.Push("qand*") + s.Push("* joy") + s.Flush()
	if out != "Bu Oshqand joy" {
		t.Errorf("oqim: %q", out)
	}
}

func TestSmallTalkOffTopic(t *testing.T) {
	for _, m := range []string{"Anekdot aytib ber", "Расскажи анекдот", "tell me a joke", "uy vazifamni qil"} {
		ans, ok := SmallTalk(Request{Message: m, ReplyLang: "uz"})
		if !ok || !strings.Contains(ans, "sayohatga yordam beradi") {
			t.Errorf("%q: %v %q", m, ok, ans)
		}
	}
	if _, ok := SmallTalk(Request{Message: "Samarqandda arzon osh qayerda?", ReplyLang: "uz"}); ok {
		t.Error("oddiy savol small talk emas")
	}
}
