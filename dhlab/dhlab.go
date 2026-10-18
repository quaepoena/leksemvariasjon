// Package dhlab controls the interaction with the National Library's DHLAB API.
// Specifically the build_corpus and corp endpoints are called.
package dhlab

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "path/filepath"
    "strings"

    "github.com/quaepoena/leksemvariasjon/config"
)

const (
    DHLabAPI = "https://api.nb.no/dhlab/"
)

// Struct CorpusMetadata contains metadata concerning a given DHLab object.
type CorpusMetadata struct {
    Doctype string
    Lang    string
    URN     string
    Year    int
}

// Struct Corpus maps a dhlabid to the metadata concerning it.
type Corpus struct {
    DHLabID map[int]CorpusMetadata
}

// Struct CorpusRequest contains the necessary information needed for the DHLab
// build_corpus API call.
type CorpusRequest struct {
    Doctype string `json:"doctype"`
    FromYear int `json:"from_year"`
    ToYear int `json:"to_year"`
    Fulltext string `json:"fulltext"`
    Lang string `json:"lang"`
    Limit int `json:"limit"`
}

// Struct CorpusResponse contains the information we need from the DHLab
// build_corpus API call.
type CorpusResponse struct {
    DHLabID map[string]int
    Doctype map[string]string
    Langs   map[string]string
    URN     map[string]string
    Year    map[string]int
}

// Function corpusRequest builds and returns the JSON object used in the DHLab
// build_corpus call.
func corpusRequest(a *config.Args, c *config.Conf) ([]byte, error) {
    var req CorpusRequest
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
    req.Limit = a.CorpLimit
    req.Fulltext = strings.Join(words, " OR ")
    req.Lang = c.Language

    b, err := json.Marshal(req)
    if err != nil {
        return nil, errors.New(fmt.Sprintf("error in json.Marshal(): %v", err))
    }

    return b, nil
}

// Function corpusResponse calls the build_corpus endpoint with req and
// populates c with the response.
func corpusResponse(req []byte, c *CorpusResponse) error {
    var uri = DHLabAPI + "build_corpus"

    resp, err := http.Post(uri, "application/json", bytes.NewReader(req))
    if err != nil {
        return errors.New(fmt.Sprintf("error in http.Post(): %v", err))
    }
    defer resp.Body.Close()

    b, err := io.ReadAll(resp.Body)
    if err != nil {
        return errors.New(fmt.Sprintf("error in io.ReadAll(): %v", err))
    }

    err = json.Unmarshal(b, c)
    if err != nil {
        return errors.New(fmt.Sprintf("error in json.Unmarshal(): %v", err))
    }

    return nil
}

// Function corpus populates a Corpus struct with data from the build_corpus
// API call.
func corpus(resp *CorpusResponse, c *Corpus) error {
    c.DHLabID = make(map[int]CorpusMetadata)

    for i, v := range resp.DHLabID {
        c.DHLabID[v] = CorpusMetadata{
            Doctype: resp.Doctype[i],
            Lang: resp.Langs[i],
            URN: resp.URN[i],
            Year: resp.Year[i]}
    }

    return nil
}

// Run builds a Corpus object and writes it to disk as JSON.
func (c *Corpus) Run(a *config.Args, conf *config.Conf) error {
    var req []byte
    resp := &CorpusResponse{}

    req, err := corpusRequest(a, conf)
    if err != nil {
        return errors.New(fmt.Sprintf("error in corpusRequest(): %v", err))
    }

    err = corpusResponse(req, resp)
    if err != nil {
        return errors.New(fmt.Sprintf("error in corpusResponse(): %v", err))
    }

    err = corpus(resp, c)
    if err != nil {
        return errors.New(fmt.Sprintf("error in corpus(): %v", err))
    }

    err = config.Marshal(filepath.Join(a.Directory, "corpus.json"), c)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Marshal(): %v", err))
    }

    return nil
}

func (*Corpus) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "corpus.json"))
}

// Struct ConcordanceRequest contains the necessary information for the DHLab
// conc API call.
type ConcordanceRequest struct {
    DHLabIDs []int `json:"dhlabids"`
    HTMLFormatting bool `json:"html_formatting"`
    Limit int `json:"limit"`
    Query string `json:"query"`
    Window int `json:"window"`
}

// Struct ConcordanceResponse contains the information we save from the DHLab
// conc API call.
type ConcordanceResponse struct {
    DocID map[string]int
    Conc map[string]string
}

// Struct Concordance contains concordance lines, mapped as DHLab IDs to list
// strings, i.e. concordance lines.
type Concordance struct {
    Lines map[int][]string
}

// Function dhlabIDs returns a list of IDs from a Corpus object previously saved
// to disk as JSON.
func dhlabIDs(a *config.Args) ([]int, error) {
    var ids []int
    corp := &Corpus{}

    err := config.Unmarshal(filepath.Join(a.Directory, "corpus.json"), corp)
    if err != nil {
        return nil, errors.New(fmt.Sprintf("error in config.Unmarshal(): %v", err))
    }

    for i := range corp.DHLabID {
        ids = append(ids, i)
    }

    return ids, nil
}

// Function concordanceRequest creates the JSON object needed from the conc API
// call.
func concordanceRequest(a *config.Args, c *config.Conf, ids []int) ([]byte, error) {
    var req ConcordanceRequest
    var words []string
    var b []byte

    for _, lemma := range c.Lemmas {
        for _, word := range lemma.Words {
            words = append(words, word.Form)
        }
    }

    req.DHLabIDs = ids
    req.HTMLFormatting = false
    req.Limit = a.ConcLimit
    req.Query = strings.Join(words, " OR ")
    req.Window = 25

    b, err := json.Marshal(req)
    if err != nil {
        return nil, errors.New(fmt.Sprintf("error in json.Marshal(): %v", err))
    }

    return b, nil
}

// Function concordanceResponse calls the conc API call and populates a
// ConcordanceRespone with the results.
func concordanceResponse(req []byte, concResp *ConcordanceResponse) error {
    var uri = DHLabAPI + "conc"

    resp, err := http.Post(uri, "application/json", bytes.NewReader(req))
    if err != nil {
        return errors.New(fmt.Sprintf("error in http.Post(): %v", err))
    }
    defer resp.Body.Close()

    b, err := io.ReadAll(resp.Body)
    if err != nil {
        return errors.New(fmt.Sprintf("error in io.ReadAll(): %v", err))
    }

    err = json.Unmarshal(b, concResp)
    if err != nil {
        return errors.New(fmt.Sprintf("error in json.Unmarshal(): %v", err))
    }

    return nil
}

// Function concordance populates a Concordance struct with the data from a
// ConcordanceResponse.
func concordance(concResp *ConcordanceResponse, conc *Concordance) error {
    conc.Lines = make(map[int][]string)

    for i, v := range concResp.DocID {
        conc.Lines[v] = append(conc.Lines[v], concResp.Conc[i])
    }

    return nil
}

// Run uses a list of previously fetched DHLab IDs to get concordance lines and
// writh the results to disk as JSON.
func (c *Concordance) Run(a *config.Args, conf *config.Conf) error {
    var ids []int
    resp := &ConcordanceResponse{}

    ids, err := dhlabIDs(a)
    if err != nil {
        return errors.New(fmt.Sprintf("error in dhlabIDs(): %v", err))
    }
    if ids == nil {
        return errors.New(fmt.Sprintf("no dhlabIDs were found in %s",
            filepath.Join(a.Directory, "corpus.json")))
    }

    req, err := concordanceRequest(a, conf, ids)
    if err != nil {
        return errors.New(fmt.Sprintf("error in concordanceRequest(): %v", err))
    }

    err = concordanceResponse(req, resp)
    if err != nil {
        return errors.New(fmt.Sprintf("error in concordanceResponse(): %v", err))
    }

    err = concordance(resp, c)
    if err != nil {
        return errors.New(fmt.Sprintf("error in concordance(): %v", err))
    }

    err = config.Marshal(filepath.Join(a.Directory, "concordance.json"), c)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Marshal(): %v", err))
    }

    return nil
}

func (*Concordance) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "concordance.json"))
}
