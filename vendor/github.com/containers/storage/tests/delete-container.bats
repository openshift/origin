#!/usr/bin/env bats

load helpers

@test "delete-container" {
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

	# Create an image using that layer.
	run storage --debug=false create-container $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Check that the container can be found.
	storage exists -c $container

	# Use delete-container to delete it.
	storage delete-container $container

	# Check that the container is gone.
	run storage exists -c $container
	[ "$status" -ne 0 ]
}
