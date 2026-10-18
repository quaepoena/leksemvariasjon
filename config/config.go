// Package config deals with saving and reading the various options specified on
// the command-line and in the configuration file. A few often-used helper
// functions are located here as well.
package config

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
)

// Struct Args holds command line arguments and saves/reads them to/from disk.
type Args struct {
    ConfigFile, Directory, Doctype string
    ConcLimit, CorpLimit, From, To int
}

// Struct Word contains word forms with the corresponding morphological
// information the user wants to search for.
type Word struct {
    Form, Value string
    Morphology []string
}

// Struct Lemma holds a lemma and all the word forms under it.
type Lemma struct {
    Lemma string
    Words []Word
}

// Struct Conf holds the parsed information from the config.
type Conf struct {
    Attribute, Language string
    Lemmas []Lemma
}

// Interface WorkflowStage represents the similarly structured tasks for each
// step of the program.
type WorkflowStage interface {
    Run(*Args, *Conf) error
    Finished(*Args) bool
}

func FileExists(s string) bool {
    f,  err := os.Open(s)
    if err != nil {
        return false
    }
    defer f.Close()

    return true
}

func Unmarshal(p string, s any) error {
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

func Marshal(p string, s any) error {
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
