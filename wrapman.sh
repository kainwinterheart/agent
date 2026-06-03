#!/bin/sh -e

wrapman_pause="${wrapman_pause:-10}"

lockfile="$1"
shift

container_name="$1"
shift

nohup /bin/sh -c "sleep ${wrapman_pause} ; flock ${lockfile} podman stop ${container_name} ; unlink ${lockfile}" > /dev/null &

exec flock "${lockfile}" podman run --name="${container_name}" "$@"
