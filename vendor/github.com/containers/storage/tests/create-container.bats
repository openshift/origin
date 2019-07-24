#!/usr/bin/env bats

load helpers

@test "create-container" {
	# Create a container based on no image.
	run storage --debug=false create-container ""
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	zerothcontainer=${output%%	*}

	# Create an image using no layer.
	run storage --debug=false create-image ""
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Create a container based on that image.
	run storage --debug=false create-container $image
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	thirdcontainer=${output%%	*}

	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image using that layer.
	run storage --debug=false create-image $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Create a container based on that image.
	run storage --debug=false create-container $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	firstcontainer=${output%%	*}

	firstwriter=$(cat ${TESTDIR}/root/${STORAGE_DRIVER}-containers/containers.lock)
	[ "$firstwriter" != "" ]

	# Check that the container can be found.
	storage exists -c $firstcontainer

	# Create another container based on the same image.
	run storage --debug=false create-container $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	secondcontainer=${output%%	*}

	secondwriter=$(cat ${TESTDIR}/root/${STORAGE_DRIVER}-containers/containers.lock)
	[ "$secondwriter" != "" ]
	[ "$firstwriter" != "$secondwriter" ]

	# Check that *that* container can be found.
	storage exists -c $secondcontainer

	# Check that a list of containers lists both of them.
	run storage --debug=false containers
	echo :"$output":
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 4 ]
	[ "${lines[0]}" != "${lines[1]}" ]
	[ "${lines[0]}" != "${lines[2]}" ]
	[ "${lines[0]}" != "${lines[3]}" ]
	[ "${lines[1]}" != "${lines[2]}" ]
	[ "${lines[1]}" != "${lines[3]}" ]
	[ "${lines[2]}" != "${lines[3]}" ]
	[ "${lines[0]}" = "$zerothcontainer" ] || [ "${lines[0]}" = "$firstcontainer" ] || [ "${lines[0]}" = "$secondcontainer" ] || [ "${lines[0]}" = "$thirdcontainer" ]
	[ "${lines[1]}" = "$zerothcontainer" ] || [ "${lines[1]}" = "$firstcontainer" ] || [ "${lines[1]}" = "$secondcontainer" ] || [ "${lines[1]}" = "$thirdcontainer" ]
	[ "${lines[2]}" = "$zerothcontainer" ] || [ "${lines[2]}" = "$firstcontainer" ] || [ "${lines[2]}" = "$secondcontainer" ] || [ "${lines[2]}" = "$thirdcontainer" ]
	[ "${lines[3]}" = "$zerothcontainer" ] || [ "${lines[3]}" = "$firstcontainer" ] || [ "${lines[3]}" = "$secondcontainer" ] || [ "${lines[3]}" = "$thirdcontainer" ]
}
