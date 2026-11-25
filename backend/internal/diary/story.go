// Package diary — sayohat kundaligi: kun hikoyasi va ulashish kartasi.
//
// Hikoya faqat berilgan faktlardan yoziladi (borilgan joylar, tasdiqlangan tarixiy faktlar, yurilgan masofa).
// Hozir — shablon; 7-bosqichda AI yozuvchi shu Writer interfeysiga ulanadi va natijasi CheckGrounded bilan tekshiriladi.
package diary

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
)

type StopFact struct {
	PlaceID  string
	Name     string
	Category string
	Novel    bool   // yangi yoki kam tanilgan (hamkor emas)
	Fact     string // tasdiqlangan fakt (bo'lsa)
}

type StoryInput struct {
	Lang      string // uz | ru | en
	Tone      string // warm | short | fun | poetic
	City      string
	CityID    string
	Date      time.Time
	Stops     []StopFact // borilgan tartibda
	WalkingM  int
	Photos    int
	Interests []string
}

type Story struct {
	Title      string
	Body       string
	Caption    string
	Highlights []string
	Hashtags   []string
	PlaceIDs   []string
	By         string // template | ai
}

type Writer interface {
	Write(ctx context.Context, in StoryInput) (Story, error)
}

var Tones = []string{"warm", "short", "fun", "poetic"}

// Template — AI siz ishlaydigan yozuvchi. Har doim natija beradi.
type Template struct{}

type phrases struct {
	titleNovel, titlePlain, intro, first, then, novel, fact, walk string
	close                                                         map[string]string
	and                                                           string
}

var tpl = map[string]phrases{
	"uz": {
		titleNovel: "%s: %s va boshqa kashfiyotlar", titlePlain: "%sdagi bir kun",
		intro: "Bugun %s bo'ylab %d ta joyni ko'rdim.", first: "Kun %s bilan boshlandi.", then: "Keyin: %s.",
		novel: "Eng esda qolgani — hali kam odam biladigan joy: %s.", fact: "Bilasizmi? %s",
		walk: "Piyoda %s km yurdim — charchadim, lekin bunga arziydi.", and: " va ",
		close: map[string]string{
			"warm": "Ertaga yana yangi kashfiyotlar!", "short": "Ajoyib kun bo'ldi.",
			"fun": "Oyoqlarim charchadi, kayfiyatim esa a'lo! 😄", "poetic": "Kun botdi, gumbazlar esa hamon yorug' — %s meni yana chaqiradi.",
		},
	},
	"ru": {
		titleNovel: "%s: %s и другие открытия", titlePlain: "Один день: %s",
		intro: "Сегодня в городе %s я посетил(а) %d мест.", first: "День начался с места «%s».", then: "Потом: %s.",
		novel: "Больше всего запомнилось место, о котором мало кто знает: %s.", fact: "Знаете ли вы? %s",
		walk: "Прошёл(ла) пешком %s км — устал(а), но оно того стоило.", and: " и ",
		close: map[string]string{
			"warm": "Завтра — новые открытия!", "short": "Отличный день.",
			"fun": "Ноги устали, а настроение — супер! 😄", "poetic": "Солнце село, а купола всё ещё светятся — %s зовёт вернуться.",
		},
	},
	"en": {
		titleNovel: "%s: %s and other discoveries", titlePlain: "A day in %s",
		intro: "Today I explored %d places in %s.", first: "The day began at %s.", then: "Then: %s.",
		novel: "The highlight was a place few people know yet: %s.", fact: "Did you know? %s",
		walk: "I walked %s km — tired, but it was worth it.", and: " and ",
		close: map[string]string{
			"warm": "More discoveries tomorrow!", "short": "A great day.",
			"fun": "Tired feet, happy heart! 😄", "poetic": "The sun has set, yet the domes still glow — %s is calling me back.",
		},
	},
}

func (Template) Write(_ context.Context, in StoryInput) (Story, error) {
	ph, ok := tpl[in.Lang]
	if !ok {
		ph = tpl["en"]
	}
	tone := in.Tone
	if _, ok := ph.close[tone]; !ok {
		tone = "warm"
	}
	if len(in.Stops) == 0 {
		return Story{}, fmt.Errorf("hikoya uchun joy yo'q")
	}
	var novel *StopFact
	for i := range in.Stops {
		if in.Stops[i].Novel {
			novel = &in.Stops[i]
			break
		}
	}
	st := Story{By: "template"}
	if novel != nil {
		st.Title = fmt.Sprintf(ph.titleNovel, in.City, novel.Name)
	} else {
		st.Title = fmt.Sprintf(ph.titlePlain, in.City)
	}

	var s []string
	if in.Lang == "en" {
		s = append(s, fmt.Sprintf(ph.intro, len(in.Stops), in.City))
	} else {
		s = append(s, fmt.Sprintf(ph.intro, in.City, len(in.Stops)))
	}
	if tone != "short" {
		s = append(s, fmt.Sprintf(ph.first, in.Stops[0].Name))
		if len(in.Stops) > 1 {
			var rest []string
			for _, x := range in.Stops[1:] {
				rest = append(rest, x.Name)
			}
			s = append(s, fmt.Sprintf(ph.then, joinList(rest, ph.and)))
		}
	}
	if novel != nil {
		s = append(s, fmt.Sprintf(ph.novel, novel.Name))
	}
	if tone != "short" {
		facts := 0
		for _, x := range in.Stops {
			if x.Fact != "" && facts < 2 {
				s = append(s, fmt.Sprintf(ph.fact, x.Fact))
				facts++
			}
		}
		if in.WalkingM >= 500 {
			s = append(s, fmt.Sprintf(ph.walk, km(in.WalkingM)))
		}
	}
	closing := ph.close[tone]
	if strings.Contains(closing, "%s") {
		closing = fmt.Sprintf(closing, in.City)
	}
	s = append(s, closing)
	st.Body = strings.Join(s, " ")

	if novel != nil {
		st.Highlights = append(st.Highlights, novel.Name)
	}
	for _, x := range in.Stops {
		if len(st.Highlights) >= 3 {
			break
		}
		if novel == nil || x.PlaceID != novel.PlaceID {
			st.Highlights = append(st.Highlights, x.Name)
		}
	}
	for _, x := range in.Stops {
		st.PlaceIDs = append(st.PlaceIDs, x.PlaceID)
	}
	st.Hashtags = Hashtags(in)
	st.Caption = Caption(st)
	return st, nil
}

func joinList(items []string, and string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + and + items[len(items)-1]
}

func km(m int) string {
	s := fmt.Sprintf("%.1f", float64(m)/1000)
	return strings.TrimSuffix(s, ".0")
}

var interestTags = map[string]map[string]string{
	"history":      {"uz": "Tarix", "ru": "История", "en": "History"},
	"food":         {"uz": "MilliyTaomlar", "ru": "Еда", "en": "Foodie"},
	"nature":       {"uz": "Tabiat", "ru": "Природа", "en": "Nature"},
	"craft":        {"uz": "Hunarmandchilik", "ru": "Ремёсла", "en": "Crafts"},
	"art":          {"uz": "Sanat", "ru": "Искусство", "en": "Art"},
	"architecture": {"uz": "Memorchilik", "ru": "Архитектура", "en": "Architecture"},
}

// Hashtags — shahar, ilova va 1–2 qiziqish (≤ 5 ta, bo'shliq va tutuq belgisisiz).
func Hashtags(in StoryInput) []string {
	tags := []string{"#" + tagWord(in.City), "#YoldoshAI", "#Uzbekistan"}
	for _, i := range in.Interests {
		if t, ok := interestTags[i][in.Lang]; ok && len(tags) < 5 {
			tags = append(tags, "#"+t)
		}
	}
	return tags
}

func tagWord(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Caption — Instagram/Telegram uchun qisqa matn (≤ 300 belgi).
func Caption(st Story) string {
	c := st.Title
	if len(st.Highlights) > 0 {
		c += "\n📍 " + strings.Join(st.Highlights, " · ")
	}
	c += "\n" + strings.Join(st.Hashtags, " ")
	if r := []rune(c); len(r) > 300 {
		c = string(r[:297]) + "..."
	}
	return c
}

// CheckGrounded — AI hikoyasi faqat berilgan joylarni tilga olganini tekshirish uchun:
// highlights dagi har bir nom kirish ro'yxatida bo'lishi shart.
func CheckGrounded(st Story, in StoryInput) error {
	names := map[string]bool{}
	for _, x := range in.Stops {
		names[x.Name] = true
	}
	for _, h := range st.Highlights {
		if !names[h] {
			return fmt.Errorf("noma'lum joy: %q", h)
		}
	}
	return nil
}
