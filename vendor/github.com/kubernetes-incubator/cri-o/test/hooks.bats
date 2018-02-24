#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

cp hooks/checkhook.sh ${HOOKSDIR}
sed "s|HOOKSDIR|${HOOKSDIR}|" hooks/checkhook.json > ${HOOKSDIR}/checkhook.json

@test "pod test hooks" {
	rm -f /run/hookscheck
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
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stopp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl rmp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run cat /run/hookscheck
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}
