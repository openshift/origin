#!/usr/bin/env bats

load helpers

@test "image" {
	# Create and populate three interesting layers.
	populate

	# Create an image using to top layer.
	name=wonderful-image
	run storage --debug=false create-image --name $name $upperlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${lines[0]}

	# Add a couple of big data items.
	createrandom ${TESTDIR}/random1
	createrandom ${TESTDIR}/random2
	storage set-image-data -f ${TESTDIR}/random1 $image random1
	storage set-image-data -f ${TESTDIR}/random2 $image random2

	# Get information about the image, and make sure the ID, name, and data names were preserved.
	run storage image $image
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "ID: $image" ]]
	[[ "$output" =~ "Name: $name" ]]
	[[ "$output" =~ "Data: random1" ]]
	[[ "$output" =~ "Data: random2" ]]
}
