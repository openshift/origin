#!/usr/bin/env bats

load helpers

@test "mount-layer" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer="$output"

	# Mount the layer.
	run storage --debug=false mount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Check if layer is mounted.
	run storage --debug=false mounted $layer
	[ "$status" -eq 0 ]
	[ "$output" == "$layer mounted" ]
	# Unmount the layer.
	run storage --debug=false unmount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Make sure layer is not mounted.
	run storage --debug=false mounted $layer
	[ "$status" -eq 0 ]
	[ "$output" == "" ]

	# Mount the layer twice.
	run storage --debug=false mount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	run storage --debug=false mount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Check if layer is mounted.
	run storage --debug=false mounted $layer
	[ "$status" -eq 0 ]
	[ "$output" == "$layer mounted" ]
	# Unmount the second layer.
	run storage --debug=false unmount $layer
	[ "$status" -eq 0 ]
	[ "$output" == "" ]
	# Check if layer is mounted.
	run storage --debug=false mounted $layer
	[ "$status" -eq 0 ]
	[ "$output" == "$layer mounted" ]
	# Unmount the first layer.
	run storage --debug=false unmount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Make sure layer is not mounted.
	run storage --debug=false mounted $layer
	[ "$status" -eq 0 ]
	[ "$output" == "" ]


	# Mount the layer twice and force umount.
	run storage --debug=false mount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	run storage --debug=false mount $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Check if layer is mounted.
	run storage --debug=false mounted $layer
	[ "$status" -eq 0 ]
	[ "$output" == "$layer mounted" ]
	# Unmount all layers.
	run storage --debug=false unmount --force $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Make sure no layers are mounted.
	run storage --debug=false mounted $layer
	[ "$status" -eq 0 ]
	[ "$output" == "" ]

	# Mount the layer with nosuid
	run storage --debug=false mount --option nosuid $layer
	[ "$status" -ne 0 ]

	# Delete the first layer
	run storage delete-layer $layer
	[ "$status" -eq 0 ]
}
