// Package static — binar ichiga joylangan rasmlar: katalog joylari, tadbirlar, turar joylar va shahar muqovalari.
// places/ ni scripts/catalog/build_seed.py yangilaydi (frontend web/public/places dan nusxa);
// cities/ — shahar muqovalari (Wikimedia Commons, 1600 px), mualliflik ma'lumoti seed/city_covers.json da.
package static

import (
	"embed"
	"io/fs"
)

//go:embed places/*.jpg cities/*.jpg
var files embed.FS

// Places — "/static/places/{file}" uchun fayl tizimi.
func Places() fs.FS { return sub("places") }

// Cities — "/static/cities/{file}" uchun fayl tizimi (shahar muqovalari).
func Cities() fs.FS { return sub("cities") }

func sub(dir string) fs.FS {
	s, err := fs.Sub(files, dir)
	if err != nil {
		panic(err)
	}
	return s
}
