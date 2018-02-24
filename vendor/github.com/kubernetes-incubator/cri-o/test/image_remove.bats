#!/usr/bin/env bats

load helpers

IMAGE=docker.io/kubernetes/pause

function teardown() {
	cleanup_test
}

@test "image remove with multiple names, by name" {
	start_crio "" "" --no-pause-image
	# Pull the image, giving it one name.
	run crictl pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	# Add a second name to the image.
	run "$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name="$IMAGE":latest --add-name="$IMAGE":othertag --signature-policy="$INTEGRATION_ROOT"/policy.json
	echo "$output"
	[ "$status" -eq 0 ]
	# Get the list of image names and IDs.
	run crictl images -v
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Cycle through each name, removing it by name.  The image that we assigned a second
	# name to should still be around when we get to removing its second name.
	grep ^RepoTags: <<< "$output" | while read -r header tag ignored ; do
		run crictl rmi "$tag"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	# List all images and their names.  There should be none now.
	run crictl images --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
	done
	# All done.
	cleanup_images
	stop_crio
}

@test "image remove with multiple names, by ID" {
	start_crio "" "" --no-pause-image
	# Pull the image, giving it one name.
	run crictl pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	# Add a second name to the image.
	run "$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name="$IMAGE":latest --add-name="$IMAGE":othertag --signature-policy="$INTEGRATION_ROOT"/policy.json
	echo "$output"
	[ "$status" -eq 0 ]
	# Get the list of the image's names and its ID.
	run crictl images -v "$IMAGE":latest
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Try to remove the image using its ID.  That should succeed.
	grep ^ID: <<< "$output" | while read -r header id ; do
		run crictl rmi "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	# The image should be gone now.
	run crictl images -v "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
	# All done.
	cleanup_images
	stop_crio
}
