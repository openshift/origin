#!/usr/bin/env bats

load helpers

@test "delete-image" {
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

	# Check that the image can be found.
	storage exists -i $image

	# Use delete-image to delete it.
	storage delete-image $image

	# Check that the image is gone.
	run storage exists -i $image
	[ "$status" -ne 0 ]
}
