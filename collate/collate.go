// Package collate collects the filtered results and writes them to both JSON
// and CSV.
package collate

import (
    "encoding/csv"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strconv"

    "github.com/quaepoena/leksemvariasjon/config"
    "github.com/quaepoena/leksemvariasjon/dhlab"
    "github.com/quaepoena/leksemvariasjon/filter"
)

// Struct Collation contains all the collected information on a given word
// matching the criteria given in the config file.
type Collation struct {
    DhlabId, Year, Index int
    Doctype, Form, Lemma, LangDhlab, LangId, URN, Value string
}

// Struct Collate contains a list of words matching the criteria given in the
// config file.
type Collate struct {
    Collations []Collation
}

// Function record creates CSV records from a Collation struct.
func record(coll *Collation) []string {
    var fields []string

    fields = append(fields, strconv.Itoa(coll.DhlabId))
    fields = append(fields, strconv.Itoa(coll.Index))
    fields = append(fields, coll.URN)
    fields = append(fields, coll.Doctype)
    fields = append(fields, strconv.Itoa(coll.Year))
    fields = append(fields, coll.Form)
    fields = append(fields, coll.Lemma)
    fields = append(fields, coll.Value)
    fields = append(fields, coll.LangDhlab)
    fields = append(fields, coll.LangId)

    return fields
}

func writeCsv(path string, rows [][]string) error {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
    if err != nil {
        return errors.New(fmt.Sprintf("error in os.OpenFile(): %v\n", err))
    }
    defer f.Close()

    wr := csv.NewWriter(f)
    err = wr.WriteAll(rows)
    if err != nil {
        return errors.New(fmt.Sprintf("error in csv.WriteAll(): %v\n", err))
    }

    return nil
}

// Run collates the filtered results with the information from the (saved)
// dhlab.Corpus struct. The results are written to disk as both JSON and CSV.
func (c *Collate) Run(a *config.Args, _ *config.Conf) error {
    var corp *dhlab.Corpus = &dhlab.Corpus{}
    var fil *filter.Filter = &filter.Filter{}

    err := config.Unmarshal(filepath.Join(a.Directory, "filter.json"), fil)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Unmarshal() with filter.json: %v", err))
    }

    err = config.Unmarshal(filepath.Join(a.Directory, "corpus.json"), corp)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Unmarshal() with corpus.json: %v", err))
    }

    for id, words := range fil.Words {
        for i, word := range words {
            c.Collations = append(c.Collations, Collation{
                DhlabId: id,
                Index: i,
                Form: word.Form,
                Lemma: word.Lemma,
                LangId: word.Lang,
                Value: word.Value,
                Doctype: corp.DHLabID[id].Doctype,
                LangDhlab: corp.DHLabID[id].Lang,
                URN: corp.DHLabID[id].URN,
                Year: corp.DHLabID[id].Year})
        }
    }

    err = config.Marshal(filepath.Join(a.Directory, "collate.json"), c)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Marshal() with collate.json: %v", err))
    }

    var records [][]string
    header := []string{"dhlabid", "index", "urn", "doctype", "year", "form",
        "lemma", "value", "lang_dhlab", "lang_id"}
    records = append(records, header)
    for _, c := range c.Collations {
        records = append(records, record(&c))
    }

    err = writeCsv(filepath.Join(a.Directory, "collate.csv"), records)
    if err != nil {
        return errors.New(fmt.Sprintf("error in writeCsv(): %v", err))
    }

    return nil
}

func (*Collate) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "collate.csv"))
}
