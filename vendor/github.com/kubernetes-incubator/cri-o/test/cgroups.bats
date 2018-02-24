#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "pids limit" {
	if ! grep pids /proc/self/cgroup; then
		skip "pids cgroup controller is not mounted"
	fi
	PIDS_LIMIT=1234 start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	pids_limit_config=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin); obj["command"] = ["/bin/sleep", "600"]; json.dump(obj, sys.stdout)')
	echo "$pids_limit_config" > "$TESTDIR"/container_pids_limit.json
	run crictl create "$pod_id" "$TESTDIR"/container_pids_limit.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl exec --sync "$ctr_id" cat /sys/fs/cgroup/pids/pids.max
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "1234" ]]
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
