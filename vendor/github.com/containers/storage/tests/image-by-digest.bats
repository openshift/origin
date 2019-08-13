#!/usr/bin/env bats

load helpers

@test "images-by-digest" {
	# Bail if "sha256sum" isn't available.
	if test -z "$(which sha256sum 2> /dev/null)" ; then
		skip "need sha256sum"
	fi

	# Create a couple of random files.
	createrandom ${TESTDIR}/random1
	createrandom ${TESTDIR}/random2
	createrandom ${TESTDIR}/random3
	digest1=$(sha256sum ${TESTDIR}/random1)
	digest2=$(sha256sum ${TESTDIR}/random2)
	digest3=$(sha256sum ${TESTDIR}/random3)

	# Create a layer.
	run storage --debug=false create-layer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image using that layer.
	run storage --debug=false create-image $layer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	firstimage=${output%%	*}
	# Set the first file as the manifest of this image.
	run storage --debug=false set-image-data -f ${TESTDIR}/random1 ${firstimage} manifest
	echo "$output"

	# Create another image using that layer.
	run storage --debug=false create-image $layer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	secondimage=${output%%	*}
	# Set the first file as the manifest of this image.
	run storage --debug=false set-image-data -f ${TESTDIR}/random1 ${secondimage} manifest
	echo "$output"

	# Create yet another image using that layer.
	run storage --debug=false create-image $layer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	thirdimage=${output%%	*}
	# Set the second file as the manifest of this image.
	run storage --debug=false set-image-data -f ${TESTDIR}/random2 ${thirdimage} manifest
	echo "$output"

	# Create still another image using that layer.
	run storage --debug=false create-image --digest sha256:${digest3// *} $layer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	fourthimage=${output%%	*}

	# Create another image using that layer.
	run storage --debug=false create-image --digest sha256:${digest3// *} $layer
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	fifthimage=${output%%	*}
	# Set the third file as the manifest of this image.
	run storage --debug=false set-image-data -f ${TESTDIR}/random3 ${fifthimage} manifest
	echo "$output"

	# Check that "images-by-digest" lists the right images.
	run storage --debug=false images-by-digest --quiet sha256:${digest1// *}
	echo "$output"
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 2 ]
	[ "${lines[0]}" != "${lines[1]}" ]
	[ "${lines[0]}" = "$firstimage" ] || [ "${lines[0]}" = "$secondimage" ]
	[ "${lines[1]}" = "$firstimage" ] || [ "${lines[1]}" = "$secondimage" ]

	run storage --debug=false images-by-digest --quiet sha256:${digest2// *}
	echo "$output"
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 1 ]
	[ "${lines[0]}" = "$thirdimage" ]

	run storage --debug=false images-by-digest --quiet sha256:${digest3// *}
	echo "$output"
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 2 ]
	[ "${lines[0]}" = "$fourthimage" ] || [ "${lines[0]}" = "$fifthimage" ]
	[ "${lines[1]}" = "$fourthimage" ] || [ "${lines[1]}" = "$fifthimage" ]

	run storage --debug=false delete-image ${secondimage}
	echo "$output"
	[ "$status" -eq 0 ]

	run storage --debug=false images-by-digest --quiet sha256:${digest1// *}
	echo "$output"
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 1 ]
	[ "${lines[0]}" = "$firstimage" ]

	run storage --debug=false delete-image ${firstimage}
	echo "$output"
	[ "$status" -eq 0 ]

	run storage --debug=false images-by-digest --quiet sha256:${digest1// *}
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
}
