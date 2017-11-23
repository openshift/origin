#!/usr/bin/env bats

load helpers

@test "create-container" {
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

	# Check that the container can be found.
	storage exists -c $firstcontainer

	# Create another container based on the same image.
	run storage --debug=false create-container $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	secondcontainer=${output%%	*}

	# Check that *that* container can be found.
	storage exists -c $secondcontainer

	# Check that a list of containers lists both of them.
	run storage --debug=false containers
	echo :"$output":
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 2 ]
	[ "${lines[0]}" != "${lines[1]}" ]
	[ "${lines[0]}" = "$firstcontainer" ] || [ "${lines[0]}" = "$secondcontainer" ]
	[ "${lines[1]}" = "$firstcontainer" ] || [ "${lines[1]}" = "$secondcontainer" ]
}
