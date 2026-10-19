// Package process handles the tagging, filtering, and final output of the
// results from DHLAB.
package process

import (
    "bufio"
    "encoding/csv"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"

    "github.com/quaepoena/leksemvariasjon/config"
    "github.com/quaepoena/leksemvariasjon/dhlab"
    "k8s.io/apimachinery/pkg/util/sets"
)

// Struct Tag represents running the external tagger, which writes its results
// directly to disk. Therefore we don't need to keep any information concerning it.
type Tag struct {}

// Function prepareForTagging writes a file to disk for each concordance line.
func prepareForTagging(conc *dhlab.Concordance, p string) error {
    for i, v := range conc.Lines {
        for j, line := range v {
            name := filepath.Join(p,
                strconv.Itoa(i) + "-" + strconv.Itoa(j))
            err := os.WriteFile(name, []byte(line), 0666)
            if err != nil {
                return errors.New(fmt.Sprintf("error in os.WriteFile() with path %s: %v\n", name, err))
            }
        }

    }

    return nil
}

// Function tagFiles runs the tagger, an external Python program.
// During normal operation, stdout from the tagger shows where each tagged file
// was written to while stderr is empty.
func tagFiles(src string, p string) error {
    cmd := exec.Command("python", filepath.Join(src, "tagger.py"), p, p)
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return errors.New(fmt.Sprintf("error in cmd.StdoutPipe(): %v\n", err))
    }
    stderr, err := cmd.StderrPipe()
    if err != nil {
        return errors.New(fmt.Sprintf("error in cmd.StderrPipe(): %v\n", err))
    }

    if err := cmd.Start(); err != nil {
        return errors.New(fmt.Sprintf("error in cmd.Start(): %v\n", err))
    }

    tOut, err := io.ReadAll(stdout)
    if err != nil {
        return errors.New(fmt.Sprintf("error in io.ReadAll() (stdout): %v\n", err))
    }

    tErr, err := io.ReadAll(stderr)
    if err != nil {
        return errors.New(fmt.Sprintf("error in io.ReadAll() (stderr): %v\n", err))
    }

    if err := cmd.Wait(); err != nil {
        return errors.New(fmt.Sprintf("error in cmd.Wait(): %v\n", err))
    }

    fmt.Fprintf(os.Stdout, "%s\n", tOut)
    fmt.Fprintf(os.Stderr, "%s\n", tErr)

    return nil
}

// Run prepares the concordance lines for tagging and runs the tagger.
func (*Tag) Run(a *config.Args, _ *config.Conf) error {
    conc := &dhlab.Concordance{}

    p := filepath.Join(a.Directory, "tagged")
    err := os.MkdirAll(p, 0775)
    if err != nil {
        return errors.New(fmt.Sprintf("error in os.Mkdir(): %v\n", err))
    }

    err = config.Unmarshal(filepath.Join(a.Directory, "concordance.json"), conc)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Unmarshal(): %v\n", err))
    }

    err = prepareForTagging(conc, p)
    if err != nil {
        return errors.New(fmt.Sprintf("error in prepareForTagging(): %v\n", err))
    }

    err = tagFiles(a.Src, p)
    if err != nil {
        return errors.New(fmt.Sprintf("error in tagFiles(): %v\n", err))
    }

    err = os.WriteFile(filepath.Join(a.Directory, "taggingFinished.txt"), []byte{}, 0666)
    if err != nil {
        return errors.New(fmt.Sprintf("error in os.WriteFile(): %v\n", err))
    }

    return nil
}

func (*Tag) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "taggingFinished.txt"))
}

// Struct TaggedWord contains the word-specific tagging information.
type TaggedWord struct {
    Word string `json:"w"`
    Tags []string `json:"t"`
    Lemma string `json:"l"`
}

// Struct TaggedEntry contains language information and a list of tagged words.
// This is most often a whole sentence.
type TaggedEntry struct {
    Lang string
    TaggedWords []TaggedWord `json:"sent"`
}

// Struct MatchingWord contains information on tagged words that match the
// criteria given in the config file.
type MatchingWord struct {
    Form, Lang, Lemma, Value string
    Index int
}

// Struct Filter maps dhlabIDs to a list of words matching the criteria given in
// the config file.
type Filter struct {
    Words map[int][]MatchingWord
}

// Function dhlabID returns the dhlabID from a (tagged) filename.
func dhlabID(s string) (int, error) {
    re := regexp.MustCompile("^.+/([[:digit:]]+)-([[:digit:]]+).tagged$")
    id, err := strconv.Atoi(re.ReplaceAllString(s, "$1"))
    if err != nil {
        return 0, errors.New(fmt.Sprintf("error in strconv.Atoi(): %v\n", err))
    }

    return id, nil
}

// Function index returns the index in relation to each unique DHLab ID.
func index(s string) (int, error) {
    re := regexp.MustCompile("^.+/([[:digit:]]+)-([[:digit:]]+).tagged$")
    index, err := strconv.Atoi(re.ReplaceAllString(s, "$2"))
    if err != nil {
        return 0, errors.New(fmt.Sprintf("error in strconv.Atoi(): %v\n", err))
    }

    return index, nil
}

// Function matchingWords checks a tagged word against the criteria given in the
// config file. If everything matches then the word is appended to Filter.Words,
// a list of matching entries.
func matchingWords(entry *TaggedEntry, c *config.Conf, dhlabId int, index int, f *Filter) error {
    for _, lemma := range c.Lemmas {
        for _, word := range lemma.Words {
            for _, taggedWord := range entry.TaggedWords {

                if taggedWord.Lemma == lemma.Lemma &&
                    taggedWord.Word == word.Form &&
                    sets.New(taggedWord.Tags...).IsSuperset(
                        sets.New(word.Morphology...)) {

                    f.Words[dhlabId] = append(f.Words[dhlabId], MatchingWord{
                        Index: index,
                        Form: taggedWord.Word,
                        Lang: entry.Lang,
                        Lemma: taggedWord.Lemma,
                        Value: word.Value})
                }
            }
        }
    }

    return nil
}

// Run reads through the tagged files and finds the words matching the criteria
// given in the config file.
func (f *Filter) Run(a *config.Args, conf *config.Conf) error {
    f.Words = make(map[int][]MatchingWord)
    var tagged []string
    dir := filepath.Join(a.Directory, "tagged")

    files, err := os.ReadDir(dir)
    if err != nil {
        return errors.New(fmt.Sprintf("error in os.ReadDir(): %v\n", err))
    }

    for _, file := range files {
        if !file.IsDir() && strings.HasSuffix(file.Name(), ".tagged") {
            tagged = append(tagged, filepath.Join(dir, file.Name()))
        }
    }

    for _, t := range tagged {
        dhlabId, err := dhlabID(t)
        if err != nil {
            return errors.New(fmt.Sprintf("error in dhlabID(): %v\n", err))
        }

        index, err := index(t)
        if err != nil {
            return errors.New(fmt.Sprintf("error in index(): %v\n", err))
        }

        fd, err := os.Open(t)
        if err != nil {
            return errors.New(fmt.Sprintf("error in os.Open() with %s: %v\n",
                t, err))
        }

        s := bufio.NewScanner(fd)
        for s.Scan() {
            entry := &TaggedEntry{}
            err = json.Unmarshal(s.Bytes(), &entry)
            if err != nil {
                return errors.New(fmt.Sprintf("error in json.Unmarshal(): %v\n", err))
            }

            err = matchingWords(entry, conf, dhlabId, index, f)
            if err != nil {
                return errors.New(fmt.Sprintf("error in matchingWords(): %v\n", err))
            }
        }

        if err = s.Err(); err != nil {
            return errors.New(fmt.Sprintf("error while scanning %s: %v\n",
                t, err))
        }

        fd.Close()
    }

    err = config.Marshal(filepath.Join(a.Directory, "filter.json"), f)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Marshal() with filter.json: %v\n", err))
    }

    return nil
}

func (*Filter) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "filter.json"))
}

// Struct Occurrence contains all the collected information on a given word
// matching the criteria given in the config file.
type Occurrence struct {
    DhlabId, Year, Index int
    Doctype, Form, Lemma, LangDhlab, LangId, URN, Value string
}

// Struct Results contains a list of words matching the criteria given in the
// config file.
type Results struct {
    Occurrences []Occurrence
}

// Function record creates CSV records from an Occurrence struct.
func record(o *Occurrence) []string {
    var fields []string

    fields = append(fields, strconv.Itoa(o.DhlabId))
    fields = append(fields, strconv.Itoa(o.Index))
    fields = append(fields, o.URN)
    fields = append(fields, o.Doctype)
    fields = append(fields, strconv.Itoa(o.Year))
    fields = append(fields, o.Form)
    fields = append(fields, o.Lemma)
    fields = append(fields, o.Value)
    fields = append(fields, o.LangDhlab)
    fields = append(fields, o.LangId)

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

// Run merges the filtered results with the information from the (saved)
// dhlab.Corpus struct. The results are written to disk as both JSON and CSV.
func (c *Results) Run(a *config.Args, _ *config.Conf) error {
    corp := &dhlab.Corpus{}
    fil := &Filter{}

    err := config.Unmarshal(filepath.Join(a.Directory, "filter.json"), fil)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Unmarshal() with filter.json: %v\n", err))
    }

    err = config.Unmarshal(filepath.Join(a.Directory, "corpus.json"), corp)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Unmarshal() with corpus.json: %v\n", err))
    }

    for id, words := range fil.Words {
        for _, word := range words {
            c.Occurrences = append(c.Occurrences, Occurrence{
                DhlabId: id,
                Index: word.Index,
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

    err = config.Marshal(filepath.Join(a.Directory, "results.json"), c)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Marshal() with results.json: %v\n", err))
    }

    var records [][]string
    header := []string{"dhlabid", "index", "urn", "doctype", "year", "form",
        "lemma", "value", "lang_dhlab", "lang_id"}
    records = append(records, header)
    for _, c := range c.Occurrences {
        records = append(records, record(&c))
    }

    err = writeCsv(filepath.Join(a.Directory, "results.csv"), records)
    if err != nil {
        return errors.New(fmt.Sprintf("error in writeCsv(): %v\n", err))
    }

    return nil
}

func (*Results) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "results.csv"))
}
