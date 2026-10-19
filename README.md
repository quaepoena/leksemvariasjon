# leksemvariasjon

`leksemvariasjon` creates a report on lexeme variation in Norwegian texts. The
data comes from the National Library of Norway's [DHLab
API](https://api.nb.no/dhlab)
([code](https://github.com/NationalLibraryOfNorway/DHLAB)), and words matching
the given lemma, word form, and morphological criteria (on which, below) are
written to a CSV. Morphological tagging is provided by the tagger
([code](https://github.com/humit-oslo/humit-tagger-sources),
[HuggingFace](https://huggingface.co/Humit-Oslo/humit-tagger-xs)) written by
[Humit UiO](https://www.hf.uio.no/humit/english/about/).

Given the various external dependencies with their various error modes, the
program runs in a sequential series of steps that ensure the previous step has
completed successfully before the next is run. This is done very simply through
writing files to disk and then checking for their existence at the beginning of
the next step. During normal operation all this is transparent to the user, but
these files can also be used to troubleshoot unexpected results. This behavior
can also be used, with the `-resume` flag (described below), to resume a
previous run.

Please don't hesitate to open an issue with questions, bugs, and/or feature
requests.

## Setup

1. Get the code:

```
git clone https://github.com/quaepoena/leksemvariasjon.git && cd leksemvariasjon/
```

2. Install Go. Using UiO's educloud, load the `Go/1.25.0` module:

`module load Go/1.25.0`

3. Build and/or install the binary (according to your preference, see [this page
on building and installing Go
programs](https://go.dev/doc/tutorial/compile-install) for more information).

`go build leksemvariasjon.go`

`go install`

4. (Optional) It is likely helpful to run the program inside a Python virtual
environment where you have ensured the tagger works properly (see the
[HuggingFace page](https://huggingface.co/Humit-Oslo/humit-tagger-xs) for
example usage). The needed packages are specified in `requirements.txt`, though
please do report if this doesn't work for you.

## Usage

```
leksemvariasjon usage:
  -concLimit int
        Limit the numbers of Concordance results. (default 999999)
  -configFile string
        Path to a JSON config file. Required on an initial run.
  -corpLimit int
        Limit the numbers of Corpus results. (default 999999)
  -directory string
        Directory to write files to, creating it if it doesn't exist.
  -doctype string
        The doctype to search for.
  -from int
        The start year for the search.
  -resume
        Resume a previously started job.
  -src string
        The source directory for leksemvariasjon.
  -to int
        The end year for the search (inclusive).
```

The `-directory` flag is always required. On an initial run, output
files are written to a directory created under this directory. `-from`
and `-to` specify an inclusive range of years. `-doctype` searches for
a given document type, [as defined by
DHLab](https://github.com/NationalLibraryOfNorway/DHLAB/blob/b69775f4fa54d75855685e3c1c761c92d62442b4/dhlab/text/corpus.py#L24),
though it can be blank. `-configFile` is the path to the JSON config
file (see below). `-corpLimit` and <br> `-confLimit` set limits on,
respectively, the number of results from DHLab's `build_corpus` and
`conc` API calls. The source directory needs to be specified with
`-src` so that the program knows where the tagger (wrapper) is
located. The usage of `-resume` is described below.

### Resuming a previous run

To resume a previous run, set `-resume` and give `-directory` as the
newly-created directory which contains the files specific to a given run, not
the parent directory specified on the initial run.

## Configuration file

The configuration file is a JSON file used to specify which word forms the user
wishes to see occurrences of. Two example configuration files are given below.
The first one can be used for measuring the occurrence of definite plural forms
in `-a` or `-ene` in Bokmål for the lemma "år".

```json
{
  "attribute": "-a/-ene i inkjekjønnssubstantiv",
  "language": "nob",
  "lemmas": [
    {
      "lemma": "år",
      "words": [
        {"form": "åra", "value": "a", "morphology": ["subst"]},
        {"form": "årene", "value": "ene", "morphology": ["subst"]}
      ]
    }
  ]
}
```

The most important fields are `lemma`, `form` and `morphology` because the
results from the tagger are filtered according to these values. `language` is
specified in DHLab's `build_corpus` call. The remaining fields, `attribute` and
`value` are for the user's benefit to help with classifying the results
afterward.

- `attribute`: A user-defined name for the given grammatical feature being researched.
- `language`: `nob` or `nno`.
- `lemmas`: A list of lemmas. (An example with more than one is shown below.)
- `lemma`: The lemma to match against (as reported by the tagger).
- `words`: A list of word forms for a given lemma that demonstrate the given feature.
- `form`: The word form as it actually exists in a text.
- `value`: A user-defined value for classifying results. While most varying
  grammatical features will have two possible values, there is no theoretical
  limit here.
- `morphology`: A set of morphological features to be matched against the
  results from the tagger. A given tagged word is included if its features are a
  superset of these features. **Note**: These sets are created from lists, so even
  if there is only one feature to be matched against, it must be specified in a
  list of strings, as shown above, and not as a simple string.

As an additional example, to search for occurrences of palatalization in
Nynorsk, one could use the following configuration file:

```json
{
  "attribute": "Former med og utan j etter g og k",
  "language": "nno",
  "lemmas": [
    {
      "lemma": "bygge",
      "words": [
        {"form": "byggja", "value": "j", "morphology": ["verb"]},
        {"form": "byggje", "value": "j", "morphology": ["verb"]},
        {"form": "bygga", "value": "Ø", "morphology": ["verb"]},
        {"form": "bygge", "value": "Ø", "morphology": ["verb"]},
        {"form": "byggjer", "value": "j", "morphology": ["verb"]},
        {"form": "bygger", "value": "Ø", "morphology": ["verb"]}
      ]
    },
    {
      "lemma": "legge",
      "words": [
        {"form": "leggja", "value": "j", "morphology": ["verb"]},
        {"form": "leggje", "value": "j", "morphology": ["verb"]},
        {"form": "legga", "value": "Ø", "morphology": ["verb"]},
        {"form": "legge", "value": "Ø", "morphology": ["verb"]}
      ]
    }
  ]
}
```

## Manually resuming the tagger

If a run were to fail in the middle of tagging, e.g. due to a time or memory
limit being exceeded on slurm, it is recommended to run the `restart_tagger.sh`
script provided in the repo to restart the tagger and rerun `leksemvariasjon`.
This prevents having to start the tagging from the beginning and thereby losing
a (potentially) significant amount of time. Otherwise manual moving of
already-tagged files is needed so as to not corrupt the output.

## Tips

- Be careful to specify the desired morphological features correctly. Homonyms
might be included if the allowed values are too inclusive.

- If you see that your process is being killed without much explanation, it
might be due to not having enough memory. Try increasing the allotted memory if
running on slurm.

### Lemma normalization

Note that the tagger normalizes lemmas to e-infinitives in Nynorsk.

## Misc

### Suppress verbose output

With the exception of the tagger, this program doesn't produce output when
everything functions properly. If one wishes to suppress the output from the
tagger, which might overflow the terminal scrollback with messages about files
being tagged, simply redirect stdout:

`leksemvariasjon … 1>/dev/null`

### Troubleshooting

A helper program (in the works) will be written to enable easy look-up
interesting/suspicious concordance lines.
