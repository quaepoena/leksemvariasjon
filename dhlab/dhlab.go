package dhlab

import (
    "github.com/quaepoena/leksemvariasjon/types"
)

const (
    DHLabAPI = "https://api.nb.no/dhlab/"
)

func BuildCorpus(a *types.Args, c *types.Conf) ([]byte, error) {
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
        return []byte, errors.New(fmt.Sprintf("Error on json.Marshal(): %+v\n", err))
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

func BuildConc(a *types.Args, c *types.Conf, ids []int) ([]byte, error) {
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
