package seed

import (
	"io/fs"
	"strings"
	"testing"

	"yoldosh/static"
)

// Har bir muqova fayli binar ichida bo'lishi va mualliflik ma'lumoti to'liq bo'lishi kerak.
func TestCityCoversFileValid(t *testing.T) {
	covers, err := ReadCityCovers("../../seed/" + CityCoversFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(covers) == 0 {
		t.Fatal("muqovalar yo'q")
	}
	for _, c := range covers {
		dir, name, _ := strings.Cut(c.File, "/")
		files := static.Places()
		if dir == "cities" {
			files = static.Cities()
		}
		if _, err := fs.Stat(files, name); err != nil {
			t.Errorf("%s: %s binar ichida yo'q: %v", c.CityID, c.File, err)
		}
		if !strings.HasPrefix(c.SourceURL, "https://commons.wikimedia.org/wiki/File:") {
			t.Errorf("%s: sourceUrl Commons sahifasi emas: %s", c.CityID, c.SourceURL)
		}
	}
}
