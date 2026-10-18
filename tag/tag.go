// Command leksemvariasjon displays lexeme variation over Norwegian texts.
// The user creates a configuration file specifying which words and
// morphological features he/she is interested in. The National Library of
// Norway's DHLAB API is queried to find concordance lines with these words
// which are then tagged and the results are filtered and put into a CSV.
// See README.md for more information and usage examples.
package tag

import (
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "time"

    "github.com/quaepoena/leksemvariasjon/config"
    "github.com/quaepoena/leksemvariasjon/dhlab"
)

// Struct Tag represents running the external tagger, which writes its results
// directly to disk. Therefore we don't need to keep any information concerning it.
type Tag struct {}

// Function writeFilesForTagging writes a file to disk for each concordance line.
func writeFilesForTagging(conc *dhlab.Concordance, p string) error {
    for i, v := range conc.Lines {
        for _, line := range v {
            name := filepath.Join(p,
                strconv.Itoa(i) + "-" +  strconv.FormatInt(time.Now().UTC().UnixMicro(), 10))
            err := os.WriteFile(name, []byte(line), 0666)
            if err != nil {
                return errors.New(fmt.Sprintf("error in os.WriteFile() with path %s: %v", name, err))
            }
        }

    }

    return nil
}

// Function tagFiles runs the tagger, an external Python program.
// During normal operation, stdout from the tagger shows where each tagged file
// was written to while stderr is empty.
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

    fmt.Fprintf(os.Stdout, "%s\n", tOut)
    fmt.Fprintf(os.Stderr, "%s\n", tErr)

    return nil
}

// Run prepares the concordance lines for tagging and runs the tagger.
func (*Tag) Run(a *config.Args, _ *config.Conf) error {
    var conc *dhlab.Concordance = &dhlab.Concordance{}

    p := filepath.Join(a.Directory, "tagged")
    err := os.MkdirAll(p, 0775)
    if err != nil {
        return errors.New(fmt.Sprintf("error in os.Mkdir(): %v\n", err))
    }

    err = config.Unmarshal(filepath.Join(a.Directory, "concordance.json"), conc)
    if err != nil {
        return errors.New(fmt.Sprintf("error in config.Unmarshal(): %v", err))
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

func (*Tag) Finished(a *config.Args) bool {
    return config.FileExists(filepath.Join(a.Directory, "taggingFinished.txt"))
}
