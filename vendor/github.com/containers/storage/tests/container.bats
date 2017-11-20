#!/usr/bin/env bats

load helpers

@test "container" {
	# Create and populate three interesting layers.
	populate

	# Create an image using to top layer.
	run storage --debug=false create-image $upperlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%   *}

	# Create a container using the image.
	name=wonderful-container
	run storage --debug=false create-container --name $name $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${lines[0]}

	# Add a couple of big data items.
	createrandom ${TESTDIR}/random1
	createrandom ${TESTDIR}/random2
	storage set-container-data -f ${TESTDIR}/random1 $container random1
	storage set-container-data -f ${TESTDIR}/random2 $container random2

	# Get information about the container, and make sure the ID, name, and data names were preserved.
	run storage container $container
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "ID: $container" ]]
	[[ "$output" =~ "Name: $name" ]]
	[[ "$output" =~ "Data: random1" ]]
	[[ "$output" =~ "Data: random2" ]]
}
