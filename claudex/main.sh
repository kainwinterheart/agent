#!/bin/sh -e

SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

set -x
exec npm --silent --prefix "${SCRIPT_DIR}" run main -- "$@"
