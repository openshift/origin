#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "crio restore" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crictl pods --quiet --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	pod_list_info="$output"

	run crictl inspectp "$pod_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	pod_status_info=`echo "$output" | grep Status`

	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl ps --quiet --id "$ctr_id" --all
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_list_info="$output"

	run crictl inspect "$ctr_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_status_info=`echo "$output" | grep State`

	stop_crio

	start_crio
	run crictl pods --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" != "" ]]
	[[ "${output}" == "${pod_id}" ]]

	run crictl pods --quiet --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" == "${pod_list_info}" ]]

	run crictl inspectp "$pod_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	output=`echo "$output" | grep Status`
	[[ "${output}" == "${pod_status_info}" ]]

	run crictl ps --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" != "" ]]
	[[ "${output}" == "${ctr_id}" ]]

	run crictl ps --quiet --id "$ctr_id" --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" == "${ctr_list_info}" ]]

	run crictl inspect "$ctr_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	output=`echo "$output" | grep State`
	[[ "${output}" == "${ctr_status_info}" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "crio restore with bad state and pod stopped" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crictl stopp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	stop_crio

	# simulate reboot with runc state going away
	for i in $("$RUNTIME" list -q | xargs); do "$RUNTIME" delete -f $i; done

	start_crio

	run crictl stopp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_pods
	stop_crio
}

@test "crio restore with bad state and ctr stopped" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl stop "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	stop_crio

	# simulate reboot with runc state going away
	for i in $("$RUNTIME" list -q | xargs); do "$RUNTIME" delete -f $i; done

	start_crio

	run crictl stop "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "crio restore with bad state and ctr removed" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl stop "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl rm "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	stop_crio

	# simulate reboot with runc state going away
	for i in $("$RUNTIME" list -q | xargs); do "$RUNTIME" delete -f $i; done

	start_crio

	run crictl stop "$ctr_id"
	echo "$output"
	[ "$status" -eq 1 ]
	[[ "${output}" =~ "not found" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "crio restore with bad state and pod removed" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crictl stopp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl rmp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	stop_crio

	# simulate reboot with runc state going away
	for i in $("$RUNTIME" list -q | xargs); do "$RUNTIME" delete -f $i; done

	start_crio

	run crictl stopp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_pods
	stop_crio
}

@test "crio restore with bad state" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	run crictl inspectp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" =~ "SANDBOX_READY" ]]

	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl inspect "$ctr_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" =~ "CONTAINER_CREATED" ]]

	stop_crio

	# simulate reboot with runc state going away
	for i in $("$RUNTIME" list -q | xargs); do "$RUNTIME" delete -f $i; done

	start_crio
	run crictl pods --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" != "" ]]
	[[ "${output}" =~ "${pod_id}" ]]

	run crictl inspectp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" =~ "SANDBOX_NOTREADY" ]]

	run crictl ps --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" != "" ]]
	[[ "${output}" =~ "${ctr_id}" ]]

	run crictl inspect "$ctr_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "${output}" =~ "CONTAINER_EXITED" ]]
	# TODO: may be cri-tool should display Exit Code
	#[[ "${output}" =~ "Exit Code: 255" ]]

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
