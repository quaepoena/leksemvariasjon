// Command leksemvariasjon displays lexeme variation over Norwegian texts.
// The user creates a configuration file which tells which words and
// morphological features he/she is interested in. The National Library of
// Norway's DHLAB API is queried to find concordance lines which are then tagged
// and the results are filtered and put into a CSV.
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/util/sets"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DHLabAPI = "https://api.nb.no/dhlab/"
)

// Flags.
var (
	config    string
	directory string
	doctype   string
	from      int
	resume    bool
	to        int
)

func init() {
	flag.StringVar(&config, "config", "", "Path to a JSON config file. Required on an initial run.")
	flag.StringVar(&directory, "directory", "", "Directory to write files to, creating it if it doesn't exist.")
	flag.StringVar(&doctype, "doctype", "", "The doctype to search for.")
	flag.BoolVar(&resume, "resume", false, "Resume a previously started job.")
	flag.IntVar(&from, "from", 0, "The start year for the search.")
	flag.IntVar(&to, "to", 0, "The end year for the search (inclusive).")
}

// Struct Args holds command line arguments and saves/read them to/from disk.
type Args struct {
	ConfigFile, Directory, Doctype string
	From, To                       int
}

// Struct Word contains word forms with the corresponding morphological
// information the user wants to search for.
type Word struct {
	Form, Value string
	Morphology  []string
}

// Struct Lemma holds a lemma and all the Words under it.
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

// Struct MatchingEntry ...
type MatchingEntry struct {
	Attribute, Form, Lang, Lemma, Value string
	DhlabId                             int
}

// Struct CorpusMetadata ...
type CorpusMetadata struct {
	Doctype string
	Lang    string
	URN     string
	Year    int
}

// Struct Corpus ...
type Corpus struct {
	DHLabID map[int]CorpusMetadata
}

// Struct CorpusRequest contains the necessary information for the DHLab
// build_corpus API call.
type CorpusRequest struct {
	Doctype  string `json:"doctype"`
	FromYear int    `json:"from_year"`
	ToYear   int    `json:"to_year"`
	Fulltext string `json:"fulltext"`
	Lang     string `json:"lang"`
	Limit    int    `json:"limit"`
}

// Struct CorpusResponse contains the information we need from the DHLab build_corpus
// API call.
type CorpusResponse struct {
	DHLabID map[string]int
	Doctype map[string]string
	Langs   map[string]string
	URN     map[string]string
	Year    map[string]int
}

// buildCorpusRequest builds and returns a JSON object for the DHLab
// build_corpus call.
func buildCorpusRequest(a *Args, c *Conf) ([]byte, error) {
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
	req.Limit = 10        // TODO: Change after testing.
	req.Fulltext = strings.Join(words, " OR ")
	req.Lang = c.Language

	b, err := json.Marshal(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error on json.Marshal():\n%v\n", err))
	}

	return b, nil
}

// buildCorpusResponse requests data with the parameters from req and populates
// c with the response.
func buildCorpusResponse(req []byte, c *CorpusResponse) error {
	var uri = DHLabAPI + "build_corpus"

	resp, err := http.Post(uri, "application/json", bytes.NewReader(req))
	if err != nil {
		return errors.New(fmt.Sprintf("Error in http.Post():\n%v\n", err))
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in io.ReadAll():\n%v\n", err))
	}

	err = json.Unmarshal(b, c)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in json.Unmarshal():\n%v\n", err))
	}

	return nil
}

// buildCorpus ...
func buildCorpus(resp *CorpusResponse, c *Corpus) error {
	for i, v := range resp.DHLabID {
		c.DHLabID[v] = CorpusMetadata{
			Doctype: resp.Doctype[i],
			Lang:    resp.Langs[i],
			URN:     resp.URN[i],
			Year:    resp.Year[i]}
	}

	return nil
}

func (c *Corpus) run(a *Args, conf *Conf) error {
	var req []byte
	var resp *CorpusResponse

	req, err := buildCorpusRequest(a, conf)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in Corpus.buildRequest():\n%v\n", err))
	}

	err = buildCorpusResponse(req, resp)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in Corpus.buildCorpusResponse():\n%v\n", err))
	}

	err = buildCorpus(resp, c)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in buildCorpus():\n%v\n", err))
	}

	b, err := json.Marshal(c)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in json.Marshal():\n%v\n", err))
	}
	err = os.WriteFile(filepath.Join(a.Directory, "corpus.json"), b, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.WriteFile() with corpus.json:\n%v\n", err))
	}

	return nil
}

func (c *Corpus) finished(a *Args) bool {
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

// Struct ConcordanceResponse contains the information we need from the DHLab conc
// API call.
type ConcordanceResponse struct {
	DocID map[string]int
	Conc  map[string]string
}

// Struct Concordance ...
type Concordance struct {
	Lines map[int][]string
}

func buildConcordanceRequest(a *Args, c *Conf, ids []int) ([]byte, error) {
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
	req.Limit = 10 // TODO: Change after testing.
	req.Query = strings.Join(words, " OR ")
	req.Window = 25

	b, err := json.Marshal(req)
	if err != nil {
		return []byte{}, errors.New(fmt.Sprintf("Error on json.Marshal():\n%v\n", err))
	}

	return b, nil
}

// buildConcordanceResponse requests data with the parameters from req and populates
// concResp with the result.
func buildConcordanceResponse(req []byte, concResp *ConcordanceResponse) error {
	var uri = DHLabAPI + "conc"

	resp, err := http.Post(uri, "application/json", bytes.NewReader(req))
	if err != nil {
		return errors.New(fmt.Sprintf("Error in http.Post():\n%v\n", err))
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in io.ReadAll():\n%v\n", err))
	}

	err = json.Unmarshal(b, concResp)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in json.Unmarshal():\n%v\n", err))
	}

	return nil
}

func dhlabIDs(a *Args) ([]int, error) {
	var ids []int
	var b []byte
	var corp *Corpus

	f, err := os.Open(filepath.Join(a.Directory, "corpus.json"))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error in os.Open():\n%v\n", err))
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		_, err = r.Read(b)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error in bufio.Read()():\n%v\n", err))
		}
	}

	err = json.Unmarshal(b, corp)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error in json.Unmarshal():\n%v\n", err))
	}

	for i := range corp.DHLabID {
		ids = append(ids, i)
	}

	return ids, nil
}

func (conc *Concordance) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "concordance.json"))
}

func (conc *Concordance) run(a *Args, c *Conf) error {
	var ids []int
	var resp *ConcordanceResponse

	ids, err := dhlabIDs(a)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in dhlabIDs():\n%v\n", err))
	}
	if ids == nil {
		return errors.New(fmt.Sprintf("No dhlabIDs were found in %s.",
			filepath.Join(a.Directory, "corpus.json")))
	}

	req, err := buildConcordanceRequest(a, c, ids)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in ConcordanceRequest():\n%v\n", err))
	}

	err = buildConcordanceResponse(req, resp)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in BuildConcordanceResponse():\n%v\n", err))
	}

	b, err := json.Marshal(conc)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in json.Marshal():\n%v\n", err))
	}

	err = os.WriteFile(filepath.Join(a.Directory, "concordance.json"), b, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.WriteFile() with concordance.json:\n%v\n", err))
	}

	return nil
}

// Struct Tag represents running the external tagger.
// The data it works with is read from and written directly to disk.
type Tag struct{}

func writeFilesToBeTagged(c *ConcordanceResponse, p string) error {
	for key := range c.DocID {
		id := c.DocID[key]
		conc := c.Conc[key]

		os.WriteFile(filepath.Join(p, strconv.Itoa(id)+"-"+strconv.FormatInt(time.Now().UnixMicro(), 10)),
			[]byte(conc), 0666)
	}

	return nil
}

func (t *Tag) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "taggingFinished.txt"))
}

func (t *Tag) run(a *Args, conf *Conf) error {
	p := filepath.Join(a.Directory, "tagged")
	err := os.Mkdir(p, 0775)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.Mkdir(): %v\n", err))
	}

	var resp *ConcordanceResponse
	err = writeFilesToBeTagged(resp, p)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in writeFilesToBeTagged():\n%v\n", err))
	}

	err = os.WriteFile(filepath.Join(a.Directory, "concordanceWritten.txt"), []byte{}, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.WriteFile():\n%v\n", err))
	}

	p = filepath.Join(a.Directory, "tagged")
	cmd := exec.Command("python", "./tagger.py", p, p)

	err = cmd.Run()
	if err != nil {
		return errors.New(fmt.Sprintf("Error in Cmd.Run():\n%v\n", err))
	}

	err = os.WriteFile(filepath.Join(a.Directory, "taggingFinished.txt"), []byte{}, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.WriteFile():\n%v\n", err))
	}

	return nil
}

func (t *Tag) writeResult(a *Args) error {
	return nil
}

// Struct Filter ...
type Filter struct{}

func extractDhlabId(s string) (int, error) {
	start, err := regexp.Compile("^/.*/")
	if err != nil {
		return 0, errors.New(fmt.Sprintf("Error in regexp.Compile():\n%v\n", err))
	}
	end, err := regexp.Compile("-.*$")
	if err != nil {
		return 0, errors.New(fmt.Sprintf("Error in regexp.Compile():\n%v\n", err))
	}

	pre := start.ReplaceAllLiteralString(s, "")
	suf := end.ReplaceAllLiteralString(pre, "")

	id, err := strconv.Atoi(suf)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("Error in strconv.Atoi():\n%v\n", err))
	}

	return id, nil
}

func matching(taggedEntry *TaggedEntry, conf *Conf, dhlabId int) []MatchingEntry {
	var matching []MatchingEntry

	for _, lemma := range conf.Lemmas {
		for _, word := range lemma.Words {
			for _, taggedWord := range taggedEntry.TaggedWords {
				if taggedWord.Lemma == lemma.Lemma &&
					taggedWord.Word == word.Form &&
					sets.New(taggedWord.Tags...).IsSuperset(
						sets.New(word.Morphology...)) {
					matching = append(matching, MatchingEntry{
						Attribute: conf.Attribute,
						Form:      taggedWord.Word,
						Lang:      taggedEntry.Lang,
						Lemma:     taggedWord.Lemma,
						Value:     word.Value,
						DhlabId:   dhlabId})
				}
			}
		}
	}

	return matching
}

func (t *Filter) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "filteringFinished.txt"))
}

func (f *Filter) run(a *Args, conf *Conf) error {
	var matchingWords []MatchingEntry
	var tagged []string
	dir := filepath.Join(a.Directory, "tagged")

	files, err := os.ReadDir(dir)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.ReadDir():\n%v\n", err))
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".tagged") {
			tagged = append(tagged, filepath.Join(dir, f.Name()))
		}
	}

	for _, t := range tagged {
		f, err := os.Open(t)
		if err != nil {
			return errors.New(fmt.Sprintf("Error in os.Open() with %s:\n%v\n",
				t, err))
		}
		defer f.Close()

		dhlabId, err := extractDhlabId(t)
		if err != nil {
			return errors.New(fmt.Sprintf("Error in extractDhlabID():\n%v\n", err))
		}

		s := bufio.NewScanner(f)
		for s.Scan() {
			taggedEntry := TaggedEntry{}
			err = json.Unmarshal(s.Bytes(), &taggedEntry)
			if err != nil {
				return errors.New(fmt.Sprintf("Error in json.Unmarshal():\n%v\n", err))
			}

			matchingWords = append(matchingWords, matching(&taggedEntry, conf, dhlabId)...)
		}

		if err = s.Err(); err != nil {
			return errors.New(fmt.Sprintf("Error while scanning file %s:\n%v\n",
				t, err))
		}

	}

	for _, m := range matchingWords {
		fmt.Println(m)
	}

	return nil
}

func (t *Filter) writeResult(a *Args) error {
	err := os.WriteFile(filepath.Join(a.Directory, "filteringFinished.txt"), []byte{}, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.WriteFile():\n%v\n", err))
	}

	return nil
}

// Struct Collate ...
type Collate struct{}

func (c *Collate) finished(a *Args) bool {
	return fileExists(filepath.Join(a.Directory, "filteringFinished.txt"))
}

func (f *Collate) run(a *Args, conf *Conf) error {
	var matchingWords []MatchingEntry
	var tagged []string
	dir := filepath.Join(a.Directory, "tagged")

	files, err := os.ReadDir(dir)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.ReadDir():\n%v\n", err))
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".tagged") {
			tagged = append(tagged, filepath.Join(dir, f.Name()))
		}
	}

	for _, t := range tagged {
		f, err := os.Open(t)
		if err != nil {
			return errors.New(fmt.Sprintf("Error in os.Open() with %s:\n%v\n",
				t, err))
		}
		defer f.Close()

		dhlabId, err := extractDhlabId(t)
		if err != nil {
			return errors.New(fmt.Sprintf("Error in extractDhlabID():\n%v\n", err))
		}

		s := bufio.NewScanner(f)
		for s.Scan() {
			taggedEntry := TaggedEntry{}
			err = json.Unmarshal(s.Bytes(), &taggedEntry)
			if err != nil {
				return errors.New(fmt.Sprintf("Error in json.Unmarshal():\n%v\n", err))
			}

			matchingWords = append(matchingWords, matching(&taggedEntry, conf, dhlabId)...)
		}

		if err = s.Err(); err != nil {
			return errors.New(fmt.Sprintf("Error while scanning file %s:\n%v\n",
				t, err))
		}

	}

	for _, m := range matchingWords {
		fmt.Println(m)
	}

	return nil
}

func (c *Collate) writeResult(a *Args) error {
	err := os.WriteFile(filepath.Join(a.Directory, "filteringFinished.txt"), []byte{}, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.WriteFile():\n%v\n", err))
	}

	return nil
}

// readArgs reads arguments (from a previous run) from path and stores them in a.
func readArgs(path string, a *Args) error {
	var f *os.File
	var dec *gob.Decoder

	f, err := os.Open(path)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.Open(): %v\n", err))
	}
	defer f.Close()

	dec = gob.NewDecoder(f)
	err = dec.Decode(a)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in dec.Decode(): %v\n", err))
	}

	return nil
}

// mkUniqueDir makes a unique output directory for each (non-resumptive) run
// of the program.
func mkUniqueDir(dir string, config string) (string, error) {
	var base, newDir, tStamp string
	var t time.Time

	t = time.Now()
	tStamp = t.Format(time.DateTime)
	base = filepath.Base(config)
	newDir = filepath.Join(dir,
		tStamp+"-"+strings.TrimSuffix(base, ".json"))

	err := os.MkdirAll(newDir, 0755)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Error on os.MkdirAll(): %v\n", err))
	}

	return newDir, nil
}

// copyConfig copies the configuration file to the newly created output directory.
func copyConfig(dir string, config string) error {
	var destPath string
	var destFile, srcFile *os.File

	srcFile, err := os.Open(config)
	if err != nil {
		return errors.New(fmt.Sprintf("Error on os.Open(): %v\n", err))
	}
	defer srcFile.Close()

	destPath = filepath.Join(dir, filepath.Base(config))
	destFile, err = os.OpenFile(destPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error on os.OpenFile(): %v\n", err))
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return errors.New(fmt.Sprintf("Error on io.Copy(): %v\n", err))
	}

	return nil
}

// writeArgs saves the arguments from a to disk in case of a resumptive run.
func writeArgs(dir string, a *Args) error {
	var argFile *os.File
	var e *gob.Encoder
	var path string

	path = filepath.Join(dir, "args.gob")
	argFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.OpenFile(): %v\n", err))
	}
	defer argFile.Close()

	e = gob.NewEncoder(argFile)
	err = e.Encode(a)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in e.Encode(): %v\n", err))
	}

	return nil
}

// loadConf reads the JSON configuration file at path into c.
func loadConf(path string, c *Conf) error {
	var data []byte

	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.ReadFile(): %v\n", err))
	}

	err = json.Unmarshal(data, c)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in json.Unmarshal(): %v\n", err))
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

func csvColumn(p string, c int) ([]any, error) {
	var fields []any

	f, err := os.Open(p)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error in os.Open():\n%v\n", err))
	}
	defer f.Close()

	r := csv.NewReader(f)
	_, err = r.Read() // Discard the header.
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error in csv.Read():\n%v\n", err))
	}

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error in csv.Read():\n%v\n", err))
		}

		fields = append(fields, rec[c])
	}

	return fields, nil
}

func dhlabIDsOld(p string, f int) ([]int, error) {
	var IDs []int

	s, err := csvColumn(p, f)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error in csvColumn():\n%v\n", err))
	}

	for i := 0; i < len(s); i++ {
		st := s[i].(string)
		in, err := strconv.Atoi(st)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error in strconv.Atoi():\n%v\n", err))
		}

		IDs = append(IDs, in)
	}

	return IDs, nil
}

// concordanceLines returns each selection of concordance text as a list of strings.
func concordanceLines(p string) ([]string, error) {
	var lines []string

	f, err := os.Open(p)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error in os.Open():\n%v\n", err))
	}
	defer f.Close()

	r := csv.NewReader(f)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Error in csv.Read():\n%v\n", err))
		}
		if rec[2] == "text" {
			continue
		}

		lines = append(lines, rec[2])
	}

	return lines, nil
}

// writeCsv
func writeCsv(rows [][]string, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in os.OpenFile(): %v\n", err))
	}
	defer f.Close()

	wr := csv.NewWriter(f)
	err = wr.WriteAll(rows)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in csv.WriteAll(): %v\n", err))
	}

	return nil
}

func main() {
	var args Args = Args{}

	var corp Corpus = Corpus{}
	var conc Concordance = Concordance{}
	var conf Conf = Conf{}
	var filter Filter = Filter{}
	var tag Tag = Tag{}
	var coll Collate = Collate{}
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
		err = readArgs(filepath.Join(directory, "args.gob"), &args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in readArgs():\n%v\nThis is a resumptive run. Did you specify the already-existing output directory from a previous run?", err)
			os.Exit(1)
		}

	} else {
		// For non-resumptive runs we need to 1) create a unique directory,
		// 2) copy the arguments and JSON config file thither, and 3) set
		// flag values appropriately.

		// The '-directory' flag is changed to the new, unique directory.
		directory, err = mkUniqueDir(directory, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in mkUniqueDir():\n%v\n", err)
			os.Exit(1)
		}

		err = copyConfig(directory, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in copyConfig():\n%v\n", err)
			os.Exit(1)
		}

		// The '-config' flag, which was a path, is changed to the basename.
		config = filepath.Base(config)

		args = Args{ConfigFile: config, Directory: directory, Doctype: doctype,
			From: from, To: to}
		err = writeArgs(directory, &args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in writeArgs():\n%v\n", err)
			os.Exit(1)
		}
	}

	err = loadConf(filepath.Join(directory, args.ConfigFile), &conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in loadConf():\n%v\n", err)
		os.Exit(1)
	}

	stages := []WorkflowStage{&corp, &conc, &tag, &filter, &coll}
	for _, s := range stages {
		if !s.finished(&args) {

			err = s.run(&args, &conf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in %T.Run():\n%v\n", s, err)
				os.Exit(1)
			}
		}
	}
}
