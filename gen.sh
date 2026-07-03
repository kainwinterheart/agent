#!/bin/sh -e

generator_version="63126051b4386a8de00bb5a4a501b362f43b3beb"
generator="go-jsonschema --disable-omitempty"

export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin

SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

cd "${SCRIPT_DIR}"

exporter_bin="./schema_exporter"
gen_dir="./gen"

mkdir -p "${gen_dir}"

for f in ${gen_dir}/*.go; do
    unlink "${f}" ||:
done

while true; do
    sleep 1
    go install "github.com/kainwinterheart/go-jsonschema@${generator_version}" || continue
    break
done

go build -o "${exporter_bin}" ./cmd/schema_exporter/

schema_dir="$(mktemp -d)"

"${exporter_bin}" "${schema_dir}"

for f in ${schema_dir}/*.json; do
    $generator -p dt "${f}" > "${gen_dir}/$(basename "${f}" | sed -e 's/json$/go/')"
    unlink "${f}"
done

rm -r "${schema_dir}"

# state

state_dir="./state"
state_file="${state_dir}/state.go"

unlink "${state_file}" ||:

$generator -t -p state "${state_dir}/state.json" > "${state_file}"

state_header_length=$(grep -n ^package "${state_file}" | head -1 | awk -F: '{print $1}')
state_body_length=$(($(wc -l "${state_file}" | awk '{print $1}') - ${state_header_length}))
state_tmp="$(mktemp)"

head -n "${state_header_length}" "${state_file}" > "${state_tmp}"
echo 'import dt "agent-go/gen"' >> "${state_tmp}"
tail -n "${state_body_length}" "${state_file}" >> "${state_tmp}"

mv -v "${state_tmp}" "${state_file}"

# test_data

td_dir="./test_data"
td_file="${td_dir}/test_data.go"

unlink "${td_file}" ||:

$generator -t -p td "${td_dir}/test_data.json" > "${td_file}"

td_header_length=$(grep -n ^package "${td_file}" | head -1 | awk -F: '{print $1}')
td_body_length=$(($(wc -l "${td_file}" | awk '{print $1}') - ${td_header_length}))
td_tmp="$(mktemp)"

head -n "${td_header_length}" "${td_file}" > "${td_tmp}"
echo 'import state "agent-go/state"' >> "${td_tmp}"
tail -n "${td_body_length}" "${td_file}" >> "${td_tmp}"

mv -v "${td_tmp}" "${td_file}"
