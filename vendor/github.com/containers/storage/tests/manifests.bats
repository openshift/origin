#!/usr/bin/env bats

load helpers

@test "manifests" {
	# Create and populate three interesting layers.
	populate

	# Create an image using the top layer.
	name=wonderful-image
	run storage --debug=false create-image --name $name $upperlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${lines[0]}

	# Add a couple of big data items as manifests.
	createrandom ${TESTDIR}/random1
	createrandom ${TESTDIR}/random2
	createrandom ${TESTDIR}/random3
	digest1=$(sha256sum ${TESTDIR}/random1)
	digest1=${digest1// *}
	digest2=$(sha256sum ${TESTDIR}/random2)
	digest2=${digest2// *}
	digest3=$(sha256sum ${TESTDIR}/random3)
	digest3=${digest3// *}
	storage set-image-data -f ${TESTDIR}/random1 $image manifest
	storage set-image-data -f ${TESTDIR}/random2 $image manifest-random2
	storage set-image-data -f ${TESTDIR}/random3 $image manifest-random3
	storage add-names --name localhost/fooimage:latest $image

	# Get information about the image, and make sure the ID, name, and data names were preserved.
	run storage image $image
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "ID: $image" ]]
	[[ "$output" =~ "Name: $name" ]]
	[[ "$output" =~ "Digest: sha256:$digest1" ]]
	[[ "$output" =~ "Digest: sha256:$digest2" ]]
	[[ "$output" =~ "Digest: sha256:$digest3" ]]

	run storage images-by-digest sha256:$digest1
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$image" ]]
	[[ "$output" =~ "name: $name" ]]
	[[ "$output" =~ "digest: sha256:$digest1" ]]
	[[ "$output" =~ "digest: sha256:$digest2" ]]
	[[ "$output" =~ "digest: sha256:$digest3" ]]

	run storage images-by-digest sha256:$digest2
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$image" ]]
	[[ "$output" =~ "name: $name" ]]
	[[ "$output" =~ "digest: sha256:$digest1" ]]
	[[ "$output" =~ "digest: sha256:$digest2" ]]
	[[ "$output" =~ "digest: sha256:$digest3" ]]

	run storage images-by-digest sha256:$digest3
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "$image" ]]
	[[ "$output" =~ "name: $name" ]]
	[[ "$output" =~ "digest: sha256:$digest1" ]]
	[[ "$output" =~ "digest: sha256:$digest2" ]]
	[[ "$output" =~ "digest: sha256:$digest3" ]]
}
