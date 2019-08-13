#!/usr/bin/env bats

load helpers

@test "create-image" {
	# Create an image using no layer.
	run storage --debug=false create-image ""
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	zerothimage=${output%%	*}

	zerothwriter=$(cat ${TESTDIR}/root/${STORAGE_DRIVER}-images/images.lock)
	[ "$zerothwriter" != "" ]

	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image using that layer.
	run storage --debug=false create-image $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	firstimage=${output%%	*}

	firstwriter=$(cat ${TESTDIR}/root/${STORAGE_DRIVER}-images/images.lock)
	[ "$firstwriter" != "" ]
	[ "$zerothwriter" != "$firstwriter" ]

	# Check that the image can be accessed.
	storage exists -i $firstimage

	# Create another image using that layer.
	run storage --debug=false create-image $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	secondimage=${output%%	*}

	# Check that *that* image can be accessed.
	storage exists -i $secondimage

	# Check that "images" lists the both of the images.
	run storage --debug=false images
	[ "$status" -eq 0 ]
	echo :"$output":
	[ "${#lines[*]}" -eq 3 ]
	[ "${lines[0]}" != "${lines[1]}" ]
	[ "${lines[1]}" != "${lines[2]}" ]
	[ "${lines[0]}" != "${lines[2]}" ]
	[ "${lines[0]}" = "$zerothimage" ] || [ "${lines[0]}" = "$firstimage" ] || [ "${lines[0]}" = "$secondimage" ]
	[ "${lines[1]}" = "$zerothimage" ] || [ "${lines[1]}" = "$firstimage" ] || [ "${lines[1]}" = "$secondimage" ]
	[ "${lines[2]}" = "$zerothimage" ] || [ "${lines[2]}" = "$firstimage" ] || [ "${lines[2]}" = "$secondimage" ]
}
