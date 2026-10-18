// Command leksemvariasjon displays lexeme variation over Norwegian texts.
// The user creates a configuration file specifying which words and
// morphological features he/she is interested in. The National Library of
// Norway's DHLAB API is queried to find concordance lines with these words
// which are then tagged and the results are filtered and put into a CSV.
// See README.md for more information and usage examples.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quaepoena/leksemvariasjon/config"
	"github.com/quaepoena/leksemvariasjon/dhlab"
	"github.com/quaepoena/leksemvariasjon/process"
)

// Flags.
var (
	concLimit  int
	configFile string
	corpLimit  int
	directory  string
	doctype    string
	from       int
	resume     bool
	to         int
)

func init() {
	flag.IntVar(&concLimit, "concLimit", 999999, "Limit the numbers of Concordance results.")
	flag.StringVar(&configFile, "configFile", "", "Path to a JSON config file. Required on an initial run.")
	flag.IntVar(&corpLimit, "corpLimit", 999999, "Limit the numbers of Corpus results.")
	flag.StringVar(&directory, "directory", "", "Directory to write files to, creating it if it doesn't exist.")
	flag.StringVar(&doctype, "doctype", "", "The doctype to search for.")
	flag.BoolVar(&resume, "resume", false, "Resume a previously started job.")
	flag.IntVar(&from, "from", 0, "The start year for the search.")
	flag.IntVar(&to, "to", 0, "The end year for the search (inclusive).")
}

// Function uniqueDir makes a unique output directory for each (non-resumptive)
// run of the program.
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

// Function copyConfig copies the configuration file to the newly created output
// directory.
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

func main() {
	args := &config.Args{}
	conf := &config.Conf{}
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

	if resume == true && (configFile != "" || doctype != "" || from != 0 || to != 0) {
		fmt.Fprintln(os.Stderr, "Flag '-resume' isn't meant to be combined ",
			"with '-configFile', '-doctype', '-from' or '-to'.")
		os.Exit(1)
	}

	if resume == false && (configFile == "" || doctype == "" || from == 0 || to == 0) {
		fmt.Fprintln(os.Stderr, "Flags '-configFile', '-doctype', '-from', and '-to' must be set when not using '-resume'.")
		os.Exit(1)
	}

	if resume {
		// For resumptive runs we read the arguments back from disk and set
		// the variables accordingly.
		err = config.Unmarshal(filepath.Join(directory, "args.json"), args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in config.Unmarshal(): %v\nThis is a resumptive run. Did you specify the already-existing output directory from a previous run?", err)
			os.Exit(1)
		}
	} else {
		// For non-resumptive runs we need to 1) create a unique directory,
		// 2) copy the arguments and JSON config file thither, and 3) set
		// flag values appropriately.

		// The '-directory' flag is changed to the new, unique directory.
		directory, err = uniqueDir(directory, configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in uniqueDir(): %v", err)
			os.Exit(1)
		}

		err = copyConfig(directory, configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in copyConfig(): %v", err)
			os.Exit(1)
		}

		// The '-configFile' flag, which (potentially) included directories,
		// is changed to the basename.
		configFile = filepath.Base(configFile)

		args.ConcLimit = concLimit
		args.CorpLimit = corpLimit
		args.ConfigFile = configFile
		args.Directory = directory
		args.Doctype = doctype
		args.From = from
		args.To = to
		err = config.Marshal(filepath.Join(directory, "args.json"), args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in config.Marshal(): %v", err)
			os.Exit(1)
		}
	}

	err = config.Unmarshal(filepath.Join(directory, args.ConfigFile), conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error in config.Unmarshal(): %v", err)
		os.Exit(1)
	}

	cor := &dhlab.Corpus{}
	con := &dhlab.Concordance{}
	t := &process.Tag{}
	f := &process.Filter{}
	col := &process.Collate{}

	stages := []config.WorkflowStage{cor, con, t, f, col}
	for _, s := range stages {
		if !s.Finished(args) {

			err = s.Run(args, conf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error in %T.Run(): %v", s, err)
				os.Exit(1)
			}
		}
	}
}
