package domain

// MinNoveltyShare — butun marshrutda yangi/kam tanilgan joylarning eng kam ulushi.
const MinNoveltyShare = 0.25

type StopKind string

const (
	KindPlace StopKind = "place"
	KindMeal  StopKind = "meal"
)

// PlanStop — rejalashtirilgan to'xtash (saqlash va tekshiruv uchun yetarli ma'lumot).
type PlanStop struct {
	Kind        StopKind
	PlaceID     string
	Novelty     Novelty
	IsPartner   bool
	ArriveMin   int // kun boshidan daqiqa
	DurationMin int
	Transit     *Transit
	ReasonCode  string
	ReasonArg   string
	ReasonText  string // AI matni (bo'lsa)
}

func (s PlanStop) EndMin() int { return s.ArriveMin + s.DurationMin }

// CountsForNovelty — kvotani to'ldiradimi. Hamkor joy kvotaga KIRMAYDI (shaffoflik tamoyili).
func (s PlanStop) CountsForNovelty() bool {
	return s.Kind == KindPlace && s.Novelty.IsNovel() && !s.IsPartner
}

type Transit struct {
	Mode        string `json:"mode"` // walk | taxi
	DurationMin int    `json:"durationMin"`
	DistanceM   int    `json:"distanceM"`
	Fare        *Money `json:"fare,omitempty"`
}

type PlanDay struct {
	Index int
	Stops []PlanStop
}

type Violation struct {
	Day  int    // -1 = butun marshrut
	Code string // day_without_novelty | novelty_share_low | overlap | empty_reason
}

// Validate — marshrut invariantlari. AI javobiga ishonmasdan har doim chaqiriladi.
func Validate(days []PlanDay) []Violation {
	var v []Violation
	for _, d := range days {
		dayNovel := 0
		for i, s := range d.Stops {
			if s.ReasonCode == "" && s.ReasonText == "" {
				v = append(v, Violation{d.Index, "empty_reason"})
			}
			if i > 0 {
				prev := d.Stops[i-1]
				transit := 0
				if s.Transit != nil {
					transit = s.Transit.DurationMin
				}
				if s.ArriveMin < prev.EndMin()+transit {
					v = append(v, Violation{d.Index, "overlap"})
				}
			}
			if s.CountsForNovelty() {
				dayNovel++
			}
		}
		if dayNovel == 0 {
			v = append(v, Violation{d.Index, "day_without_novelty"})
		}
	}
	if share, total := NoveltyShare(days); total > 0 && share < MinNoveltyShare {
		v = append(v, Violation{-1, "novelty_share_low"})
	}
	return v
}

// NoveltyShare — hamkor bo'lmagan new|hidden / barcha `place` to'xtashlari.
func NoveltyShare(days []PlanDay) (share float64, places int) {
	novel := 0
	for _, d := range days {
		for _, s := range d.Stops {
			if s.Kind != KindPlace {
				continue
			}
			places++
			if s.CountsForNovelty() {
				novel++
			}
		}
	}
	if places == 0 {
		return 0, 0
	}
	return float64(novel) / float64(places), places
}
