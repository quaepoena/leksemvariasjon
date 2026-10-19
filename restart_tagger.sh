#!/bin/bash


function tagger_restart::move_files() {
    local run_dir="$1"

    ls -d "${run_dir}/tagged/" &>/dev/null
    if [[ "$?" -ne 0 ]]; then
        echo "No tagged/ directory found in ${run_dir}. Was correct directory specified or has the tagger run?" >&2
        return 1
    fi

    mkdir -p "${run_dir}/tmp/"
    for i in $(find "${run_dir}/tagged/" -type f -name '*.tagged'); do
        mv "${i}" "${run_dir}/tmp/"
        mv "${i%.tagged}" "${run_dir}/tmp/"
    done

    return 0
}


function tagger_restart::run_tagger() {
    local run_dir="$1"
    local src_dir="$2"

    ls "${src_dir}/tagger.py" &>/dev/null
    if [[ "$?" -ne 0 ]]; then
        echo "tagger.py not found in ${src_dir}. Was the correct path specified?" >&2
        return 1
    fi

    python "${src_dir}/tagger.py" "${run_dir}/tagged" "${run_dir}/tagged"

    if [[ "$?" -eq 0 ]]; then
        mv "${run_dir}/tmp/"* "${run_dir}/tagged/"
        touch "${run_dir}/taggingFinished.txt"
    else
        echo "The tagger failed." >&2
        return 1
    fi

    return 0
}


function tagger_restart::run_leksemvariasjon() {
    local run_dir="$1"
    local src_dir="$2"

    if ! which leksemvariasjon &>/dev/null; then
        echo "leksemvariasjon not found in \$PATH. Run manually." >&2
        return 1
    fi

    leksemvariasjon -resume -directory "${run_dir}" -src "${src_dir}"

    if [[ "$?" -ne 0 ]]; then
        echo "leksemvariasjon failed." >&2
        return 1
    fi

    return 0
}

function tagger_restart::main() {
    local run_dir="$1"
    local src_dir="$2"

    if [[ "$#" -ne 2 ]]; then
        echo "Usage: $0 <run-specific diretory> <source directory>"
        exit 1
    fi

    tagger_restart::move_files "${run_dir}"
    if [[ "$?" -ne 0 ]]; then
        echo "Exiting." >&2
        exit 1
    fi

    tagger_restart::run_tagger "${run_dir}" "${src_dir}"
    if [[ "$?" -ne 0 ]]; then
        echo "Exiting." >&2
        exit 1
    fi

    tagger_restart::run_leksemvariasjon "${run_dir}" "${src_dir}"
    if [[ "$?" -ne 0 ]]; then
        echo "leksemvariasjon didn't run properly." >&2
        echo "Exiting." >&2
        exit 1
    fi

    rm -r "${run_dir}/tmp/"

    return 0
}

tagger_restart::main "$@"
