#!/usr/bin/env bats

load helpers

ROOT="$TESTDIR/crio"
RUNROOT="$TESTDIR/crio-run"
KPOD_OPTIONS="--root $ROOT --runroot $RUNROOT --storage-driver vfs"
function teardown() {
    cleanup_test
}

@test "stop a bogus container" {
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop foobar
    echo "$output"
    [ "$status" -eq 1 ]
}

@test "stop a running container by id" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    id="$output"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop "$id"
    cleanup_pods
    stop_crio
}

@test "stop a running container by name" {
    start_crio
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run crioctl ctr start --id "$ctr_id"
    echo "$output"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} stop "k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0"
    cleanup_pods
    stop_crio
}
