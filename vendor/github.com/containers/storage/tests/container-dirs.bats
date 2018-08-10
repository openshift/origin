#!/usr/bin/env bats

load helpers

@test "container-dirs" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Check that the layer can be found.
	storage exists -l $layer

	# Create an image using the layer.
	run storage --debug=false create-image -m danger $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Check that the image can be found.
	storage exists -i $image

	# Create a container based on the layer.
	run storage --debug=false create-container $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Check that the container can be found.
	storage exists -c $container

	# Check that the container's user data directory is somewhere under the root.
	run storage --debug=false get-container-dir $container
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	dir=${output%%	*}
	touch "$dir"/dirfile
	echo "$dir"/dirfile | grep -q ^"${TESTDIR}/root/"

	# Check that the container's user run data directory is somewhere under the run root.
	run storage --debug=false get-container-run-dir $container
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	rundir=${output%%	*}
	touch "$rundir"/rundirfile
	echo "$rundir"/rundirfile | grep -q ^"${TESTDIR}/runroot/"
}
