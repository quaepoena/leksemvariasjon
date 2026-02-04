const (
    DHLabAPI = "https://api.nb.no/dhlab/"
)

type Corpus struct {
    DHLabID map[string]int
    Doctype map[string]string
    Langs   map[string]string
    Title   map[string]string
    URN     map[string]string
    Year    map[string]int
}

func CorpusRequest(a *Args, c *Conf) ([]byte, error) {
    type CorpusRequest struct {
        Doctype string `json:"doctype"`
        FromYear int `json:"from_year"`
        ToYear int `json:"to_year"`
        Fulltext string `json:"fulltext"`
        Lang string `json:"lang"`
        Limit int `json:"limit"`
    }

    var req CorpusRequest
    var words []string
    var reqData []byte

    for _, lemma := range c.Lemmas {
        for _, word := range lemma.Words {
            words = append(words, word.Form)
        }
    }

    req.Doctype = a.Doctype
    req.FromYear = a.From
    req.ToYear = a.To + 1 // "to_year" on the server side is exclusive.
    req.Limit = 10
    req.Fulltext = strings.Join(words, " OR ")
    req.Lang = c.Language

    reqData, err := json.Marshal(req)
    if err != nil {
        return nil, errors.New(fmt.Sprintf("Error on json.Marshal(): %+v\n", err))
    }

    return reqData, nil
}

func PopulateCorpusRecord(s string, c *Corpus) []string {
    var fields []string

    fields = append(fields, strconv.Itoa(c.DHLabID[s]))
    fields = append(fields, c.Doctype[s])
    fields = append(fields, c.Langs[s])
    fields = append(fields, c.Title[s])
    fields = append(fields, c.URN[s])
    fields = append(fields, strconv.Itoa(c.Year[s]))

    return fields
}

func ConcRequest(a *Args, c *Conf, ids []int) ([]byte, error) {
    type ConcRequest struct {
        DHLabIDs []int `json:"dhlabids"`
        Limit int `json:"limit"`
        Query string `json:"query"`
        Window int `json:"window"`
        HTMLFormatting bool `json:"html_formatting"`
    }

    var req ConcRequest
    var words []string
    var reqData []byte

    for _, lemma := range c.Lemmas {
        for _, word := range lemma.Words {
            words = append(words, word.Form)
        }
    }

    req.DHLabIDs = ids
    req.Limit = 100
    req.Query = strings.Join(words, " OR ")
    req.Window = 25
    req.HTMLFormatting = false

    reqData, err := json.Marshal(req)
    if err != nil {
        return nil, errors.New(fmt.Sprintf("Error on json.Marshal(): %+v\n", err))
    }

    return reqData, nil
}
