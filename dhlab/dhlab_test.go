package dhlab

import (
	"slices"
	"testing"
)

func TestDhlabIDs(t *testing.T) {
	meta := make(map[int]CorpusMetadata)
	meta[0] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn0", Year: 1990}
	meta[1] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn1", Year: 1991}
	meta[2] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn2", Year: 1992}
	meta[3] = CorpusMetadata{Doctype: "digibok", Lang: "nob", URN: "urn3", Year: 1993}
	corp := &Corpus{DHLabID: meta}

	want := make([]int, 0)
	want = append(want, 0)
	want = append(want, 1)
	want = append(want, 2)
	want = append(want, 3)

	ids := DhlabIDs(corp)
	if !slices.Equal(ids, want) {
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
