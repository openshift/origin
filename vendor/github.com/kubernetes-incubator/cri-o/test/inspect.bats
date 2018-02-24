#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "info inspect" {
	start_crio
	out=`echo -e "GET /info HTTP/1.1\r\nHost: crio\r\n" | socat - UNIX-CONNECT:$CRIO_SOCKET`
	echo "$out"
	[[ "$out" =~ "\"cgroup_driver\":\"$CGROUP_MANAGER\"" ]]
	[[ "$out" =~ "\"storage_root\":\"$TESTDIR/crio\"" ]]

	stop_crio
}

@test "ctr inspect" {
	start_crio
	run crictl runp "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crictl create "$pod_id" "$TESTDATA"/container_config.json "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	out=`echo -e "GET /containers/$ctr_id HTTP/1.1\r\nHost: crio\r\n" | socat - UNIX-CONNECT:$CRIO_SOCKET`
	echo "$out"
	[[ "$out" =~ "\"sandbox\":\"$pod_id\"" ]]
	[[ "$out" =~ "\"image\":\"docker.io/library/redis:alpine\"" ]]
	[[ "$out" =~ "\"image_ref\":\"$REDIS_IMAGEREF\"" ]]

	run crictl inspect --output json "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "\"id\": \"$ctr_id\"" ]]
	[[ "$output" =~ "\"image\": \"docker.io/library/redis:alpine\"" ]]
	[[ "$output" =~ "\"imageRef\": \"$REDIS_IMAGEREF\"" ]]

	run crictl inspectp --output json "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]

	inet=`crictl exec --sync "$ctr_id" ip addr show dev eth0 scope global 2>&1 | grep inet`

	IFS=" "
	ip=`parse_pod_ip $inet`
	[[ "$out" =~ "\"ip_address\":\"$ip\"" ]]
	[[ "$output" =~ "\"ip\": \"$ip\"" ]]


# TODO: add some other check based on the json below:
#
# {"name":"k8s_container1_podsandbox1_redhat.test.crio_redhat-test-crio_1","pid":27477,"image":"redis:alpine","created_time":1505223601111546169,"labels":{"batch":"no","type":"small"},"annotations":{"daemon":"crio","owner":"dragon"},"log_path":"/var/log/crio/pods/297d014ba2c54236779da0c2f80dfba45dc31b106e4cd126a1c3c6d78edc2201/81567e9573ea798d6494c9aab156103ee91b72180fd3841a7c24d2ca39886ba2.log","root":"/tmp/tmp.0bkjphWudF/crio/overlay/d7cfc1de83cab9f377a4a1542427d2a019e85a70c1c660a9e6cf9e254df68873/merged","sandbox":"297d014ba2c54236779da0c2f80dfba45dc31b106e4cd126a1c3c6d78edc2201","ip_address":"10.88.9.153"}

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "ctr inspect not found" {
	start_crio
	out=`echo -e "GET /containers/notexists HTTP/1.1\r\nHost: crio\r\n" | socat - UNIX-CONNECT:$CRIO_SOCKET`
	echo "$out"
	[[ "$out" =~ "can't find the container with id notexists" ]]

	stop_crio
}
