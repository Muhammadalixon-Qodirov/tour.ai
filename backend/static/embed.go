// Package static — binar ichiga joylangan rasmlar: katalog joylari, tadbirlar, turar joylar va shahar muqovalari.
// Fayllarni scripts/catalog/build_seed.py yangilaydi (frontend web/public/places dan nusxa).
package static

import (
	"embed"
	"io/fs"
)

//go:embed places/*.jpg
var files embed.FS

// Places — "/static/places/{file}" uchun fayl tizimi.
func Places() fs.FS {
	sub, err := fs.Sub(files, "places")
	if err != nil {
		panic(err)
	}
	return sub
}
