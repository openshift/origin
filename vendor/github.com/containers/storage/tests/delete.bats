#!/usr/bin/env bats

load helpers

@test "delete" {
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
	container=${output%%	*}

	# Check that the container can be found, and delete it using the general delete command.
	storage exists -c $container
	storage delete $container

	# Check that the container is gone.
	run storage exists -c $container
	[ "$status" -ne 0 ]

	# Check that the image can be found, and delete it using the general delete command.
	storage exists -i $image
	storage delete $image

	# Check that the image is gone.
	run storage exists -i $image
	[ "$status" -ne 0 ]

	# Check that the layer can be found, and delete it using the general delete command.
	storage exists -l $layer
	storage delete $layer

	# Check that the layer is gone.
	run storage exists -l $layer
	[ "$status" -ne 0 ]
}
