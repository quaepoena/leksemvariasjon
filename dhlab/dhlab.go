package dhlab

import (
	"bytes"
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/quaepoena/leksemvariasjon/types"
)

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
	Output  string
	Config  Conf
}

type Concordance struct {
	DocID  map[string]int
	URN    map[string]string
	Conc   map[string]string
	Output string
	Config Conf
}

// BuildCorpus requests data with the parameters from req and populates
// c with the response.
func BuildCorpus(req []byte, c *Corpus) error {
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

// WriteResult writes a Corpus struct to disk as a CSV.
func (c *Corpus) WriteResult(a *Args, header []string) error {
	records := [][]string
	records = append(records, header)
	for key, _ := range c.DHLabID {
		records = append(records, PopulateCorpusRecord(key, c))
	}

	path := filepath.Join(a.Directory, "corpus.csv")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.OpenFile(): %v\n", err))
	}
	defer f.Close()

	w := csv.NewWriter(f)
	err = w.WriteAll(records)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in csv.WriteAll(): %v\n", err))
	}
	defer w.Close()

	return nil
}

func (c *Corpus) Finished() bool {
	return fileExists(c.Output)
}

func (c *Corpus) Run(a *Args, conf *Conf) error {
	req, err := BuildRequest(a, conf)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in Corpus.BuildRequest(): %v\n", err))
	}

	err = BuildCorpus(req, c)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in Corpus.BuildCorpus(): %v\n", err))
	}

	header := []string{"dhlabid", "doctype", "lang", "title", "urn", "year"}
	err = c.WriteResult(header)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in Corpus.WriteResult(): %v\n", err))
	}

	return nil
}

func BuildConcordance(req []byte, c *Concordance) error {
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

func (c *Concordance) Finished() bool {
	return fileExists(os.path.Join(c.Directory, c.Output))
}

func (c *Concordance) Run() error {
	return nil
}

func (c *Concordance) WriteResult() error {
	return nil
}

// fileExists returns true if a given file path exists.
func fileExists(s string) bool {
	f, err := os.Open(s)
	if err != nil {
		return false
	}
	defer f.Close()

	return true
}
