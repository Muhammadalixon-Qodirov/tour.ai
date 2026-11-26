package pricing

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestCompute(t *testing.T) {
	// 7 ta mos narx + bitta aksessuar narxi (chetga chiqadi) + bitta juda qimmat
	st, kept := Compute([]int64{5_000_000, 5_200_000, 4_900_000, 5_500_000, 6_000_000, 5_100_000, 4_800_000, 50_000, 30_000_000})
	if st.Count != 7 || len(kept) != 7 {
		t.Fatalf("chetga chiqqanlar olinmadi: %+v", st)
	}
	if st.Average < 5_000_000 || st.Average > 5_300_000 || st.Min < 4_800_000 || st.Max > 6_000_000 || st.Min > st.Max {
		t.Fatalf("statistika: %+v", st)
	}
	if st.Average%1000 != 0 {
		t.Fatalf("1000 ga yaxlitlanmagan: %d", st.Average)
	}
	// kam namuna — min..max
	st, _ = Compute([]int64{300_000, 500_000})
	if st.Min != 300_000 || st.Max != 500_000 || st.Average != 400_000 {
		t.Fatalf("kam namuna: %+v", st)
	}
	if st, _ := Compute(nil); st.Count != 0 {
		t.Fatal("bo'sh")
	}
}

func TestHeuristicRelevant(t *testing.T) {
	ls := []Listing{{Title: "Apple iPhone 13 128GB Midnight"}, {Title: "iPhone 13 uchun chexol"}, {Title: "Samsung Galaxy A54"},
		{Title: "Himoya oynasi iPhone 13"}, {Title: "IPHONE 13 256gb"}}
	keep := heuristicRelevant("iPhone 13", ls)
	want := []bool{true, false, false, false, true}
	for i := range want {
		if keep[i] != want[i] {
			t.Errorf("%q: %v", ls[i].Title, keep[i])
		}
	}
}

func TestShrink(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2000, 1000))
	for x := 0; x < 2000; x += 10 {
		src.Set(x, 500, color.RGBA{255, 0, 0, 255})
	}
	var buf bytes.Buffer
	png.Encode(&buf, src)
	out, mime := shrink(buf.Bytes(), "image/png")
	if mime != "image/jpeg" {
		t.Fatalf("mime: %s", mime)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil || img.Bounds().Dx() != 768 || img.Bounds().Dy() != 384 {
		t.Fatalf("o'lcham: %v %v", img.Bounds(), err)
	}
	// rasm emas — asl holicha
	if b, m := shrink([]byte("not an image"), "image/jpeg"); string(b) != "not an image" || m != "image/jpeg" {
		t.Fatal("dekod bo'lmasa asl qaytishi kerak")
	}
}

func TestSearcherModes(t *testing.T) {
	if s := NewSearcher("", ""); s.fallback != nil {
		t.Fatal("to'g'ridan-to'g'ri — fallback bo'lmasligi kerak")
	}
	if s := NewSearcher("http://10.0.0.5:3128", ""); s.fallback == nil {
		t.Fatal("proksi — fallback bo'lishi kerak")
	}
	if s := NewSearcher("", "http://market-fetch:8890"); s.fallback == nil {
		t.Fatal("fetch — fallback bo'lishi kerak")
	}
}
