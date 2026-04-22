#!/bin/sh -e

codex=$1
shift

if [ ! -z "${AC_AGENT_NAME}" ]; then
    if [ "${AC_AGENT_NAME}" != 'coder' ]; then
        if [ "$1" != 'exec' ]; then
            echo 'Only `codex exec` is supported' >&2
            exit 1
        fi
        shift
        set -x
        exec $codex --ask-for-approval never exec --sandbox read-only "$@"
    fi
fi

set -x
exec $codex "$@" --yolo
