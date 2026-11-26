package speech

import "testing"

func TestCyrToLat(t *testing.T) {
	cases := map[string]string{
		"Маслахат лешмух.":     "Maslaxat leshmux.",
		"Ўзбекистон":           "O'zbekiston",
		"Қаерда ғишт ҳовуз":    "Qayerda g'isht hovuz",
		"Ер, ел, бер":          "Yer, yel, ber",
		"Samarqand lotin matn": "Samarqand lotin matn",
		"Шаҳар маркази":        "Shahar markazi",
	}
	for in, want := range cases {
		if got := CyrToLat(in); got != want {
			t.Errorf("%q -> %q (kutilgan %q)", in, got, want)
		}
	}
}
