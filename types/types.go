package types

type Word struct {
    Form, Value string
    Morphology []string
}

type Lemma struct {
    Lemma string
    Words []Word
}

type Conf struct {
    Attribute, Language string
    Lemmas []Lemma
}

type Args struct {
    Config, Directory, Doctype string
    From, To int
}

type Concordance struct {
    DocID map[string]int
    URN map[string]string
    Conc map[string]string
}

type Corpus struct {
    DHLabID map[string]int
    Doctype map[string]string
    Langs   map[string]string
    Title   map[string]string
    URN     map[string]string
    Year    map[string]int
}

type CorpusRequest struct {
    Doctype string `json:"doctype"`
    FromYear int `json:"from_year"`
    ToYear int `json:"to_year"`
    Fulltext string `json:"fulltext"`
    Lang string `json:"lang"`
    Limit int `json:"limit"`
}

type ConcRequest struct {
    DHLabIDs []int `json:"dhlabids"`
    HTMLFormatting bool `json:"html_formatting"`
    Limit int `json:"limit"`
    Query string `json:"query"`
    Window int `json:"window"`
}
