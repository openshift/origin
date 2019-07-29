#!/usr/bin/env bats

load helpers

@test "changes" {
	# Create and populate three interesting layers.
	populate

	# Mount the layers.
	run storage --debug=false mount "$lowerlayer"
	[ "$status" -eq 0 ]
	lowermount="$output"
	run storage --debug=false mount "$midlayer"
	[ "$status" -eq 0 ]
	midmount="$output"
	run storage --debug=false mount "$upperlayer"
	[ "$status" -eq 0 ]
	uppermount="$output"

	# Check the "changes" output.
	checkchanges

	# Unmount the layers.
	storage unmount $lowerlayer
	storage unmount $midlayer
	storage unmount $upperlayer

	# Now check the "changes" again.
	checkchanges
}
