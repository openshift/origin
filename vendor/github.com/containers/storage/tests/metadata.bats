#!/usr/bin/env bats

load helpers

@test "metadata" {
	echo danger > $TESTDIR/danger.txt

	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Make sure the layer's there.
	storage exists -l $layer

	# Create an image using the layer and directly-supplied metadata.
	run storage --debug=false create-image -m danger $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Make sure that the image is there.
	storage exists -i $image

	# Read back the metadata and make sure it's the right value.
	run storage --debug=false metadata -q $image
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]

	# Change the metadata to a directly-supplied value.
	run storage set-metadata -m thunder $image
	[ "$status" -eq 0 ]

	# Read back the metadata and make sure it's the new value.
	run storage --debug=false metadata -q $image
	[ "$status" -eq 0 ]
	[ "$output" = "thunder" ]

	# Change the metadata to a value supplied via a file.
	storage set-metadata -f $TESTDIR/danger.txt $image

	# Read back the metadata and make sure it's the newer value.
	run storage --debug=false metadata -q $image
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]

	# Create an image using the layer and metadata read from a file.
	run storage --debug=false create-image -f $TESTDIR/danger.txt $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Make sure that the image is there.
	storage exists -i $image

	# Read back the metadata and make sure it's the right value.
	run storage --debug=false metadata -q $image
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]

	# Change the metadata to a directly-supplied value.
	storage set-metadata -m thunder $image

	# Read back the metadata and make sure it's the new value.
	run storage --debug=false metadata -q $image
	[ "$status" -eq 0 ]
	[ "$output" = "thunder" ]

	# Change the metadata to a value supplied via a file.
	storage set-metadata -f $TESTDIR/danger.txt $image

	# Read back the metadata and make sure it's the newer value.
	run storage --debug=false metadata -q $image
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]

	# Create a container based on the image and directly-supplied metadata.
	run storage --debug=false create-container -m danger $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Make sure the container is there.
	storage exists -c $container

	# Read the metadata and make sure it's the right value.
	run storage --debug=false metadata -q $container
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]

	# Change the metadata to a new value.
	storage set-metadata -m thunder $container

	# Read back the new metadata value.
	run storage --debug=false metadata -q $container
	[ "$status" -eq 0 ]
	[ "$output" = "thunder" ]

	# Change the metadata to a new value read from a file.
	storage set-metadata -f $TESTDIR/danger.txt $container

	# Read back the newer metadata value.
	run storage --debug=false metadata -q $container
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]

	# Create a container based on the image and metadata read from a file.
	run storage --debug=false create-container -f $TESTDIR/danger.txt $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Make sure the container is there.
	storage exists -c $container

	# Read the metadata and make sure it's the right value.
	run storage --debug=false metadata -q $container
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]

	# Change the metadata to a new value.
	storage set-metadata -m thunder $container

	# Read back the new metadata value.
	run storage --debug=false metadata -q $container
	[ "$status" -eq 0 ]
	[ "$output" = "thunder" ]

	# Change the metadata to a new value read from a file.
	storage set-metadata -f $TESTDIR/danger.txt $container

	# Read back the newer metadata value.
	run storage --debug=false metadata -q $container
	[ "$status" -eq 0 ]
	[ "$output" = "danger" ]
}
