package config

import (
	"slices"
    "testing"
)

func TestWords(t *testing.T) {
	m1 := []string{"verb", "inf"}
	m2 := []string{"verb", "inf"}
	m3 := []string{"verb", "pres"}
	m4 := []string{"verb", "inf"}
	m5 := []string{"verb", "inf"}
	m6 := []string{"verb", "pres"}

	f1 := "leggja"
	f2 := "leggje"
	f3 := "leggjer"
	f4 := "legga"
	f5 := "legge"
	f6 := "legger"

	v1 := "med"
	v2 := "med"
	v3 := "med"
	v4 := "utan"
	v5 := "utan"
	v6 := "utan"

	l1 := Lemma{Lemma: "leggje",
		Words: []Word{
			Word{Form: f1, Value: v1, Morphology: m1},
			Word{Form: f2, Value: v2, Morphology: m2},
			Word{Form: f3, Value: v3, Morphology: m3},
		},
	}
	l2 := Lemma{Lemma: "legge",
		Words: []Word{
			Word{Form: f4, Value: v4, Morphology: m4},
			Word{Form: f5, Value: v5, Morphology: m5},
			Word{Form: f6, Value: v6, Morphology: m6},
		},
	}

	c := &Conf{
		Attribute: "palatalisering",
		Language:  "nno",
		Lemmas:    []Lemma{l1, l2},
	}

	words := Words(c)

	want := []string{
		"leggja", "leggje", "leggjer",
		"legga", "legge", "legger",
	}

	if !slices.Equal(words, want) {
		t.Errorf("Equal(): %v != %v", words, want)
	}
}
