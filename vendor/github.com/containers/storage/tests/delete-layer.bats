#!/usr/bin/env bats

load helpers

@test "delete-layer" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer.
	run storage --debug=false mount $lowerlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Create a random file in the layer.
	createrandom "$lowermount"/layer1file1
	# Unmount the layer.
	storage unmount $lowerlayer

	# Create a second layer based on the first one.
	run storage --debug=false create-layer "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midlayer="$output"
	# Mount the second layer.
	run storage --debug=false mount $midlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midmount="$output"
	# Make sure the file from the first layer is present in this layer, then remove it.
	test -s "$midmount"/layer1file1
	rm -f -v "$midmount"/layer1file1
	# Create a new file in this layer.
	createrandom "$midmount"/layer2file1
	# Unmount the second layer.
	storage unmount $midlayer

	# Create a third layer based on the second one.
	run storage --debug=false create-layer "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer="$output"
	# Mount the third layer.
	run storage --debug=false mount $upperlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	uppermount="$output"
	# Make sure the file from the second layer is present in this layer,
	# and that the one from the first didn't come back somehow..
	test -s "$uppermount"/layer2file1
	run test -s "$uppermount"/layer1file1
	[ "$status" -ne 0 ]
	# Unmount the third layer.
	storage unmount $upperlayer

	# Try to delete the first layer, which should fail because it has children.
	run storage delete-layer $lowerlayer
	[ "$status" -ne 0 ]
	# Try to delete the second layer, which should fail because it has children.
	run storage delete-layer $midlayer
	[ "$status" -ne 0 ]
	# Try to delete the third layer, which should succeed because it has no children.
	storage delete-layer $upperlayer
	# Try to delete the second again, and it should succeed because that child is gone.
	storage delete-layer $midlayer
	# Try to delete the first again, and it should succeed because that child is gone.
	storage delete-layer $lowerlayer
}
