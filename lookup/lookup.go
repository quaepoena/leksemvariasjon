// Command leksemvariasjon-troubleshoot looks up a concordance line given a
// concordance JSON file, a DHLab ID, and in index number from a results file.
package main

import (
    "bufio"
    "errors"
    "path/filepath"
    "flag"
    "fmt"
    "io"
    "os"
    "strings"
)

// Flags.
var (
    input string
    tagged string
)

func init() {
    flag.StringVar(&input, "i", "", "Path to a an input file.")
    flag.StringVar(&tagged, "t", "tagged", "Path to the tagged/ directory.")
}

func errExit(s string, err error) {
    fmt.Fprintf(os.Stderr, "error in %s: %v\n", s, err)
    flag.Usage()
    os.Exit(1)
}

// Function parseLine populates a map after splitting a string on comma.
func parseLine(s string, l map[string][]string) error {
    var split []string
    var id, index string

    split = strings.Split(s, ",")
    if len(split) != 2 {
        return errors.New(fmt.Sprintf("each line must have only two comma-separated values, '%v' has %d\n", s, len(split)))
    }

    id = split[0]
    index = split[1]

    l[id] = append(l[id], index)

    return nil
}

// Function readFile opens a filepath and writes the contents to stdout.
func readFile(p string) ([]byte, error) {
    f, err := os.Open(p)
    if err != nil {
        return nil, errors.New(fmt.Sprintf("error in os.Open() with '%s': %v\n", p, err))
    }
    defer f.Close()

    b, err := io.ReadAll(f)
    if err != nil {
        return nil, errors.New(fmt.Sprintf("error in io.Reader() with '%s': %v\n", p, err))
    }

    return b, nil
}

func main() {
    lookup := make(map[string][]string)

    flag.Parse()

    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "\n%s usage:\n", os.Args[0])
        flag.PrintDefaults()
    }

    switch {
        case len(flag.Args()) > 0:
        for _, a := range flag.Args() {
            err := parseLine(a, lookup)
            if err != nil {
                errExit("parseLine()", err)
            }
        }

        case input == "":
        s := bufio.NewScanner(os.Stdin)
        for s.Scan() {
            err := parseLine(s.Text(), lookup)
            if err != nil {
                errExit("parseLine()", err)
            }
        }

        default:
        f, err := os.Open(input)
        if err != nil {
            errExit("os.Open()", err)
        }
        s := bufio.NewScanner(f)
        for s.Scan() {
            err := parseLine(s.Text(), lookup)
            if err != nil {
                errExit("parseLine()", err)
            }
        }
    }

    for k, v := range lookup {
        for _, i := range v {
            b, err := readFile(filepath.Join(tagged, k + "-" + i))
            if err != nil {
                errExit("readFile", err)
            }
            fmt.Printf("%s %s: %s\n", k, i, b)
        }
    }
}
