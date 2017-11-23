#!/usr/bin/env bats

load helpers


IMAGE="docker.io/library/alpine:latest"
ROOT="$TESTDIR/crio"
RUNROOT="$TESTDIR/crio-run"
KPOD_OPTIONS="--root $ROOT --runroot $RUNROOT --storage-driver vfs"

function teardown() {
    cleanup_test
}

@test "kpod tag with shortname:latest" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} tag $IMAGE foobar:latest
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} inspect foobar:latest
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi foobar:latest
	[ "$status" -eq 0 ]
}

@test "kpod tag with shortname" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} tag $IMAGE foobar
	run ${KPOD_BINARY} ${KPOD_OPTIONS} inspect foobar:latest
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi foobar:latest
	[ "$status" -eq 0 ]
}

@test "kpod tag with shortname:tag" {
	run ${KPOD_BINARY} ${KPOD_OPTIONS} pull $IMAGE
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} tag $IMAGE foobar:v
	run ${KPOD_BINARY} ${KPOD_OPTIONS} inspect foobar:v
	echo "$output"
	[ "$status" -eq 0 ]
	run ${KPOD_BINARY} ${KPOD_OPTIONS} rmi foobar:v
	[ "$status" -eq 0 ]
}
