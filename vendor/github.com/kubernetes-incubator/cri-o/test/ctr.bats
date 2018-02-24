#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "ctr not found correct error message" {
	start_crio
	run crictl inspect "container_not_exist"
	echo "$output"
	[ "$status" -eq 1 ]

	stop_crio
}

@test "ctr termination reason Completed" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run sleep 5
	run crictl inspect --output yaml "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "reason: Completed" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr termination reason Error" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	errorconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["command"] = ["false"]; json.dump(obj, sys.stdout)')
	echo "$errorconfig" > "$TESTDIR"/container_config_error.json
	run crictl create "$pod_id" "$TESTDIR"/container_config_error.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run sleep 5
	run crictl inspect --output yaml "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "reason: Error" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr remove" {
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
	run crictl rm "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
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

@test "ctr lifecycle" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl pods --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$pod_id" ]]
	run crictl create "$pod_id" "$TESTDATA"/container_redis.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl ps --quiet --state created
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr_id" ]]
	run crictl inspect "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl inspect "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl ps --quiet --state running
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr_id" ]]
	run crictl stop "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl inspect "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl ps --quiet --state stopped
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr_id" ]]
	run crictl rm "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl ps --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stopp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl pods --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$pod_id" ]]
	run crictl ps --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]
	run crictl rmp "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl pods --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr logging" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	# Create a new container.
	newconfig=$(mktemp --tmpdir crio-config.XXXXXX.json)
	cp "$TESTDATA"/container_config_logging.json "$newconfig"
	sed -i 's|"%shellcommand%"|"echo here is some output \&\& echo and some from stderr >\&2"|' "$newconfig"
	run crictl create "$pod_id" "$newconfig" "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stop "$ctr_id"
	echo "$output"
	# Ignore errors on stop.
	run crictl inspect "$ctr_id"
	[ "$status" -eq 0 ]
	run crictl rm "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	# Check that the output is what we expect.
	logpath="$DEFAULT_LOG_PATH/$pod_id/$ctr_id.log"
	[ -f "$logpath" ]
	echo "$logpath :: $(cat "$logpath")"
	grep -E "^[^\n]+ stdout F here is some output$" "$logpath"
	grep -E "^[^\n]+ stderr F and some from stderr$" "$logpath"

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

@test "ctr logging [tty=true]" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	# Create a new container.
	newconfig=$(mktemp --tmpdir crio-config.XXXXXX.json)
	cp "$TESTDATA"/container_config_logging.json "$newconfig"
	sed -i 's|"%shellcommand%"|"echo here is some output"|' "$newconfig"
	sed -i 's|"tty": false,|"tty": true,|' "$newconfig"
	run crictl create "$pod_id" "$newconfig" "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stop "$ctr_id"
	echo "$output"
	# Ignore errors on stop.
	run crictl inspect "$ctr_id"
	[ "$status" -eq 0 ]
	run crictl rm "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	# Check that the output is what we expect.
	logpath="$DEFAULT_LOG_PATH/$pod_id/$ctr_id.log"
	[ -f "$logpath" ]
	echo "$logpath :: $(cat "$logpath")"
	grep --binary -P "^[^\n]+ stdout F here is some output\x0d$" "$logpath"

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

@test "ctr log max" {
	LOG_SIZE_MAX_LIMIT=10000 start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	# Create a new container.
	newconfig=$(mktemp --tmpdir crio-config.XXXXXX.json)
	cp "$TESTDATA"/container_config_logging.json "$newconfig"
	sed -i 's|"%shellcommand%"|"for i in $(seq 250); do echo $i; done"|' "$newconfig"
	run crictl create "$pod_id" "$newconfig" "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	sleep 6
	run crictl inspect "$ctr_id"
	[ "$status" -eq 0 ]
	run crictl rm "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	# Check that the output is what we expect.
	logpath="$DEFAULT_LOG_PATH/$pod_id/$ctr_id.log"
	[ -f "$logpath" ]
	echo "$logpath :: $(cat "$logpath")"
	len=$(wc -l "$logpath" | awk '{print $1}')
	[ $len -lt 250 ]

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

@test "ctr partial line logging" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	# Create a new container.
	newconfig=$(mktemp --tmpdir crio-config.XXXXXX.json)
	cp "$TESTDATA"/container_config_logging.json "$newconfig"
	sed -i 's|"%shellcommand%"|"echo -n hello"|' "$newconfig"
	run crictl create "$pod_id" "$newconfig" "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stop "$ctr_id"
	echo "$output"
	# Ignore errors on stop.
	run crictl inspect "$ctr_id"
	[ "$status" -eq 0 ]
	run crictl rm "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	# Check that the output is what we expect.
	logpath="$DEFAULT_LOG_PATH/$pod_id/$ctr_id.log"
	[ -f "$logpath" ]
	echo "$logpath :: $(cat "$logpath")"
	grep -E "^[^\n]+ stdout P hello$" "$logpath"

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

# regression test for #127
@test "ctrs status for a pod" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_redis.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl ps --quiet --state created
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" == "$ctr_id" ]]

	printf '%s\n' "$output" | while IFS= read -r id
	do
		run crictl inspect "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr list filtering" {
	# start 3 redis sandbox
	# pod1 ctr1 create & start
	# pod2 ctr2 create
	# pod3 ctr3 create & start & stop
	start_crio
	run crictl runp "$TESTDATA"/sandbox1_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod1_id="$output"
	run crictl create "$pod1_id" "$TESTDATA"/container_redis.json "$TESTDATA"/sandbox1_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr1_id="$output"
	run crictl start "$ctr1_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl runp "$TESTDATA"/sandbox2_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod2_id="$output"
	run crictl create "$pod2_id" "$TESTDATA"/container_redis.json "$TESTDATA"/sandbox2_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr2_id="$output"
	run crictl runp "$TESTDATA"/sandbox3_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod3_id="$output"
	run crictl create "$pod3_id" "$TESTDATA"/container_redis.json "$TESTDATA"/sandbox3_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr3_id="$output"
	run crictl start "$ctr3_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stop "$ctr3_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl ps --id "$ctr1_id" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr1_id" ]]
	run crictl ps --id "${ctr1_id:0:4}" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr1_id" ]]
	run crictl ps --id "$ctr2_id" --podsandbox "$pod2_id" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr2_id" ]]
	run crictl ps --id "$ctr2_id" --podsandbox "$pod3_id" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]
	run crictl ps --state created --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr2_id" ]]
	run crictl ps --state running --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr1_id" ]]
	run crictl ps --state stopped --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr3_id" ]]
	run crictl ps --podsandbox "$pod1_id" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr1_id" ]]
	run crictl ps --podsandbox "$pod2_id" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr2_id" ]]
	run crictl ps --podsandbox "$pod3_id" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr3_id" ]]
	run crictl stopp "$pod1_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl rmp "$pod1_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stopp "$pod2_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl rmp "$pod2_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stopp "$pod3_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl rmp "$pod3_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr list label filtering" {
	# start a pod with 3 containers
	# ctr1 with labels: group=test container=redis version=v1.0.0
	# ctr2 with labels: group=test container=redis version=v1.0.0
	# ctr3 with labels: group=test container=redis version=v1.1.0
	start_crio

	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	ctrconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["metadata"]["name"] = "ctr1";obj["labels"]["group"] = "test";obj["labels"]["name"] = "ctr1";obj["labels"]["version"] = "v1.0.0"; json.dump(obj, sys.stdout)')
	echo "$ctrconfig" > "$TESTDATA"/labeled_container_redis.json
	run crictl create "$pod_id" "$TESTDATA"/labeled_container_redis.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr1_id="$output"

	ctrconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["metadata"]["name"] = "ctr2";obj["labels"]["group"] = "test";obj["labels"]["name"] = "ctr2";obj["labels"]["version"] = "v1.0.0"; json.dump(obj, sys.stdout)')
	echo "$ctrconfig" > "$TESTDATA"/labeled_container_redis.json
	run crictl create "$pod_id" "$TESTDATA"/labeled_container_redis.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr2_id="$output"

	ctrconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["metadata"]["name"] = "ctr3";obj["labels"]["group"] = "test";obj["labels"]["name"] = "ctr3";obj["labels"]["version"] = "v1.1.0"; json.dump(obj, sys.stdout)')
	echo "$ctrconfig" > "$TESTDATA"/labeled_container_redis.json
	run crictl create "$pod_id" "$TESTDATA"/labeled_container_redis.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr3_id="$output"

	run crictl ps --label "group=test" --label "name=ctr1" --label "version=v1.0.0" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "$ctr1_id" ]]
	run crictl ps --label "group=production" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]
	run crictl ps --label "group=test" --label "version=v1.0.0" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$ctr1_id" ]]
	[[ "$output" =~ "$ctr2_id" ]]
	[[ "$output" != "$ctr3_id" ]]
	run crictl ps --label "group=test" --quiet --all
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" != "" ]]
	[[ "$output" =~ "$ctr1_id"  ]]
	[[ "$output" =~ "$ctr2_id"  ]]
	[[ "$output" =~ "$ctr3_id"  ]]
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

@test "ctr metadata in list & status" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crictl ps --id "$ctr_id" --output yaml --state created
	echo "$output"
	[ "$status" -eq 0 ]
	# TODO: expected value should not hard coded here
	[[ "$output" =~ "name: container1" ]]
	[[ "$output" =~ "attempt: 1" ]]

	run crictl inspect "$ctr_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	# TODO: expected value should not hard coded here
	[[ "$output" =~ "Name: container1" ]]
	[[ "$output" =~ "Attempt: 1" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr execsync conflicting with conmon flags parsing" {
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
	run crictl exec --sync "$ctr_id" sh -c "echo hello world"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "hello world" ]]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr execsync" {
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
	run crictl exec --sync "$ctr_id" echo HELLO
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" == "HELLO" ]]
	run crictl exec --sync --timeout 1 "$ctr_id" sleep 3
	echo "$output"
	[[ "$output" =~ "command timed out" ]]
	[ "$status" -ne 0 ]
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

@test "ctr device add" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_redis_device.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl exec --sync "$ctr_id" ls /dev/mynull
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "/dev/mynull" ]]
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

@test "ctr hostname env" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl exec --sync "$ctr_id" env
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "HOSTNAME" ]]
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

@test "ctr execsync failure" {
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
	run crictl exec --sync "$ctr_id" doesnotexist
	echo "$output"
	[ "$status" -ne 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr execsync exit code" {
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
	run crictl exec --sync "$ctr_id" false
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Exit code: 1" ]]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr execsync std{out,err}" {
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
	run crictl exec --sync "$ctr_id" echo hello0 stdout
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "hello0 stdout" ]]

	stderrconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["image"]["image"] = "runcom/stderr-test"; obj["command"] = ["/bin/sleep", "600"]; json.dump(obj, sys.stdout)')
	echo "$stderrconfig" > "$TESTDIR"/container_config_stderr.json
	run crictl create "$pod_id" "$TESTDIR"/container_config_stderr.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl exec --sync "$ctr_id" stderr
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "this goes to stderr" ]]
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

@test "ctr stop idempotent" {
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
	run crictl stop "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl stop "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr caps drop" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	capsconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["linux"]["security_context"]["capabilities"] = {u"add_capabilities": [], u"drop_capabilities": [u"mknod", u"kill", u"sys_chroot", u"setuid", u"setgid"]}; json.dump(obj, sys.stdout)')
	echo "$capsconfig" > "$TESTDIR"/container_config_caps.json
	run crictl create "$TESTDIR"/container_config_caps.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "run ctr with image with Config.Volumes" {
	start_crio
	run crictl pull gcr.io/k8s-testimages/redis:e2e
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	volumesconfig=$(cat "$TESTDATA"/container_redis.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["image"]["image"] = "gcr.io/k8s-testimages/redis:e2e"; obj["args"] = []; json.dump(obj, sys.stdout)')
	echo "$volumesconfig" > "$TESTDIR"/container_config_volumes.json
	run crictl create "$pod_id" "$TESTDIR"/container_config_volumes.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr oom" {
	if [[ "$TRAVIS" == "true" ]]; then
		skip "travis container tests don't support testing OOM"
	fi
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	oomconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["image"]["image"] = "mrunalp/oom"; obj["linux"]["resources"]["memory_limit_in_bytes"] = 5120000; obj["command"] = ["/oom"]; json.dump(obj, sys.stdout)')
	echo "$oomconfig" > "$TESTDIR"/container_config_oom.json
	run crictl create "$pod_id" "$TESTDIR"/container_config_oom.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	# Wait for container to OOM
	attempt=0
	while [ $attempt -le 100 ]; do
		attempt=$((attempt+1))
		run crictl inspect --output yaml "$ctr_id"
		echo "$output"
		[ "$status" -eq 0 ]
		if [[ "$output" =~ "OOMKilled" ]]; then
			break
		fi
		sleep 10
	done
	[[ "$output" =~ "OOMKilled" ]]
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

@test "ctr /etc/resolv.conf rw/ro mode" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_config_resolvconf.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl inspect "$ctr_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "State: CONTAINER_EXITED" ]]
	[[ "$output" =~ "Exit Code: 0" ]]

	run crictl create "$pod_id" "$TESTDATA"/container_config_resolvconf_ro.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl inspect "$ctr_id" --output table
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "State: CONTAINER_EXITED" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr create with non-existent command" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	newconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["command"] = ["nonexistent"]; json.dump(obj, sys.stdout)')
	echo "$newconfig" > "$TESTDIR"/container_nonexistent.json
	run crictl create "$pod_id" "$TESTDIR"/container_nonexistent.json "$TESTDATA"/sandbox_config.json
	[ "$status" -ne 0 ]
	[[ "$output" =~ "executable file not found" ]]
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

@test "ctr create with non-existent command [tty]" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	newconfig=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["command"] = ["nonexistent"]; obj["tty"] = True; json.dump(obj, sys.stdout)')
	echo "$newconfig" > "$TESTDIR"/container_nonexistent.json
	run crictl create "$pod_id" "$TESTDIR"/container_nonexistent.json "$TESTDATA"/sandbox_config.json
	[ "$status" -ne 0 ]
	[[ "$output" =~ "executable file not found" ]]
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

@test "ctr update resources" {
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

	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/memory/memory.limit_in_bytes"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "209715200" ]]
	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpu/cpu.shares"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "512" ]]
	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpu/cpu.cfs_period_us"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "10000" ]]
	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "20000" ]]

	run crictl update --memory 524288000 --cpu-period 20000 --cpu-quota 10000 --cpu-share 256 "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/memory/memory.limit_in_bytes"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "524288000" ]]
	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpu/cpu.shares"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "256" ]]
	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpu/cpu.cfs_period_us"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "20000" ]]
	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "10000" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr correctly setup working directory" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	notexistcwd=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["working_dir"] = "/thisshouldntexistatall"; json.dump(obj, sys.stdout)')
	echo "$notexistcwd" > "$TESTDIR"/container_cwd_notexist.json
	run crictl create "$pod_id" "$TESTDIR"/container_cwd_notexist.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]

	filecwd=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["working_dir"] = "/etc/passwd"; obj["metadata"]["name"] = "container2"; json.dump(obj, sys.stdout)')
	echo "$filecwd" > "$TESTDIR"/container_cwd_file.json
	run crictl create "$pod_id" "$TESTDIR"/container_cwd_file.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -ne 0 ]
	ctr_id="$output"
	[[ "$output" =~ "not a directory" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr execsync conflicting with conmon env" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_redis_env_custom.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crictl start "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crictl exec "$ctr_id" env
	echo "$output"
	echo "$status"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "acustompathinpath" ]]
	run crictl exec --sync "$ctr_id" env
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "acustompathinpath" ]]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr resources" {
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

	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpuset/cpuset.cpus"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "0-1" ]]
	run crictl exec --sync "$ctr_id" sh -c "cat /sys/fs/cgroup/cpuset/cpuset.mems"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "0" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}
