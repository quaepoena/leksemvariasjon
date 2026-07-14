package main

import (
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quaepoena/leksemvariasjon/dhlab"
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

type WorkflowStage interface {
	Finished() bool
	Run(*Args, *Conf) error
}

type Args struct {
	ConfigFile, Directory, Doctype string
	From, To                       int
}

type Word struct {
	Form, Value string
	Morphology  []string
}

type Lemma struct {
	Lemma string
	Words []Word
}

type Conf struct {
	Attribute, Language string
	Lemmas              []Lemma
}

type Concordance struct {
	DocID  map[string]int
	URN    map[string]string
	Conc   map[string]string
	Output string
	Config Conf
}

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

type CorpusRequest struct {
	Doctype  string `json:"doctype"`
	FromYear int    `json:"from_year"`
	ToYear   int    `json:"to_year"`
	Fulltext string `json:"fulltext"`
	Lang     string `json:"lang"`
	Limit    int    `json:"limit"`
}

type ConcordanceRequest struct {
	DHLabIDs       []int  `json:"dhlabids"`
	HTMLFormatting bool   `json:"html_formatting"`
	Limit          int    `json:"limit"`
	Query          string `json:"query"`
	Window         int    `json:"window"`
}

func (c *Corpus) Finished() bool {
	return fileExists(c.Output)
}

func (c *Corpus) Run(a *Args, conf *Conf) error {

	req, err := dhlab.CorpusRequest(a, conf)
	if err != nil {
		return errors.New(fmt.Sprintf("Error in Corpus.Run(): %v\n", err))
		// fmt.Fprintf(os.Stderr, "Error in dhlab.CorpusRequest():\n%v\n", err)
		// os.Exit(1)
	}

	err = dhlab.Corpus(req, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in dhlab.Corpus():\n%v\n", err)
		os.Exit(1)
	}

	header = []string{"dhlabid", "doctype", "lang", "title", "urn", "year"}
	err = c.WriteResult(header)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in Corpus.WriteResult():\n%v\n", err)
		os.Exit(1)
	}

	return nil
}

func (c *Corpus) WriteResult(header []string) error {
	records = append(records, header)
	for key, _ := range c.DHLabID {
		records = append(records, dhlab.PopulateCorpusRecord(key, c))
	}

	path := filepath.Join(c.Directory, "corpus.csv")
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

func (c *Concordance) Finished() bool {
	return fileExists(os.path.Join(c.Directory, c.Output))
}

func (c *Concordance) Run() error {
	return nil
}

func (c *Concordance) WriteResult() error {
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

// mkUniqueDir makes a unique output directory for each (non-resumptive) run of the program.
func mkUniqueDir(dir string, config string) (string, error) {
	var base, newDir, tStamp string
	var t time.Time

	t = time.Now().UTC()
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

func writeCorpusCSV(records [][]string, header []string, c *Corpus, dir string) error {
	records = append(records, header)
	for key, _ := range c.DHLabID {
		records = append(records, dhlab.PopulateCorpusRecord(key, c))
	}

	path := filepath.Join(dir, "outgoing", "corpus.csv")
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
	defer f.Close()

	return nil
}

func dhlabIDs(c *Corpus) []int {
	var ids []int

	for _, v := range c.DHLabID {
		ids = append(ids, v)
	}

	return ids
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

func main() {
	var args *Args
	var conc *Concordance
	var conf *Conf
	var corp *Corpus
	var err error

	flag.Parse()

	if directory == "" {
		fmt.Fprintln(os.Stderr, "Flag '-directory' must be set.")
		os.Exit(1)
	}

	if from < to {
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
			fmt.Fprintf(os.Stderr, "Error in readArgs():\n%v\n", err)
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

		// We can now dispense with the original path to the config file.
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

	// tag := &Tagging{}
	// id := &LanguageID{}
	// coll := &Collected{}

	// workflow_steps := []WorkflowStep{corp, conc, tag, id, coll}
	workflow_steps := []WorkflowStep{corp}
	for _, w := range workflow_steps {
		if !w.Finished() {

			err = w.Run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in %T.Run():\n%v\n", w, err)
				os.Exit(1)
			}
		}
	}

	// dhlabIDs := dhlabIDs(&corpus)
	// req, err = dhlab.ConcordanceRequest(&args, &conf, dhlabIDs)
	// if err != nil {
	//     fmt.Fprintf(os.Stderr, "Error in dhlab.ConcordanceRequest():\n%v\n", err)
	//     os.Exit(1)
	// }

	// err = dhlab.Concordance(req, &conc)
	// if err != nil {
	//     fmt.Fprintf(os.Stderr, "Error in dhlab.Concordance():\n%v\n", err)
	//     os.Exit(1)
	// }

	// fmt.Printf("%+v\n", conc)

	// concPath = filepath.Join(directory, "outgoing", "concordance.csv")
	// err = dhlab.WriteConcordance(corpusPath, concPath, &args, &conf)
}
