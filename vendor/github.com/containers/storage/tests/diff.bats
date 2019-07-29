#!/usr/bin/env bats

load helpers

@test "diff" {
	# The checkdiffs function needs "tar".
	if test -z "$(which tar 2> /dev/null)" ; then
		skip "need tar"
	fi

	# Create and populate three interesting layers.
	populate

	# Mount the layers.
	run storage --debug=false mount "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	run storage --debug=false mount "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midmount="$output"
	run storage --debug=false mount "$upperlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	uppermount="$output"

	# Check the "diff" output.
	checkdiffs

	# Unmount the layers.
	storage unmount $lowerlayer
	storage unmount $midlayer
	storage unmount $upperlayer

	# Now check the "diff" again.
	checkdiffs
}
