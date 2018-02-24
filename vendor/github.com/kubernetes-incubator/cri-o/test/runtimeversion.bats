#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "crictl runtimeversion" {
	start_crio
	run crictl info
	echo "$output"
	[ "$status" -eq 0 ]
	stop_crio
}
