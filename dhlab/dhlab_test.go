package dhlab

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/quaepoena/leksemvariasjon/config"
)

func TestCorpRequest(t *testing.T) {
	a := config.Args{
		ConfigFile: "/dev/null",
		Directory:  "/path/to/dev/null",
		Doctype:    "digibok",
		Src:        "/path/to/src",
		CorpLimit:  100,
		ConcLimit:  500,
		From:       1990,
		To:         1995,
	}

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

	l1 := config.Lemma{Lemma: "leggje",
		Words: []config.Word{
			config.Word{Form: f1, Value: v1, Morphology: m1},
			config.Word{Form: f2, Value: v2, Morphology: m2},
			config.Word{Form: f3, Value: v3, Morphology: m3},
		},
	}
	l2 := config.Lemma{Lemma: "legge",
		Words: []config.Word{
			config.Word{Form: f4, Value: v4, Morphology: m4},
			config.Word{Form: f5, Value: v5, Morphology: m5},
			config.Word{Form: f6, Value: v6, Morphology: m6},
		},
	}

	c := &config.Conf{
		Attribute: "palatalisering",
		Language:  "nno",
		Lemmas:    []config.Lemma{l1, l2},
	}

	wantByte := []byte(`{
"doctype": "digibok",
"from_year": 1990,
"to_year": 1996,
"fulltext": "leggja OR leggje OR leggjer OR legga OR legge OR legger",
"lang": "nno",
"limit": 100
}`)

	want := &bytes.Buffer{}
	err := json.Compact(want, wantByte)
	if err != nil {
		t.Errorf("json.Compact() failed on wantByte: %v", wantByte)
	}

	req, err := CorpRequest(&a, c)
	if !slices.Equal(req, want.Bytes()) {
		t.Errorf("CorpRequest(): %v != %v\nerr: %v", req, want, err)
	}

	if err != nil {
		t.Errorf("err in CorpRequest():\nerr: %v\nreq: %v\nwant: %v", err, req, want)
	}
}

func TestCorpRequestEmptyDoctype(t *testing.T) {
	a := config.Args{
		ConfigFile: "/dev/null",
		Directory:  "/path/to/dev/null",
		Src:        "/path/to/src",
		CorpLimit:  100,
		ConcLimit:  500,
		From:       1990,
		To:         1995,
	}

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

	l1 := config.Lemma{Lemma: "leggje",
		Words: []config.Word{
			config.Word{Form: f1, Value: v1, Morphology: m1},
			config.Word{Form: f2, Value: v2, Morphology: m2},
			config.Word{Form: f3, Value: v3, Morphology: m3},
		},
	}
	l2 := config.Lemma{Lemma: "legge",
		Words: []config.Word{
			config.Word{Form: f4, Value: v4, Morphology: m4},
			config.Word{Form: f5, Value: v5, Morphology: m5},
			config.Word{Form: f6, Value: v6, Morphology: m6},
		},
	}

	c := &config.Conf{
		Attribute: "palatalisering",
		Language:  "nno",
		Lemmas:    []config.Lemma{l1, l2},
	}

	wantByte := []byte(`{
"doctype": "",
"from_year": 1990,
"to_year": 1996,
"fulltext": "leggja OR leggje OR leggjer OR legga OR legge OR legger",
"lang": "nno",
"limit": 100
}`)

	want := &bytes.Buffer{}
	err := json.Compact(want, wantByte)
	if err != nil {
		t.Errorf("json.Compact() failed on wantByte: %v", wantByte)
	}

	req, err := CorpRequest(&a, c)
	if !slices.Equal(req, want.Bytes()) {
		t.Errorf("CorpRequest(): %v != %v\nerr: %v", req, want, err)
	}

	if err != nil {
		t.Errorf("err in CorpRequest():\nerr: %v\nreq: %v\nwant: %v", err, req, want)
	}
}

func TestConfRequest(t *testing.T) {
	// få dette frå dhlab swagger
}

func TestConfResponse(t *testing.T) {
	// få dette frå dhlab swagger
}

func TestDhlabIDs(t *testing.T) {
	meta := make(map[int]CorpusMetadata)
	meta[0] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn0", Year: 1990}
	meta[1] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn1", Year: 1991}
	meta[2] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn2", Year: 1992}
	meta[3] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn3", Year: 1993}
	corp := &Corpus{DHLabID: meta}

	want := []int{0, 1, 2, 3}

	ids := DhlabIDs(corp)

	slices.Sort(ids)
	slices.Sort(want)
	if !cmp.Equal(ids, want) {
		t.Errorf("DHLabIDs(): %v != %v", ids, want)
	}
}

func TestDhlabIDsEmptyCorp(t *testing.T) {
	meta := make(map[int]CorpusMetadata)
	corp := &Corpus{DHLabID: meta}

	want := make([]int, 0)

	ids := DhlabIDs(corp)
	if !slices.Equal(ids, want) {
		t.Errorf("DHLabIDs(): %v != %v", ids, want)
	}
}

func TestCorp(t *testing.T) {
	corpResp := &CorpusResponse{}
	c := &Corpus{}

	err := config.Unmarshal("testdata/build_corpus_response.json", corpResp)
	if err != nil {
		t.Errorf("error in config.Marshal() with 'testdata/build_corpus_response.json': %v", err)
	}

	err = Corp(corpResp, c)
	if err != nil {
		t.Errorf("error in Corp() with %v: %v", corpResp, err)
	}

	d1 := CorpusMetadata{
		Doctype: "digibok",
		URN: "URN:100",
		Year: 1991,
		Lang: "nob",
	}
	d2 := CorpusMetadata{
		Doctype: "digibok",
		URN: "URN:200",
		Year: 1993,
		Lang: "mul / nob / swe / eng",
	}
	d3 := CorpusMetadata{
		Doctype: "digibok",
		URN: "URN:300",
		Year: 1994,
		Lang: "nob / swe",
	}
	d4 := CorpusMetadata{
		Doctype: "digibok",
		URN: "URN:400",
		Year: 1995,
		Lang: "nob",
	}
	d5 := CorpusMetadata{
		Doctype: "digibok",
		URN: "URN:500",
		Year: 1992,
		Lang: "nob",
	}

	want := &Corpus{}
	want.DHLabID = make(map[int]CorpusMetadata)
	want.DHLabID[10] = d1
	want.DHLabID[25] = d2
	want.DHLabID[40] = d3
	want.DHLabID[55] = d4
	want.DHLabID[70] = d5

	if !cmp.Equal(c, want) {
		t.Errorf("error in Corp(): %v != %v", c, want)
	}
}
