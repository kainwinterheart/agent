#!/bin/sh -e

export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin

SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

cd "${SCRIPT_DIR}"

exporter_bin="./schema_exporter"
gen_dir="./gen"

mkdir -p "${gen_dir}"

for f in ${gen_dir}/*.go; do
    unlink "${f}" ||:
done

go install github.com/kainwinterheart/go-jsonschema@e6b713ed30e4f0882c175345aedb6fe3fe1ee29f

go build -o "${exporter_bin}" ./cmd/schema_exporter/

schema_dir="$(mktemp -d)"

"${exporter_bin}" "${schema_dir}"

for f in ${schema_dir}/*.json; do
    go-jsonschema -p dt "${f}" > "${gen_dir}/$(basename "${f}" | sed -e 's/json$/go/')"
    unlink "${f}"
done

rm -r "${schema_dir}"
