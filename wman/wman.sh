#!/bin/sh -e

oldpwd="${PWD}"
SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

cd "${SCRIPT_DIR}"

if [ ! -e .venv ]; then
    uv venv -p 3.14
    uv pip install -r requirements.txt
fi

exec uv run ./wman.py "${oldpwd}" "$@"
