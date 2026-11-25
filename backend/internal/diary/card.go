package diary

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Ulashish kartasi: rasm kolaji + sarlavha + lahzalar + "Yo'ldosh AI" belgisi.
// Formatlar: story 1080×1920 (Instagram/Telegram story), post 1080×1350.

type CardInput struct {
	Format     string // story | post
	Title      string
	Highlights []string
	City       string
	Date       string
	Link       string
	Photos     []image.Image
}

var (
	cardBG     = color.RGBA{0x10, 0x2a, 0x2e, 0xff}
	cardAccent = color.RGBA{0x3c, 0xc4, 0xb1, 0xff}
	cardText   = color.RGBA{0xf4, 0xf6, 0xf5, 0xff}
	cardMuted  = color.RGBA{0xa9, 0xbc, 0xb9, 0xff}
)

var (
	fontOnce      sync.Once
	regular, bold *opentype.Font
	errFont       error
	faceMu        sync.Mutex
	faceCache     = map[string]font.Face{}
)

func face(isBold bool, size float64) (font.Face, error) {
	fontOnce.Do(func() {
		if regular, errFont = opentype.Parse(goregular.TTF); errFont == nil {
			bold, errFont = opentype.Parse(gobold.TTF)
		}
	})
	if errFont != nil {
		return nil, errFont
	}
	key := map[bool]string{true: "b", false: "r"}[isBold] + string(rune(int(size)))
	faceMu.Lock()
	defer faceMu.Unlock()
	if f, ok := faceCache[key]; ok {
		return f, nil
	}
	src := regular
	if isBold {
		src = bold
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	faceCache[key] = f
	return f, nil
}

func RenderCard(in CardInput) ([]byte, error) {
	w, h, photoH := 1080, 1920, 1100
	if in.Format == "post" {
		h, photoH = 1350, 720
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{cardBG}, image.Point{}, draw.Src)

	area := image.Rect(0, 0, w, photoH)
	if len(in.Photos) == 0 {
		// rasm yo'q — shahar nomi katta harflarda
		draw.Draw(img, area, &image.Uniform{color.RGBA{0x17, 0x4a, 0x4f, 0xff}}, image.Point{}, draw.Src)
		f, err := face(true, 120)
		if err != nil {
			return nil, err
		}
		drawCentered(img, f, strings.ToUpper(in.City), photoH/2+40, cardAccent)
	} else {
		collage(img, area, in.Photos)
	}
	// accent chiziq
	draw.Draw(img, image.Rect(0, photoH, w, photoH+10), &image.Uniform{cardAccent}, image.Point{}, draw.Src)

	pad := 72
	y := photoH + 120
	titleF, err := face(true, 64)
	if err != nil {
		return nil, err
	}
	for i, line := range wrap(titleF, in.Title, w-2*pad) {
		if i == 3 {
			break
		}
		drawText(img, titleF, line, pad, y, cardText)
		y += 80
	}
	y += 30
	hlF, err := face(false, 42)
	if err != nil {
		return nil, err
	}
	maxHL := 3
	if in.Format == "post" {
		maxHL = 2
	}
	for i, hl := range in.Highlights {
		if i == maxHL {
			break
		}
		lines := wrap(hlF, hl, w-2*pad-50)
		draw.Draw(img, image.Rect(pad, y-22, pad+18, y-4), &image.Uniform{cardAccent}, image.Point{}, draw.Src)
		drawText(img, hlF, lines[0], pad+50, y, cardText)
		y += 64
	}

	brandF, err := face(true, 44)
	if err != nil {
		return nil, err
	}
	smallF, err := face(false, 32)
	if err != nil {
		return nil, err
	}
	drawText(img, brandF, "Yo'ldosh AI", pad, h-140, cardAccent)
	drawText(img, smallF, strings.TrimSpace(in.City+" · "+in.Date), pad, h-90, cardMuted)
	if in.Link != "" {
		lw := font.MeasureString(smallF, in.Link).Ceil()
		drawText(img, smallF, in.Link, w-pad-lw, h-90, cardMuted)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// collage — 1 rasm: to'liq; 2: yonma-yon; 3–4: 2×2 panjara. Har biri markazdan kesiladi.
func collage(dst *image.RGBA, area image.Rectangle, photos []image.Image) {
	if len(photos) > 4 {
		photos = photos[:4]
	}
	gap := 8
	var cells []image.Rectangle
	W, H := area.Dx(), area.Dy()
	switch len(photos) {
	case 1:
		cells = []image.Rectangle{area}
	case 2:
		cells = []image.Rectangle{image.Rect(0, 0, W/2-gap/2, H), image.Rect(W/2+gap/2, 0, W, H)}
	default:
		cells = []image.Rectangle{
			image.Rect(0, 0, W/2-gap/2, H/2-gap/2), image.Rect(W/2+gap/2, 0, W, H/2-gap/2),
			image.Rect(0, H/2+gap/2, W/2-gap/2, H), image.Rect(W/2+gap/2, H/2+gap/2, W, H),
		}
		if len(photos) == 3 { // uchinchi rasm pastki qatorni to'liq egallaydi
			cells = []image.Rectangle{cells[0], cells[1], image.Rect(0, H/2+gap/2, W, H)}
		}
	}
	for i, p := range photos {
		coverFit(dst, cells[i].Add(area.Min), p)
	}
}

func coverFit(dst *image.RGBA, cell image.Rectangle, src image.Image) {
	sb := src.Bounds()
	cw, ch := cell.Dx(), cell.Dy()
	// manbadan katak nisbatidagi markaziy bo'lak
	sw, sh := sb.Dx(), sb.Dy()
	if sw*ch > sh*cw {
		nw := sh * cw / ch
		sb = image.Rect(sb.Min.X+(sw-nw)/2, sb.Min.Y, sb.Min.X+(sw-nw)/2+nw, sb.Max.Y)
	} else {
		nh := sw * ch / cw
		sb = image.Rect(sb.Min.X, sb.Min.Y+(sh-nh)/2, sb.Max.X, sb.Min.Y+(sh-nh)/2+nh)
	}
	draw.ApproxBiLinear.Scale(dst, cell, src, sb, draw.Src, nil)
}

func drawText(dst *image.RGBA, f font.Face, s string, x, y int, c color.Color) {
	d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

func drawCentered(dst *image.RGBA, f font.Face, s string, y int, c color.Color) {
	w := font.MeasureString(f, s).Ceil()
	drawText(dst, f, s, (dst.Bounds().Dx()-w)/2, y, c)
}

// wrap — matnni berilgan kenglikka so'z bo'yicha bo'ladi.
func wrap(f font.Face, s string, width int) []string {
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, word := range words {
		try := strings.TrimSpace(cur + " " + word)
		if font.MeasureString(f, try).Ceil() > width && cur != "" {
			lines = append(lines, cur)
			cur = word
		} else {
			cur = try
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}
