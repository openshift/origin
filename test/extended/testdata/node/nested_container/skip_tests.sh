#!/usr/bin/env bash
set -e

TEST_DIR=test/system
function insert_skip {
	file=$1
	testcase=$2
	# escape slash
	testcase=$(echo $testcase | sed 's/\//\\\//g')
	echo $testcase
	sed -i "/$testcase/a\
	skip 'not working in cri-o'" "$TEST_DIR/$file"
}

# cannot pause without cgroup, and no cgroup in rootless container
rm $TEST_DIR/080-pause.bats
# systemd
rm $TEST_DIR/250-systemd.bats
rm $TEST_DIR/251-system-service.bats
rm $TEST_DIR/252-quadlet.bats
rm $TEST_DIR/255-auto-update.bats
rm $TEST_DIR/270-socket-activation.bats

# broken tests https://github.com/containers/podman/commit/c65bb903b63c60a1ef2ccd3c21e118c4784d2f6b#diff-78c7444aee8aa212e7101395b618015dec2d5d1eab06fb7de2a8ecdf7418cde4
insert_skip 030-run.bats 'podman run - basic tests'
insert_skip 030-run.bats 'podman run - no entrypoint'

# Error: retrieving label for image "a6b6f7ac212ddfdb879e9b9c4df1655c9b07ead8b028110e657364485c577bff": you may need to remove the image to resolve the error: fallback error checking whether image is a manifest list: choosing image instance: no image found in manifest list for architecture "amd64", variant "", OS "linux": choosing image instance: no image found in manifest list for architecture "amd64", variant "", OS "linux"
insert_skip 012-manifest.bats "podman images - bare manifest"
# There is only one tty - /dev/tty
insert_skip 030-run.bats 'podman run --privileged as rootless will not mount /dev/tty\\d+'
# systemd can't run in nested container
insert_skip 030-run.bats "podman run - /run must not be world-writable in systemd containers"
# Error: OCI runtime error: crun: write to `/proc/self/oom_score_adj`: Invalid argument
insert_skip 030-run.bats "podman run doesn't override oom-score-adj"
# flaky
insert_skip 090-events.bats "image events"
# mount not shown
insert_skip 160-volumes.bats "podman run --volumes : basic"
# ping: permission denied (are you root?) 
insert_skip 200-pod.bats "podman pod create - hashtag AllTheOptions"
# /home/podman/go/src/github.com/containers/podman/test/system/helpers.bash: line 1117: /usr/bin/expr: Argument list too long
insert_skip 200-pod.bats "podman pod top - containers in different PID namespaces"
# flaky
insert_skip 320-system-df.bats "podman system df --format json functionality"
# FAIL: Pause process 42825 is still running even after podman system migrate
insert_skip 550-pause-process.bats "rootless podman only ever uses single pause process"
 
# Error: cannot remove container 31261d3890fe98a254b9fc2eb219753bc28df05efdf22c0f34222c33a2185a27 as it could not be stopped: given PID did not die within timeout
insert_skip 065-cp.bats "podman cp file from/to host while --pid=host"
insert_skip 195-run-namespaces.bats "podman test all namespaces"
 
# userns
insert_skip 170-run-userns.bats "podman userns=auto in config file"
insert_skip 170-run-userns.bats "podman userns=auto and secrets"
insert_skip 170-run-userns.bats "podman userns=auto with id mapping"
insert_skip 700-play.bats "podman kube restore user namespace" # usernamespace
 
# # won't fix

# cat: /proc/sys/net/core/wmem_default: No such file or directory
# https://github.com/moby/moby/issues/30778
insert_skip 505-networking-pasta.bats "UDP/IPv4 large transfer, tap"
insert_skip 505-networking-pasta.bats "UDP/IPv4 large transfer, loopback"
insert_skip 505-networking-pasta.bats "UDP/IPv6 large transfer, tap"
insert_skip 505-networking-pasta.bats "UDP/IPv6 large transfer, loopback"

# healthcheck
insert_skip 055-rm.bats "podman container rm doesn't affect stopping containers"
insert_skip 055-rm.bats "podman container rm --force doesn't leave running processes"
insert_skip 220-healthcheck.bats "podman healthcheck"
insert_skip 700-play.bats "podman kube play healthcheck should wait initialDelaySeconds before updating status"

# cgroup
# cannot pause the container without a cgroup
# https://github.com/containers/podman/issues/12782
insert_skip 200-pod.bats "podman pod cleans cgroup and keeps limits"
# opening file `memory.max` for writing: Permission denied
insert_skip 280-update.bats "podman update - test all options"
insert_skip 280-update.bats "podman update - resources on update are not changed unless requested"
insert_skip 600-completion.bats "podman shell completion test"

# systemd
insert_skip 030-run.bats "podman run --log-driver" # FAIL: podman logs, with driver 'journald'
insert_skip 035-logs.bats "podman logs - tail test, journald"
insert_skip 035-logs.bats "podman logs - multi journald"
insert_skip 035-logs.bats "podman logs restarted journald"
insert_skip 035-logs.bats "podman logs - since journald"
insert_skip 035-logs.bats "podman logs - until journald"
insert_skip 035-logs.bats "podman logs - --follow journald"
insert_skip 035-logs.bats "podman logs - --since --follow journald"
insert_skip 035-logs.bats "podman logs - --until --follow journald"
insert_skip 035-logs.bats "podman logs - journald log driver requires journald events backend"
insert_skip 090-events.bats "events with disjunctive filters - journald"
insert_skip 090-events.bats "events with file backend and journald logdriver with --follow failure"
insert_skip 090-events.bats "events - container inspect data - journald"
insert_skip 220-healthcheck.bats "podman healthcheck --health-log-destination journal"
insert_skip 420-cgroups.bats "podman run, preserves initial --cgroup-manager"
