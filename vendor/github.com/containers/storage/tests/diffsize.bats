#!/usr/bin/env bats

load helpers

@test "diffsize" {
	# Create and populate three interesting layers.
	populate

	# Mount the layers.
	run storage --debug=false diffsize "$lowerlayer"
	[ "$status" -eq 0 ]
	echo size:"$output":
	[ "$output" -ne 0 ]
	run storage --debug=false diffsize "$midlayer"
	[ "$status" -eq 0 ]
	echo size:"$output":
	[ "$output" -ne 0 ]
	run storage --debug=false diffsize "$upperlayer"
	[ "$status" -eq 0 ]
	echo size:"$output":
	[ "$output" -ne 0 ]
}
