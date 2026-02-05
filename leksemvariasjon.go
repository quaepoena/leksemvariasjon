package main

import (
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

type Word struct {
    Form, Value string
    Morphology []string
}

type Lemma struct {
    Lemma string
    Words []Word
}

type Conf struct {
    Attribute, Language string
    Lemmas []Lemma
}

type Args struct {
    Config, Directory string
    Doctype string
    From int
    To int
}

// Flags.
var (
    config string
    directory string
    doctype string
    from int
    resume bool
    to int
)

func init() {
    flag.StringVar(&config, "config", "", "Path to a JSON config file. Required on an initial run.")
    flag.StringVar(&directory, "directory", "", "Directory to write files to, creating it if it doesn't exist.")
    flag.StringVar(&doctype, "doctype", "", "The doctype to search for.")
    flag.BoolVar(&resume, "resume", false, "Resume a previously started job.")
    flag.IntVar(&from, "from", 0, "The start year for the search.")
    flag.IntVar(&to, "to", 0, "The end year for the search (inclusive).")
}

func readArgs(path string, a *Args) error {
    var argFile *os.File
    var dec *gob.Decoder

    argFile, err := os.Open(path)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in os.Open(): %v\n", err))
    }
    defer argFile.Close()

    dec = gob.NewDecoder(argFile)
    err = dec.Decode(a)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in dec.Decode(): %v\n", err))
    }

    return nil
}

func mkUniqueDir(dir string, config string) (string, error) {
    var base, newDir, tStamp string
    var t time.Time

    t = time.Now().UTC()
    tStamp = t.Format(time.DateTime)
    base = filepath.Base(config)
    newDir = filepath.Join(dir,
        tStamp + "-" + strings.TrimSuffix(base, ".json"))
    err := os.MkdirAll(newDir, 0755)
    if err != nil {
        return "", errors.New(fmt.Sprintf("Error on os.MkdirAll(): %v\n", err))
    }

    return newDir, nil
}

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

func writeArgs(dir string, a *Args) error {
    var argFile *os.File
    var enc *gob.Encoder
    var path string

    path = filepath.Join(dir, "args.gob")
    argFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in os.OpenFile(): %v\n", err))
    }
    defer argFile.Close()

    enc = gob.NewEncoder(argFile)
    err = enc.Encode(a)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in enc.Encode(): %v\n", err))
    }

    return nil
}

func loadConf(path string, conf *Conf) error {
    var config_data []byte

    config_data, err := os.ReadFile(path)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in os.ReadFile(): %v\n", err))
    }

    err = json.Unmarshal(config_data, conf)
    if err != nil {
        return errors.New(fmt.Sprintf("Error in json.Unmarshal(): %v\n", err))
    }

    return nil
}

func createDirs(path string, dirs []string) error {
    for _, dir := range dirs {
        err := os.Mkdir(filepath.Join(path, dir), 0777)
        if err != nil {
            return errors.New(fmt.Sprintf("Error in os.Mkdir(): %v", err))
        }
    }

    return nil
}

func main() {
    var args Args
    var argPath string
	var conf Conf
	var processDirectories = []string{"incoming", "working", "outgoing", "results"}

    flag.Parse()

    if directory == "" {
        fmt.Fprintln(os.Stderr, "Flag '-directory' must be set.")
        os.Exit(1)
    }

    if resume == true && (
        config != "" || doctype != "" || from != 0 || to != 0) {
        fmt.Fprintln(os.Stderr, "Flag '-resume' isn't meant to be combined ",
            "with '-config', '-doctype', '-from' or '-to'.")
        os.Exit(1)
    }

    if from == 0 || to == 0 {
        fmt.Fprintln(os.Stderr, "Flags '-from' and '-to' must be set.")
        os.Exit(1)
    }

    if from < to {
        fmt.Fprintln(os.Stderr, "Flag '-to' must be greater than or equal to '-from'.")
        os.Exit(1)
    }

    if config == "" {
        fmt.Fprintln(os.Stderr, "Flag '-config' must be set when not using '-resume'.")
        os.Exit(1)
    }

    if doctype == "" {
        fmt.Fprintln(os.Stderr, "Flag '-doctype' must be set when not using '-resume'.")
        os.Exit(1)
    }

    if resume {
        // For resumptive runs we read the config info back from disk and set
        // the variables accordingly.
        argPath = filepath.Join(directory, "args.gob")
        err := readArgs(argPath, &args)
        if err != nil {
            panic(err)
        }

        config = args.Config
        doctype = args.Doctype
        from = args.From
        to = args.To
    } else {
        // For non-resumptive runs we need to 1) create a unique directory,
        // 2) copy the arguments and JSON config file thither, and 3) set
        // flag values appropriately.

        // The '-directory' flag is changed to the new, unique directory.
        directory, err := mkUniqueDir(directory, config)
        if err != nil {
            panic(err)
        }

        err = copyConfig(directory, config)
        if err != nil {
            panic(err)
        }

        // We can now dispense with the original path to the config file.
        config = filepath.Base(config)

        // "-directory" is a required flag, so it doesn't need to be saved.
        args = Args{Config: config, Doctype: doctype, From: from, To: to}
        err = writeArgs(directory, &args)
        if err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(1)
        }

		err = createDirs(directory, processDirectories)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in createDirs(): %v", err)
			os.Exit(1)
		}

		err = loadConf(filepath.Join(directory, config), &conf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in loadConf(): %+v\n", err)
			os.Exit(1)
		}
    }
}
