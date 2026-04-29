#!/bin/sh -e

codex="${codex_interactive}"

if [ ! -z "${AC_AGENT_NAME}" ]; then
    codex="${codex_auto}"

    if [ "${AC_AGENT_NAME}" != 'coder' ]; then
        spec=""
        spec="${spec} -v /tmp:/tmp:O"
        spec="${spec} -v ${HOME}:${HOME}:O"
        spec="${spec} -v ${PWD}:${PWD}:O"

        x="${HOME}/.codex"
        spec="${spec} -v $x:$x"

        container="docker.io/node:25-slim" # $(podman build -f Dockerfile -q)
        codex="podman run -e HOME=${HOME} --rm ${spec} -i ${container} ${codex} --cd ${PWD}"
    fi
fi

set -x
exec $codex "$@"
