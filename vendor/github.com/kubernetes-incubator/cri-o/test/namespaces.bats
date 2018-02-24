#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

function pid_namespace_test() {
	start_crio

	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_redis.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	[ "$status" -eq 0 ]

	run crictl exec --sync "$ctr_id" cat /proc/1/cmdline
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "${EXPECTED_INIT:-redis}" ]]

	run crictl stopp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl rmp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "pod disable shared pid namespace" {
	ENABLE_SHARED_PID_NAMESPACE=false pid_namespace_test
}

@test "pod enable shared pid namespace" {
	ENABLE_SHARED_PID_NAMESPACE=true EXPECTED_INIT=pause pid_namespace_test
}
