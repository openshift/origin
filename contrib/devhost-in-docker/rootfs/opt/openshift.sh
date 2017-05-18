#!/bin/bash -eu

. /opt/env.sh

fatal() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

cd "$OPENSHIFT_CONFIG_PATH"

case "$0" in
	*-master)
			openshift start master \
				--loglevel=8
		;;
	*-node)
		. /opt/docker.sh

		num=0
		node_path=
		for n in openshift.local.config/node-*; do
			[ -d "$n" ] ||
				fatal "Failed to find configuration for node: $n"
			
			if ! mkdir -- "$n/.lock" 2>/dev/null; then
				num=$(($num+1))
				continue
			fi

			node_path="$n"
			break
		done

		[ -n "$node_path" ] ||
				fatal 'All node already active'

		trap "rmdir -- $node_path/.lock" EXIT HUP PIPE INT QUIT TERM

		openshift start node \
			--loglevel=8 \
			--config="$node_path"/node-config.yaml
		;;
	*)
		/bin/bash
		;;
esac
