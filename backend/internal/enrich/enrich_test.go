package enrich

import "testing"

func TestOpenLicense(t *testing.T) {
	ok := []string{"CC BY-SA 4.0", "CC BY 2.0", "CC0", "Public domain", "PD-self", "CC-BY-SA-3.0"}
	bad := []string{"", "Fair use", "CC BY-NC-SA 4.0", "CC BY-ND 2.0", "All rights reserved", "Copyrighted"}
	for _, l := range ok {
		if !OpenLicense(l) {
			t.Errorf("ochiq bo'lishi kerak: %q", l)
		}
	}
	for _, l := range bad {
		if OpenLicense(l) {
			t.Errorf("ochiq emas: %q", l)
		}
	}
}

func TestFileMatches(t *testing.T) {
	cases := []struct {
		file  string
		names []string
		want  bool
	}{
		{"Sher-Dor Madrasah, Samarkand (3).jpg", []string{"Sherdor madrasa"}, true},
		{"Bolo Hauz Mosque Bukhara.jpg", []string{"Bolo Hovuz masjidi", "Bolo Hauz Mosque"}, true},
		{"Registan square at night.jpg", []string{"Sherdor madrasa"}, false},
		{"IMG_2034.JPG", []string{"Chor Minor"}, false},
		{"Chor-Minor Bukhara 2019.jpg", []string{"Chor Minor"}, true},
	}
	for _, c := range cases {
		if got := fileMatches(c.file, c.names); got != c.want {
			t.Errorf("%q vs %v: %v", c.file, c.names, got)
		}
	}
}

func TestShortExtract(t *testing.T) {
	s := shortExtract("Birinchi gap. Ikkinchi gap! Uchinchi gap bu yerda.")
	if s != "Birinchi gap. Ikkinchi gap!" {
		t.Fatalf("%q", s)
	}
}

func TestStripAccents(t *testing.T) {
	if got := stripAccents("Чагата́й Актепа́"); got != "Чагатай Актепа" {
		t.Fatalf("%q", got)
	}
}
