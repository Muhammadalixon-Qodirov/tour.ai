package egov

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"

	"yoldosh/internal/discover"
)

// HotelsDataset — "Sertifikatga ega mehmonxonalar ro‘yxati" (Turizm sohasi, ~1143 obyekt).
// Ustunlar: viloyat, tuman, name, type, address, manzil, tel, email, regnum, stir, koyka, data, valid, yur.
const HotelsDataset = "610b6fe11a64fdd0373a8eaf"

type HotelOptions struct {
	Limit   int  // 0 — hammasi
	DryRun  bool // ma'lumot bazaga yozilmaydi
	Trusted bool // true bo'lsa yuqori ishonchda avtomatik chop etilishi mumkin (default: admin navbatiga)
}

type HotelReport struct {
	Rows     int             `json:"rows"`
	Geocoded int             `json:"geocoded"`
	Approx   int             `json:"approxGeocode"` // faqat tuman/viloyat markazi topildi
	NoGeo    int             `json:"noGeocode"`
	Ingest   discover.Report `json:"ingest"`
}

// ImportHotels — sertifikatlangan mehmonxonalarni oladi, manzilini geokodlaydi va discover navbatiga joylaydi.
func ImportHotels(ctx context.Context, db *pgxpool.Pool, cl *Client, gc *Geocoder, o HotelOptions, log *slog.Logger) (HotelReport, error) {
	founds, rep, err := CollectHotels(ctx, cl, gc, o, log)
	if err != nil && len(founds) == 0 {
		return rep, err
	}
	ing, ierr := discover.Ingest(ctx, db, "egov", founds, o.DryRun)
	rep.Ingest = ing
	if ierr != nil {
		return rep, ierr
	}
	return rep, err
}

// IngestFounds — oldindan geokodlangan Found ro'yxatini (export faylidan) discover navbatiga joylaydi.
// Tashqi tarmoq talab qilmaydi — foreign-IP bloklangan serverda ishlaydi.
func IngestFounds(ctx context.Context, db *pgxpool.Pool, founds []discover.Found, dryRun bool) (discover.Report, error) {
	return discover.Ingest(ctx, db, "egov", founds, dryRun)
}

// CollectHotels — mehmonxonalarni data.egov.uz dan oladi va Nominatim bilan geokodlaydi (DB kerak emas).
// Natijani faylga saqlab, bloklangan serverda IngestFounds bilan joylash mumkin.
func CollectHotels(ctx context.Context, cl *Client, gc *Geocoder, o HotelOptions, log *slog.Logger) ([]discover.Found, HotelReport, error) {
	var rep HotelReport
	rows, err := cl.AllRows(ctx, HotelsDataset, 200)
	if err != nil && len(rows) == 0 {
		return nil, rep, fmt.Errorf("mehmonxonalarni olish: %w", err)
	}
	if o.Limit > 0 && len(rows) > o.Limit {
		rows = rows[:o.Limit]
	}
	rep.Rows = len(rows)

	var founds []discover.Found
	for i, r := range rows {
		if ctx.Err() != nil {
			return founds, rep, ctx.Err()
		}
		name := strings.TrimSpace(r.Get("name"))
		if name == "" {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(r.Get("type")))
		region := strings.TrimSpace(r.Get("viloyat"))
		district := strings.TrimSpace(r.Get("tuman"))
		address := firstNonEmpty(r.Get("address"), r.Get("manzil"))
		display := displayName(name, typ)

		m, err := gc.GeocodeAddress(ctx, address, district, region)
		if err != nil {
			if log != nil {
				log.Warn("geokod xato", "mehmonxona", display, "err", err)
			}
			continue
		}
		if m == nil {
			rep.NoGeo++
			if log != nil {
				log.Debug("koordinata topilmadi", "mehmonxona", display, "manzil", address)
			}
			continue
		}
		if m.Precise {
			rep.Geocoded++
		} else {
			rep.Approx++
		}

		ref := firstNonEmpty(r.Get("regnum"), r.Get("stir"), fmt.Sprintf("%s-%d", HotelsDataset, i))
		data := map[string]any{
			"address":    address,
			"region":     region,
			"district":   district,
			"phone":      strings.TrimSpace(r.Get("tel")),
			"email":      strings.TrimSpace(r.Get("email")),
			"regnum":     strings.TrimSpace(r.Get("regnum")),
			"beds":       strings.TrimSpace(r.Get("koyka")),
			"approxGeo":  !m.Precise,
			"sourceName": "data.egov.uz — Sertifikatlangan mehmonxonalar",
			"sourceUrl":  "https://data.egov.uz/eng/dataset/" + HotelsDataset,
		}
		names := map[string]string{"uz": display}
		founds = append(founds, discover.Found{
			Source: "egov",
			Ref:    "hotel-" + ref,
			Names:  names,
			Loc:    m.Loc,
			Signals: discover.Signals{
				Names: names,
				Hint:  "hotel",
				Text:  typ + " " + address,
			},
			Desc:          map[string]string{"uz": shortDesc(display, region, district)},
			Data:          data,
			ForceCategory: "hotel",   // rasmiy sertifikatlangan mehmonxona — kategoriya aniq
			Trusted:       o.Trusted, // --trusted/--publish bilan avtomatik chop etiladi
		})
	}

	if log != nil {
		log.Info("mehmonxonalar geokodlandi", "jami", rep.Rows, "aniq", rep.Geocoded, "taxminiy", rep.Approx, "topilmadi", rep.NoGeo)
	}
	return founds, rep, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// displayName — "ANDIJON" + "mehmonxonasi" -> "Andijon mehmonxonasi". Tur nomi allaqachon bo'lsa qo'shilmaydi.
func displayName(name, typ string) string {
	n := titleCase(name)
	if typ == "" {
		return n
	}
	if strings.Contains(strings.ToLower(n), strings.ToLower(typ)) {
		return n
	}
	return n + " " + typ
}

func shortDesc(name, region, district string) string {
	loc := firstNonEmpty(district, region)
	if loc == "" {
		return "Sertifikatlangan mehmonxona (data.egov.uz)."
	}
	return fmt.Sprintf("%s hududidagi sertifikatlangan mehmonxona (data.egov.uz).", loc)
}

// titleCase — har bir so'zning birinchi harfini katta, qolganini kichik qiladi (lotin/kirill).
func titleCase(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			if !prevLetter {
				b.WriteRune(unicode.ToUpper(r))
			} else {
				b.WriteRune(r)
			}
			prevLetter = true
		} else {
			b.WriteRune(r)
			prevLetter = false
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
