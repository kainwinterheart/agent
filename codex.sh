#!/bin/sh -e

container="${container:-docker.io/node:25-slim}" # $(podman build -f Dockerfile -q)
codex_root="${codex_root:-${HOME}/.codex}"

codex="${codex_interactive}"

if [ ! -z "${AC_AGENT_NAME}" ]; then
    codex="${codex_auto}"

    if [ "${AC_AGENT_NAME}" != 'coder' ] || [ ! -z "${isolate_coder}" ]; then
        spec=""
        spec="${spec} -v /tmp:/tmp:O"
        spec="${spec} -v ${HOME}:${HOME}:O"

        pwd_mount_suffix=''
        if [ "${AC_AGENT_NAME}" != 'coder' ]; then
            pwd_mount_suffix=':O'
        fi
        spec="${spec} -v ${PWD}:${PWD}${pwd_mount_suffix}"

        spec="${spec} -v $codex_root:$codex_root"

        codex="podman run -e AC_AGENT_NAME=${AC_AGENT_NAME} -e HOME=${HOME} --rm ${spec} -i ${container} ${codex}"
    fi
fi

set -x
exec $codex --cd ${PWD} "$@"
