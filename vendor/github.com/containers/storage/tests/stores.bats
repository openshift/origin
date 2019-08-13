#!/usr/bin/env bats

load helpers

@test "additional-stores" {
	case "$STORAGE_DRIVER" in
	overlay*|vfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac
	# Initialize a store somewhere that we'll later use as a read-only store.
	storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot layers
	# Fail this test if we can't initialize the driver with the option.
	storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root layers
	# Create a layer in what will become the read-only store.
	run storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	# Mount the layer in what will become the read-only store.
	run storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot --debug=false mount $lowerlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowermount="$output"
	# Put a file in the layer in what will become the read-only store.
	createrandom "$lowermount"/layer1file1

	# Create a second layer based on the first one in what will become the read-only store.
	run storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot --debug=false create-layer "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midlayer="$output"
	# Mount that layer, too.
	run storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot --debug=false mount $midlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midmount="$output"
	# Check that the file from the first layer is there.
	test -s "$midmount"/layer1file1
	# Check that we can remove it...
	rm -f -v "$midmount"/layer1file1
	# ... and that doing so doesn't affect the first layer.
	test -s "$lowermount"/layer1file1
	# Create a new file in this layer.
	createrandom "$midmount"/layer2file1
	# Unmount this layer.
	storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot unmount $midlayer
	# Unmount the first layer.
	storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot unmount $lowerlayer

	# Create an image using this second layer.
	run storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot --debug=false create-image $midlayer
        [ "$status" -eq 0 ]
        [ "$output" != "" ]
        lowerimage=${output%%  *}

	# We no longer need to use the read-only root as a writeable location, so shut it down.
	storage --graph ${TESTDIR}/ro-root --run ${TESTDIR}/ro-runroot shutdown

	# Create a third layer based on the second one.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false create-layer "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer="$output"
	# Mount this layer.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false mount $upperlayer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	uppermount="$output"
	# Check that the file we removed from the second layer is still gone.
	run test -s "$uppermount"/layer1file1
	[ "$status" -ne 0 ]
	# Check that the file we added to the second layer is still there.
	test -s "$uppermount"/layer2file1
	# Unmount the third layer.
	storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root unmount $upperlayer

	# Create an image using this third layer.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false create-image $upperlayer
        [ "$status" -eq 0 ]
        [ "$output" != "" ]
	upperimage=${output%%  *}
	# Create a container based on the upperimage.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false create-container "$upperimage"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	uppercontainer="$output"
	# Mount this container.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false mount $uppercontainer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	uppercontainermount="$output"
	# Check that the file we removed from the second layer is still gone.
	run test -s "$uppercontainermount"/layer1file1
	[ "$status" -ne 0 ]
	# Check that the file we added to the second layer is still there.
	test -s "$uppercontainermount"/layer2file1
	# Unmount the container.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false unmount $uppercontainer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Delete the container.
	storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root delete-container $uppercontainer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]

	# Create a container based on the lowerimage.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false create-container "$lowerimage"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container="$output"
	# Mount this container.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root --debug=false mount $container
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	containermount="$output"
	# Check that the file we removed from the second layer is still gone.
	run test -s "$containermount"/layer1file1
	[ "$status" -ne 0 ]
	# Check that the file we added to the second layer is still there.
	test -s "$containermount"/layer2file1
	# Unmount the container.
	storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root delete-container $container

	# Check that the first two layers and the first image are marked read-only, and that the rest are not.
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root layer $lowerlayer
        [ "$status" -eq 0 ]
        [ "$output" != "" ]
        [[ "$output" =~ "Read Only: true" ]]
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root layer $midlayer
        [ "$status" -eq 0 ]
        [ "$output" != "" ]
        [[ "$output" =~ "Read Only: true" ]]
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root layer $upperlayer
        [ "$status" -eq 0 ]
        [ "$output" != "" ]
        ! [[ "$output" =~ "Read Only: true" ]]
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root image $lowerimage
        [ "$status" -eq 0 ]
        [ "$output" != "" ]
        [[ "$output" =~ "Read Only: true" ]]
	run storage --storage-opt ${STORAGE_DRIVER}.imagestore=${TESTDIR}/ro-root image $upperimage
        [ "$status" -eq 0 ]
        [ "$output" != "" ]
        ! [[ "$output" =~ "Read Only: true" ]]
}
