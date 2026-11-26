package osmimport

import "testing"

func TestParseHours(t *testing.T) {
	h, open24, ok := ParseHours("24/7")
	if !ok || !open24 || len(h) != 7 {
		t.Fatal("24/7")
	}
	h, _, ok = ParseHours("Mo-Fr 09:00-18:00; Sa 10:00-16:00; Su off")
	if !ok || len(h) != 6 || h[0].Day != 1 || h[5].Day != 6 || h[5].Close != "16:00" {
		t.Fatalf("hafta kunlari: %+v", h)
	}
	h, _, ok = ParseHours("Mo-Su 8:00-13:00,14:00-20:00")
	if !ok || len(h) != 7 || h[0].Open != "08:00" || len(h[0].Breaks) != 1 || h[0].Breaks[0].From != "13:00" {
		t.Fatalf("tanaffus: %+v", h)
	}
	h, _, ok = ParseHours("10:00-22:00")
	if !ok || len(h) != 7 {
		t.Fatal("kunsiz vaqt — har kuni")
	}
	h, _, ok = ParseHours("Sa-Mo 10:00-15:00")
	if !ok || len(h) != 3 {
		t.Fatalf("hafta oxiridan o'tuvchi oraliq: %+v", h)
	}
	for _, bad := range []string{"sunrise-sunset", "Mo-Fr 22:00-02:00", "PH off", "by appointment"} {
		if _, _, ok := ParseHours(bad); ok {
			t.Errorf("%q tushunilmasligi kerak edi", bad)
		}
	}
}
