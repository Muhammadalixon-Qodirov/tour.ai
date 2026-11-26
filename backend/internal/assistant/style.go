package assistant

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Yozish qoidalari (CLAUDE.md, Apple HIG "Writing"): uzun chiziq "—", "≈" va raqam oldidagi "~" ishlatilmaydi.
// Prompt modelga shuni aytadi; Sanitize — oxirgi himoya: model baribir yozsa, foydalanuvchiga borguncha tuzatiladi.

// ApproxWord — "≈" va "~" o'rniga yoziladigan so'z.
func ApproxWord(lang string) string {
	switch lang {
	case "ru":
		return "примерно"
	case "en":
		return "about"
	}
	return "taxminan"
}

func isDashRune(r rune) bool { return r == '—' || r == '–' }

// special — oqimda keyingi bo'lak kelmaguncha qaror qilib bo'lmaydigan belgilar.
func special(r rune) bool { return r == '—' || r == '–' || r == '≈' || r == '~' }

// Sanitize — "—" (va bo'shliqlar orasidagi "–") vergul yoki ikki nuqtaga, "≈" va raqam oldidagi "~" "taxminan" ga almashadi.
// Raqamlar orasidagi "–" (45 000–60 000) o'zgarmaydi.
func Sanitize(s, lang string) string {
	if !strings.ContainsFunc(s, special) {
		return s
	}
	word := []rune(ApproxWord(lang))
	rs := []rune(s)
	b := make([]rune, 0, len(rs)+16)
	skipSpaces := func(j int) int {
		for j < len(rs) && (rs[j] == ' ' || rs[j] == ' ') {
			j++
		}
		return j
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '—' || r == '–' && dashAsPunct(rs, i):
			b = trimSpaces(b)
			j := skipSpaces(i + 1)
			i = j - 1
			if len(b) == 0 || b[len(b)-1] == '\n' || j >= len(rs) || rs[j] == '\n' {
				continue // qator boshidagi yoki oxiridagi chiziq: olib tashlanadi
			}
			if last := b[len(b)-1]; strings.ContainsRune(",.:;!?(", last) {
				if last != '(' {
					b = append(b, ' ')
				}
				continue
			}
			if colonFits(b) {
				b = append(b, ':', ' ')
			} else {
				b = append(b, ',', ' ')
			}
		case r == '≈' || r == '~' && digitAfter(rs, i):
			j := skipSpaces(i + 1)
			i = j - 1
			if j >= len(rs) || rs[j] == '\n' {
				b = trimSpaces(b)
				continue
			}
			if endsWithWord(b, word) {
				if b[len(b)-1] != ' ' {
					b = append(b, ' ')
				}
				continue
			}
			w := word
			if sentenceStart(b) {
				w = append([]rune{unicode.ToUpper(w[0])}, w[1:]...)
			}
			if len(b) > 0 && !unicode.IsSpace(b[len(b)-1]) && b[len(b)-1] != '(' {
				b = append(b, ' ')
			}
			b = append(b, w...)
			b = append(b, ' ')
		default:
			b = append(b, r)
		}
	}
	return string(b)
}

// dashAsPunct — "–" tinish belgisi sifatida: ikki tomonida bo'shliq bor yoki qator boshida.
func dashAsPunct(rs []rune, i int) bool {
	before := i == 0 || unicode.IsSpace(rs[i-1])
	after := i == len(rs)-1 || unicode.IsSpace(rs[i+1])
	return before && after
}

func digitAfter(rs []rune, i int) bool {
	for j := i + 1; j < len(rs); j++ {
		if rs[j] == ' ' {
			continue
		}
		return unicode.IsDigit(rs[j])
	}
	return false
}

func trimSpaces(b []rune) []rune {
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

func endsWithWord(b []rune, word []rune) bool {
	t := trimSpaces(b)
	if len(t) < len(word) {
		return false
	}
	tail := t[len(t)-len(word):]
	return strings.EqualFold(string(tail), string(word))
}

func sentenceStart(b []rune) bool {
	t := trimSpaces(b)
	if len(t) == 0 {
		return true
	}
	switch t[len(t)-1] {
	case '\n', '.', '!', '?', '•', ':':
		return true
	}
	return false
}

// colonFits — ro'yxat qatoridagi birinchi chiziq ("• Registon — maydon") ikki nuqtaga aylanadi; qolganlari vergulga.
func colonFits(b []rune) bool {
	line := b
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == '\n' {
			line = b[i+1:]
			break
		}
	}
	l := strings.TrimSpace(string(line))
	if strings.ContainsAny(l, ":,") {
		return false
	}
	return strings.HasPrefix(l, "•") || strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")
}

// Sanitizer — oqim uchun: bo'laklar ("delta") istalgan joyda bo'linishi mumkin (" —" + " keyin", "~" + "5 000").
// Oxiridagi bo'shliq va hal qilinmagan belgilar keyingi bo'lakkacha ushlab turiladi; chiqarilgan matn doim
// Sanitize(to'liq matn) ning boshi bo'ladi.
type Sanitizer struct {
	lang    string
	raw     strings.Builder
	emitted int // Sanitize natijasidan chiqarilgan baytlar
}

func NewSanitizer(lang string) *Sanitizer { return &Sanitizer{lang: lang} }

// Push — yangi bo'lak; xavfsiz chiqariladigan qismni qaytaradi (bo'sh bo'lishi mumkin).
func (s *Sanitizer) Push(delta string) string {
	s.raw.WriteString(delta)
	raw := s.raw.String()
	cut := len(raw)
	for cut > 0 {
		r, size := utf8.DecodeLastRuneInString(raw[:cut])
		if !unicode.IsSpace(r) && !special(r) {
			break
		}
		cut -= size
	}
	return s.take(Sanitize(raw[:cut], s.lang))
}

// Flush — oqim tugadi: qolgan hamma narsa.
func (s *Sanitizer) Flush() string { return s.take(Sanitize(s.raw.String(), s.lang)) }

func (s *Sanitizer) take(clean string) string {
	if len(clean) <= s.emitted {
		return ""
	}
	out := clean[s.emitted:]
	s.emitted = len(clean)
	return out
}

// SanitizeEmit — "delta" hodisalarini Sanitizer orqali o'tkazadi. flush oqim oxirida chaqiriladi.
func SanitizeEmit(lang string, emit Emit) (wrapped Emit, flush func()) {
	s := NewSanitizer(lang)
	wrapped = func(event string, data any) {
		if event != "delta" {
			emit(event, data)
			return
		}
		var text string
		switch d := data.(type) {
		case map[string]string:
			text = d["text"]
		case string:
			text = d
		default:
			emit(event, data)
			return
		}
		if out := s.Push(text); out != "" {
			emit("delta", map[string]string{"text": out})
		}
	}
	flush = func() {
		if out := s.Flush(); out != "" {
			emit("delta", map[string]string{"text": out})
		}
	}
	return wrapped, flush
}
