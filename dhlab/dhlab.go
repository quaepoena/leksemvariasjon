package dhlab

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "strings"

    "github.com/quaepoena/leksemvariasjon/types"
)

const (
    DHLabAPI = "https://api.nb.no/dhlab/"
)

func BuildCorpusRequest(a *types.Args, c *types.Conf) ([]byte, error) {
    var req types.CorpusRequest
    var words []string
    var b []byte

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

    b, err := json.Marshal(req)
    if err != nil {
        return []byte{}, errors.New(fmt.Sprintf("Error on json.Marshal(): %+v\n", err))
    }

    return b, nil
}

func BuildCorpus(req []byte, c *types.Corpus) error {
    var uri = DHLabAPI + "build_corpus"

    resp, err := http.Post(uri, "application/json", bytes.NewReader(req))
    if err != nil {
        return errors.New(fmt.Sprintf("Error in http.Post(): %v\n", err))
    }
    defer resp.Body.Close()

    b, err := io.ReadAll(resp.Body)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in io.ReadAll(): %v\n", err))
    }

    err = json.Unmarshal(b, &c)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in json.Unmarshal(): %v\n", err))
    }

    return nil
}

func PopulateCorpusRecord(s string, c *types.Corpus) []string {
    var fields []string

    fields = append(fields, strconv.Itoa(c.DHLabID[s]))
    fields = append(fields, c.Doctype[s])
    fields = append(fields, c.Langs[s])
    fields = append(fields, c.Title[s])
    fields = append(fields, c.URN[s])
    fields = append(fields, strconv.Itoa(c.Year[s]))

    return fields
}

func BuildConcRequest(a *types.Args, c *types.Conf, ids []int) ([]byte, error) {
    var req types.ConcRequest
    var words []string
    var b []byte

    for _, lemma := range c.Lemmas {
        for _, word := range lemma.Words {
            words = append(words, word.Form)
        }
    }

    req.DHLabIDs = ids
    req.HTMLFormatting = false
    req.Limit = 10
    req.Query = strings.Join(words, " OR ")
    req.Window = 25

    b, err := json.Marshal(req)
    if err != nil {
        return []byte{}, errors.New(fmt.Sprintf("Error on json.Marshal(): %+v\n", err))
    }

    return b, nil
}

func BuildConc(req []byte, c *types.Concordance) error {
    var uri = DHLabAPI + "conc"

    resp, err := http.Post(uri, "application/json", bytes.NewReader(req))
    if err != nil {
        return errors.New(fmt.Sprintf("Error in http.Post(): %v\n", err))
    }
    defer resp.Body.Close()

    b, err := io.ReadAll(resp.Body)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in io.ReadAll(): %v\n", err))
    }

    err = json.Unmarshal(b, &c)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in json.Unmarshal(): %v\n", err))
    }

    return nil
}
