#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "ctr termination reason Completed" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config_selinux.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_redis.json "$TESTDATA"/sandbox_config_selinux.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}
