// Command leksemvariasjon displays lexeme variation over Norwegian texts.
// The user creates a configuration file specifying which words and
// morphological features he/she is interested in. The National Library of
// Norway's DHLAB API is queried to find concordance lines with these words
// which are then tagged and the results are filtered and put into a CSV.
// See README.md for more information and usage examples.
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	DHLabAPI = "https://api.nb.no/dhlab/"
)

// Flags.
var (
	concLimit int
	config    string
	corpLimit int
	directory string
	doctype   string
	from      int
	resume    bool
	to        int
)

func init() {
	flag.IntVar(&concLimit, "concLimit", 999999, "Limit the numbers of Concordance results.")
	flag.StringVar(&config, "config", "", "Path to a JSON config file. Required on an initial run.")
	flag.IntVar(&corpLimit, "corpLimit", 999999, "Limit the numbers of Corpus results.")
	flag.StringVar(&directory, "directory", "", "Directory to write files to, creating it if it doesn't exist.")
	flag.StringVar(&doctype, "doctype", "", "The doctype to search for.")
	flag.BoolVar(&resume, "resume", false, "Resume a previously started job.")
	flag.IntVar(&from, "from", 0, "The start year for the search.")
	flag.IntVar(&to, "to", 0, "The end year for the search (inclusive).")
}

// Struct Args holds command line arguments and saves/reads them to/from disk.
type Args struct {
	ConfigFile, Directory, Doctype string
	ConcLimit, CorpLimit, From, To int
}

// Struct Word contains word forms with the corresponding morphological
// information the user wants to search for.
type Word struct {
	Form, Value string
	Morphology  []string
}

// Struct Lemma holds a lemma and all the word forms under it.
type Lemma struct {
	Lemma string
	Words []Word
}

// Struct Conf holds the parsed information from the config.
type Conf struct {
	Attribute, Language string
	Lemmas              []Lemma
}

// Interface WorkflowStage represents the repetitive tasks for each step of the
// program.
type WorkflowStage interface {
	run(*Args, *Conf) error
	finished(*Args) bool
}

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
	Doctype  string `json:"doctype"`
	FromYear int    `json:"from_year"`
	ToYear   int    `json:"to_year"`
	Fulltext string `json:"fulltext"`
	Lang     string `json:"lang"`
	Limit    int    `json:"limit"`
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

// corpusRequest builds and returns the JSON object used in the DHLab
// build_corpus call.
func corpusRequest(a *Args, c *Conf) ([]byte, error) {
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

// corpusResponse calls the build_corpus endpoint with req and populates
// c with the response.
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

// corpus populates a Corpus struct with data from the build_corpus
// API call.
func corpus(resp *CorpusResponse, c *Corpus) error {
	c.DHLabID = make(map[int]CorpusMetadata)

	for i, v := range resp.DHLabID {
		c.DHLabID[v] = CorpusMetadata{
			Doctype: resp.Doctype[i],
			Lang:    resp.Langs[i],
			URN:     resp.URN[i],
			Year:    resp.Year[i]}
	}

	return nil
}

// run builds a Corpus object and writes it to disk as JSON.
func (c *Corpus) run(a *Args, conf *Conf) error {
	var req []byte
	var resp *CorpusResponse = &CorpusResponse{}

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

	err = marshal(filepath.Join(a.Directory, "corpus.json"), c)
	if err != nil {
		return errors.New(fmt.Sprintf("error in json.Marshal(): %v", err))
	}

	return nil
}

func (*Corpus) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "corpus.json"))
}

// Struct ConcordanceRequest contains the necessary information for the DHLab
// conc API call.
type ConcordanceRequest struct {
	DHLabIDs       []int  `json:"dhlabids"`
	HTMLFormatting bool   `json:"html_formatting"`
	Limit          int    `json:"limit"`
	Query          string `json:"query"`
	Window         int    `json:"window"`
}

// Struct ConcordanceResponse contains the information we save from the DHLab
// conc API call.
type ConcordanceResponse struct {
	DocID map[string]int
	Conc  map[string]string
}

// Struct Concordance contains concordance lines, mapped as DHLab IDs to list
// strings, i.e. concordance lines.
type Concordance struct {
	Lines map[int][]string
}

// dhlabIDs returns a list of IDs from a Corpus object previously saved to disk
// as JSON.
func dhlabIDs(a *Args) ([]int, error) {
	var ids []int
	var corp *Corpus = &Corpus{}

	err := unmarshal(filepath.Join(a.Directory, "corpus.json"), corp)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error in unmarshal(): %v", err))
	}

	for i := range corp.DHLabID {
		ids = append(ids, i)
	}

	return ids, nil
}

// concordanceRequest creates the JSON object needed from the conc API call.
func concordanceRequest(a *Args, c *Conf, ids []int) ([]byte, error) {
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

// concordanceResponse calls the conc API call and populates a
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

// concordance populates a Concordance struct with the data from a
// ConcordanceResponse.
func concordance(concResp *ConcordanceResponse, conc *Concordance) error {
	conc.Lines = make(map[int][]string)

	for i, v := range concResp.DocID {
		conc.Lines[v] = append(conc.Lines[v], concResp.Conc[i])
	}

	return nil
}

// run uses a list of previously fetched DHLab IDs to get concordance lines and
// writh the results to disk as JSON.
func (c *Concordance) run(a *Args, conf *Conf) error {
	var ids []int
	var resp *ConcordanceResponse = &ConcordanceResponse{}

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

	err = marshal(filepath.Join(a.Directory, "concordance.json"), c)
	if err != nil {
		return errors.New(fmt.Sprintf("error in json.Marshal(): %v", err))
	}

	return nil
}

func (*Concordance) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "concordance.json"))
}

// Struct Tag represents running the external tagger.
// The data it works with is read from and written directly to disk.
type Tag struct{}

func writeFilesForTagging(conc *Concordance, p string) error {
	for i, v := range conc.Lines {
		for _, line := range v {
			name := filepath.Join(p,
				strconv.Itoa(i)+"-"+strconv.FormatInt(time.Now().UTC().UnixMicro(), 10))
			err := os.WriteFile(name, []byte(line), 0666)
			if err != nil {
				return errors.New(fmt.Sprintf("error in os.WriteFile() with path %s: %v", name, err))
			}
		}

	}

	return nil
}

func tagFiles(p string) error {
	cmd := exec.Command("python", "./tagger.py", p, p)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.New(fmt.Sprintf("error in cmd.StdoutPipe(): %v", err))
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.New(fmt.Sprintf("error in cmd.StderrPipe(): %v", err))
	}

	if err := cmd.Start(); err != nil {
		return errors.New(fmt.Sprintf("error in cmd.Start(): %v", err))
	}

	tOut, err := io.ReadAll(stdout)
	if err != nil {
		return errors.New(fmt.Sprintf("error in io.ReadAll() (stdout): %v", err))
	}

	tErr, _ := io.ReadAll(stderr)
	if err != nil {
		return errors.New(fmt.Sprintf("error in io.ReadAll() (stderr): %v", err))
	}

	if err := cmd.Wait(); err != nil {
		return errors.New(fmt.Sprintf("error in cmd.Wait(): %v", err))
	}

	fmt.Printf("%s\n", tOut)
	fmt.Printf("%s\n", tErr)

	return nil
}

func (*Tag) run(a *Args, _ *Conf) error {
	var conc *Concordance = &Concordance{}

	p := filepath.Join(a.Directory, "tagged")
	err := os.MkdirAll(p, 0775)
	if err != nil {
		return errors.New(fmt.Sprintf("error in os.Mkdir(): %v\n", err))
	}

	err = unmarshal(filepath.Join(a.Directory, "concordance.json"), conc)
	if err != nil {
		return errors.New(fmt.Sprintf("error in unmarshal(): %v", err))
	}

	err = writeFilesForTagging(conc, p)
	if err != nil {
		return errors.New(fmt.Sprintf("error in writeFilesForTagging(): %v", err))
	}

	err = tagFiles(p)
	if err != nil {
		return errors.New(fmt.Sprintf("error in tagFiles(): %v", err))
	}

	err = os.WriteFile(filepath.Join(a.Directory, "taggingFinished.txt"), []byte{}, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("error in os.WriteFile(): %v", err))
	}

	return nil
}

func (*Tag) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "taggingFinished.txt"))
}

// Struct TaggedWord ...
type TaggedWord struct {
	Word  string   `json:"w"`
	Tags  []string `json:"t"`
	Lemma string   `json:"l"`
}

// Struct TaggedEntry ...
type TaggedEntry struct {
	Lang        string
	TaggedWords []TaggedWord `json:"sent"`
}

// Struct MatchingWord ...
type MatchingWord struct {
	Form, Lang, Lemma, Value string
}

// Struct Filter ...
type Filter struct {
	Words map[int][]MatchingWord
}

// dhlabID returns the dhlabID from a (tagged) filename.
func dhlabID(s string) (int, error) {
	re := regexp.MustCompile("^/.+/([[:digit:]]+)-.+$")
	id, err := strconv.Atoi(re.ReplaceAllString(s, "$1"))
	if err != nil {
		return 0, errors.New(fmt.Sprintf("error in strconv.Atoi(): %v", err))
	}

	return id, nil
}

func matchingWords(entry *TaggedEntry, conf *Conf, dhlabId int, fil *Filter) error {
	for _, lemma := range conf.Lemmas {
		for _, word := range lemma.Words {
			for _, taggedWord := range entry.TaggedWords {

				if taggedWord.Lemma == lemma.Lemma &&
					taggedWord.Word == word.Form &&
					sets.New(taggedWord.Tags...).IsSuperset(
						sets.New(word.Morphology...)) {

					fil.Words[dhlabId] = append(fil.Words[dhlabId], MatchingWord{
						Form:  taggedWord.Word,
						Lang:  entry.Lang,
						Lemma: taggedWord.Lemma,
						Value: word.Value})
				}
			}
		}
	}

	return nil
}

func (f *Filter) run(a *Args, conf *Conf) error {
	f.Words = make(map[int][]MatchingWord)
	var tagged []string
	dir := filepath.Join(a.Directory, "tagged")

	files, err := os.ReadDir(dir)
	if err != nil {
		return errors.New(fmt.Sprintf("error in os.ReadDir(): %v", err))
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".tagged") {
			tagged = append(tagged, filepath.Join(dir, file.Name()))
		}
	}

	for _, t := range tagged {
		dhlabId, err := dhlabID(t)
		if err != nil {
			return errors.New(fmt.Sprintf("error in dhlabID(): %v", err))
		}

		fd, err := os.Open(t)
		if err != nil {
			return errors.New(fmt.Sprintf("error in os.Open() with %s: %v",
				t, err))
		}

		s := bufio.NewScanner(fd)
		for s.Scan() {
			entry := &TaggedEntry{}
			err = json.Unmarshal(s.Bytes(), &entry)
			if err != nil {
				return errors.New(fmt.Sprintf("error in json.Unmarshal(): %v", err))
			}

			err = matchingWords(entry, conf, dhlabId, f)
			if err != nil {
				return errors.New(fmt.Sprintf("error in matchingWords(): %v", err))
			}
		}

		if err = s.Err(); err != nil {
			return errors.New(fmt.Sprintf("error while scanning %s: %v",
				t, err))
		}

		fd.Close()
	}


	err = marshal(filepath.Join(a.Directory, "filter.json"), f)
	if err != nil {
		return errors.New(fmt.Sprintf("error in marshal() with filter.json: %v", err))
	}

	return nil
}

func (*Filter) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "filter.json"))
}

// Struct Collation ...
type Collation struct {
	DhlabId, Year                                       int
	Doctype, Form, Lemma, LangDhlab, LangId, URN, Value string
}

// Struct Collate ...
type Collate struct {
	Collations []Collation
}

func record(coll *Collation) []string {
	var fields []string

	fields = append(fields, strconv.Itoa(coll.DhlabId))
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

func (c *Collate) run(a *Args, conf *Conf) error {
	var corp *Corpus = &Corpus{}
	var fil *Filter = &Filter{}

	err := unmarshal(filepath.Join(a.Directory, "filter.json"), fil)
	if err != nil {
		return errors.New(fmt.Sprintf("error in unmarshal() with filter.json: %v", err))
	}

	err = unmarshal(filepath.Join(a.Directory, "corpus.json"), corp)
	if err != nil {
		return errors.New(fmt.Sprintf("error in unmarshal() with corpus.json: %v", err))
	}

	for id, words := range fil.Words {
		for _, word := range words {
			c.Collations = append(c.Collations, Collation{
				DhlabId:   id,
				Form:      word.Form,
				Lemma:     word.Lemma,
				LangId:    word.Lang,
				Value:     word.Value,
				Doctype:   corp.DHLabID[id].Doctype,
				LangDhlab: corp.DHLabID[id].Lang,
				URN:       corp.DHLabID[id].URN,
				Year:      corp.DHLabID[id].Year})
		}
	}

	err = marshal(filepath.Join(a.Directory, "collate.json"), c)
	if err != nil {
		return errors.New(fmt.Sprintf("error in marshal() with collate.json: %v", err))
	}

	var records [][]string
	header := []string{"dhlabid", "urn", "doctype", "year", "form", "lemma",
		"value", "lang_dhlab", "lang_id"}
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

func (*Collate) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "collate.csv"))
}

// uniqueDir makes a unique output directory for each (non-resumptive) run
// of the program.
func uniqueDir(dir string, config string) (string, error) {
	var base, newDir, tStamp string
	var t time.Time

	t = time.Now().UTC()
	tStamp = t.Format("2006-01-02_15:04:05")
	base = filepath.Base(config)
	newDir = filepath.Join(dir,
		tStamp+"_"+strings.TrimSuffix(base, ".json"))

	err := os.MkdirAll(newDir, 0755)
	if err != nil {
		return "", errors.New(fmt.Sprintf("error in os.MkdirAll(): %v\n", err))
	}

	return newDir, nil
}

// copyConfig copies the configuration file to the newly created output directory.
func copyConfig(dir string, config string) error {
	var p string
	var dst, src *os.File

	src, err := os.Open(config)
	if err != nil {
		return errors.New(fmt.Sprintf("error in os.Open(): %v\n", err))
	}
	defer src.Close()

	p = filepath.Join(dir, filepath.Base(config))
	dst, err = os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("error in os.OpenFile(): %v\n", err))
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return errors.New(fmt.Sprintf("error in io.Copy(): %v\n", err))
	}

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

// writeCsv
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

// unmarshal
func unmarshal(p string, s any) error {
	var b []byte

	f, err := os.Open(p)
	if err != nil {
		return errors.New(fmt.Sprintf("error in os.Open(): %v", err))
	}
	defer f.Close()

	b, err = io.ReadAll(f)
	if err != nil {
		return errors.New(fmt.Sprintf("error in io.ReadAll(): %v", err))
	}

	err = json.Unmarshal(b, s)
	if err != nil {
		return errors.New(fmt.Sprintf("error in json.Unmarshal(): %v", err))
	}

	return nil
}

// marshal
func marshal(p string, s any) error {
	b, err := json.Marshal(s)
	if err != nil {
		return errors.New(fmt.Sprintf("error in json.Marshal(): %v", err))
	}

	err = os.WriteFile(p, b, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("error in os.WriteFile() with %s: %v", p, err))
	}

	return nil
}

func main() {
	var args *Args = &Args{}
	var corp *Corpus = &Corpus{}
	var conc *Concordance = &Concordance{}
	var conf *Conf = &Conf{}
	var tag *Tag = &Tag{}
	var filter *Filter = &Filter{}
	var coll *Collate = &Collate{}
	var err error

	flag.Parse()

	if directory == "" {
		fmt.Fprintln(os.Stderr, "Flag '-directory' must be set.")
		os.Exit(1)
	}

	if from > to {
		fmt.Fprintln(os.Stderr, "Flag '-to' must be greater than or equal to '-from'.")
		os.Exit(1)
	}

	if resume == true && (config != "" || doctype != "" || from != 0 || to != 0) {
		fmt.Fprintln(os.Stderr, "Flag '-resume' isn't meant to be combined ",
			"with '-config', '-doctype', '-from' or '-to'.")
		os.Exit(1)
	}

	if resume == false && (config == "" || doctype == "" || from == 0 || to == 0) {
		fmt.Fprintln(os.Stderr, "Flags '-config', '-doctype', '-from', and '-to' must be set when not using '-resume'.")
		os.Exit(1)
	}

	if resume {
		// For resumptive runs we read the arguments back from disk and set
		// the variables accordingly.
		err = unmarshal(filepath.Join(directory, "args.json"), args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in unmarshal(): %v\nThis is a resumptive run. Did you specify the already-existing output directory from a previous run?", err)
			os.Exit(1)
		}
	} else {
		// For non-resumptive runs we need to 1) create a unique directory,
		// 2) copy the arguments and JSON config file thither, and 3) set
		// flag values appropriately.

		// The '-directory' flag is changed to the new, unique directory.
		directory, err = uniqueDir(directory, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in uniqueDir(): %v", err)
			os.Exit(1)
		}

		err = copyConfig(directory, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in copyConfig(): %v", err)
			os.Exit(1)
		}

		// The '-config' flag, which (potentially) included directories,
		// is changed to the basename.
		config = filepath.Base(config)

		args.ConcLimit = concLimit
		args.CorpLimit = corpLimit
		args.ConfigFile = config
		args.Directory = directory
		args.Doctype = doctype
		args.From = from
		args.To = to
		err = marshal(filepath.Join(directory, "args.json"), args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in marshal(): %v", err)
			os.Exit(1)
		}
	}

	err = unmarshal(filepath.Join(directory, args.ConfigFile), conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error in unmarshal(): %v", err)
		os.Exit(1)
	}
	stages := []WorkflowStage{corp, conc, tag, filter, coll}
	for _, s := range stages {
		if !s.finished(args) {

			err = s.run(args, conf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error in %T.Run(): %v", s, err)
				os.Exit(1)
			}
		}
	}
}
