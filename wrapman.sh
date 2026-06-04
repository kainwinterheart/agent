#!/bin/sh -e

lockfile="$1"
shift

container_name="$1"
shift

wrapman_pause="${wrapman_pause:-10}"
timeout_cmd=${timeout_cmd:-$(pstree -A -a -s -l $$ |grep -oE "\btimeout\s+(-s\s+[0-9A-Z]+\s+)?[0-9a-zA-Z]+" | tail -1)}

setsid -f /bin/sh -c "sleep ${wrapman_pause} ; flock ${lockfile} podman stop ${container_name} ; unlink ${lockfile}" &>/dev/null

if [ -t 1 ]; then
    if [ ! -z "${timeout_cmd}" ]; then
        timeout_cmd=$(echo "${timeout_cmd}" | sed -e 's/^timeout/timeout --foreground/')
    fi
fi

exec flock "${lockfile}" ${timeout_cmd} podman run --name="${container_name}" "$@"
