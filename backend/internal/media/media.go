// Package media — rasm yuklash: tekshirish, kichraytirish, qayta kodlash (EXIF/GPS o'chadi) va saqlash.
// Hozircha lokal disk; keyin S3 mos ombor shu interfeys orqali ulanadi.
package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

const (
	MaxUploadBytes = 8 << 20 // 8 MB
	MaxSourcePx    = 8000    // juda katta rasm (dekompressiya bombasi) rad etiladi
	MaxLongSide    = 1600
	jpegQuality    = 85
)

var (
	ErrUnsupported = errors.New("faqat JPEG yoki PNG")
	ErrTooLarge    = errors.New("rasm juda katta")
)

type Stored struct {
	URL    string
	Width  int
	Height int
}

type Store interface {
	Save(name string, data []byte) (url string, err error)
}

// Process — rasmni o'qiydi, tekshiradi, uzun tomonini MaxLongSide gacha kichraytiradi va JPEG ga qayta kodlaydi.
// Qayta kodlash metama'lumotni (EXIF, GPS) olib tashlaydi.
func Process(r io.Reader) ([]byte, int, int, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return nil, 0, 0, err
	}
	if len(raw) > MaxUploadBytes {
		return nil, 0, 0, ErrTooLarge
	}
	switch http.DetectContentType(raw) {
	case "image/jpeg", "image/png":
	default:
		return nil, 0, 0, ErrUnsupported
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, ErrUnsupported
	}
	if cfg.Width > MaxSourcePx || cfg.Height > MaxSourcePx || cfg.Width < 1 || cfg.Height < 1 {
		return nil, 0, 0, ErrTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, ErrUnsupported
	}
	w, h := cfg.Width, cfg.Height
	if long := max(w, h); long > MaxLongSide {
		w, h = w*MaxLongSide/long, h*MaxLongSide/long
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src) // PNG shaffofligi oq fonga
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), w, h, nil
}

// NewName — tasodifiy fayl nomi (mijoz nomi hech qachon ishlatilmaydi).
func NewName() string { return uuid.Must(uuid.NewV7()).String() + ".jpg" }

var safeName = regexp.MustCompile(`^[0-9a-f-]{36}\.jpg$`)

func SafeName(name string) bool { return safeName.MatchString(name) }

// Local — fayllarni diskda saqlaydi va {baseURL}/uploads/{name} orqali beradi.
type Local struct {
	Dir     string
	BaseURL string
}

func (l Local) Save(name string, data []byte) (string, error) {
	if !SafeName(name) {
		return "", fmt.Errorf("xavfsiz bo'lmagan nom %q", name)
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return "", err
	}
	tmp := filepath.Join(l.Dir, "."+name+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, filepath.Join(l.Dir, name)); err != nil {
		return "", err
	}
	return l.BaseURL + "/uploads/" + name, nil
}
