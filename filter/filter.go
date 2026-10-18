// Package filter filters the tagged results after lemma, word form, and
// morphological features, as defined in the configuration file.
package filter

import (
    "bufio"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"

    "k8s.io/apimachinery/pkg/util/sets"
    "github.com/quaepoena/leksemvariasjon/config"
)

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
}

// Struct Filter maps dhlabIDs to a list of words matching the criteria given in
// the config file.
type Filter struct {
    Words map[int][]MatchingWord
}

// Function dhlabID returns the dhlabID from a (tagged) filename.
func dhlabID(s string) (int, error) {
    re := regexp.MustCompile("^/.+/([[:digit:]]+)-.+$")
    id, err := strconv.Atoi(re.ReplaceAllString(s, "$1"))
    if err != nil {
        return 0, errors.New(fmt.Sprintf("error in strconv.Atoi(): %v", err))
    }

    return id, nil
}

// Function matchingWords checks a tagged word against the criteria given in the
// config file. If everything matches then the word is appended to fil, a list
// of matching entries.
func matchingWords(entry *TaggedEntry, conf *config.Conf, dhlabId int, fil *Filter) error {
    for _, lemma := range conf.Lemmas {
        for _, word := range lemma.Words {
            for _, taggedWord := range entry.TaggedWords {

                if taggedWord.Lemma == lemma.Lemma &&
                    taggedWord.Word == word.Form &&
                    sets.New(taggedWord.Tags...).IsSuperset(
                        sets.New(word.Morphology...)) {

                    fil.Words[dhlabId] = append(fil.Words[dhlabId], MatchingWord{
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

    err = config.Marshal(filepath.Join(a.Directory, "filter.json"), f)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Marshal() with filter.json: %v", err))
    }

    return nil
}

func (*Filter) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "filter.json"))
}
