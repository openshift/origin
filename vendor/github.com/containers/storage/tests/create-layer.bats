#!/usr/bin/env bats

load helpers

@test "create-layer" {
	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	lowerwriter=$(cat ${TESTDIR}/root/${STORAGE_DRIVER}-layers/layers.lock)
	[ "$lowerwriter" != "" ]
	# Mount the layer.
	run storage --debug=false mount $lowerlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	lowermwriter=$(cat ${TESTDIR}/runroot/${STORAGE_DRIVER}-layers/mountpoints.lock)
	[ "$lowermwriter" != "" ]
	# Put a file in the layer.
	createrandom "$lowermount"/layer1file1

	# Create a second layer based on the first one.
	run storage --debug=false create-layer "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midlayer="$output"
	midwriter=$(cat ${TESTDIR}/root/${STORAGE_DRIVER}-layers/layers.lock)
	[ "$midwriter" != "" ]
	# Mount that layer, too.
	run storage --debug=false mount $midlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midmount="$output"
	midmwriter=$(cat ${TESTDIR}/runroot/${STORAGE_DRIVER}-layers/mountpoints.lock)
	[ "$midmwriter" != "" ]
	# Check that the file from the first layer is there.
	test -s "$midmount"/layer1file1
	# Check that we can remove it...
	rm -f -v "$midmount"/layer1file1
	# ... and that doing so doesn't affect the first layer.
	test -s "$lowermount"/layer1file1
	# Create a new file in this layer.
	createrandom "$midmount"/layer2file1
	# Unmount this layer.
	storage unmount $midlayer
	# Unmount the first layer.
	storage unmount $lowerlayer

	# Create a third layer based on the second one.
	run storage --debug=false create-layer "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer="$output"
	upperwriter=$(cat ${TESTDIR}/root/${STORAGE_DRIVER}-layers/layers.lock)
	[ "$upperwriter" != "" ]
	# Mount this layer.
	run storage --debug=false mount $upperlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	uppermount="$output"
	uppermwriter=$(cat ${TESTDIR}/runroot/${STORAGE_DRIVER}-layers/mountpoints.lock)
	[ "$uppermwriter" != "" ]
	# Check that the file we removed from the second layer is still gone.
	run test -s "$uppermount"/layer1file1
	[ "$status" -ne 0 ]
	# Check that the file we added to the second layer is still there.
	test -s "$uppermount"/layer2file1
	# Unmount the third layer.
	storage unmount $upperlayer

	# Get a list of the layers, and make sure all three, and no others, are listed.
	run storage --debug=false layers
	[ "$status" -eq 0 ]
	echo :"$output":
	[ "${#lines[*]}" -eq 3 ]
	[ "${lines[0]}" != "${lines[1]}" ]
	[ "${lines[1]}" != "${lines[2]}" ]
	[ "${lines[2]}" != "${lines[0]}" ]
	[ "${lines[0]}" = "$lowerlayer" ] || [ "${lines[0]}" = "$midlayer" ] || [ "${lines[0]}" = "$upperlayer" ]
	[ "${lines[1]}" = "$lowerlayer" ] || [ "${lines[1]}" = "$midlayer" ] || [ "${lines[1]}" = "$upperlayer" ]
	[ "${lines[2]}" = "$lowerlayer" ] || [ "${lines[2]}" = "$midlayer" ] || [ "${lines[2]}" = "$upperlayer" ]

	# Check that we updated the layers last-writer consistently.
	[ "${lowerwriter}" != "${midwriter}" ]
	[ "${lowerwriter}" != "${upperwriter}" ]
	[ "${midwriter}" != "${upperwriter}" ]

	# Check that we updated the mountpoints last-writer consistently.
	[ "${lowermwriter}" != "${midmwriter}" ]
	[ "${lowermwriter}" != "${uppermwriter}" ]
	[ "${midmwriter}" != "${uppermwriter}" ]
}
