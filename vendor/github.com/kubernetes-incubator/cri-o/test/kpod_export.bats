#!/usr/bin/env bats

load helpers

IMAGE="redis:alpine"
ROOT="$TESTDIR/crio"
RUNROOT="$TESTDIR/crio-run"
KPOD_OPTIONS="--root $ROOT --runroot $RUNROOT ${STORAGE_OPTS}"


@test "kpod export output flag" {
    start_crio
    [ "$status" -eq 0 ]
    run crioctl pod run --config "$TESTDATA"/sandbox_config.json
    echo "$output"
    [ "$status" -eq 0 ]
    pod_id="$output"
    run crioctl image pull "$IMAGE"
    [ "$status" -eq 0 ]
    run crioctl ctr create --config "$TESTDATA"/container_config.json --pod "$pod_id"
    echo "$output"
    [ "$status" -eq 0 ]
    ctr_id="$output"
    run ${KPOD_BINARY} ${KPOD_OPTIONS} export -o container.tar "$ctr_id"
    echo "$output"
    [ "$status" -eq 0 ]
    cleanup_ctrs
    [ "$status" -eq 0 ]
    cleanup_pods
    [ "$status" -eq 0 ]
    stop_crio
    [ "$status" -eq 0 ]
    rm -f container.tar
    [ "$status" -eq 0 ]
}
