#!/usr/bin/env bats

load helpers

@test "image-data" {
	# Bail if "sha256sum" isn't available.
	if test -z "$(which sha256sum 2> /dev/null)" ; then
		skip "need sha256sum"
	fi

	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image using that layer.
	run storage --debug=false create-image $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Make sure the image can be located.
	storage exists -i $image

	# Make sure the image has no big data items associated with it.
	run storage --debug=false list-image-data $image
	[ "$status" -eq 0 ]
	[ "$output" = "" ]

	# Create two random files.
	createrandom $TESTDIR/big-item-1 1234
	createrandom $TESTDIR/big-item-2 5678

	# Set each of those files as a big data item named after the file.
	storage set-image-data -f $TESTDIR/big-item-1 $image big-item-1
	storage set-image-data -f $TESTDIR/big-item-2 $image big-item-2

	# Get a list of the items.  Make sure they're both listed.
	run storagewithsorting --debug=false list-image-data $image
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 2 ]
	[ "${lines[0]}" = "big-item-1" ]
	[ "${lines[1]}" = "big-item-2" ]

	# Check that the recorded sizes of the items match what we decided above.
	run storage get-image-data-size $image no-such-item
	[ "$status" -ne 0 ]
	run storage --debug=false get-image-data-size $image big-item-1
	[ "$status" -eq 0 ]
	[ "$output" -eq 1234 ]
	run storage --debug=false get-image-data-size $image big-item-2
	[ "$status" -eq 0 ]
	[ "$output" -eq 5678 ]

	# Save the contents of the big data items to disk and compare them with the originals.
	run storage --debug=false get-image-data $image no-such-item
	[ "$status" -ne 0 ]
	storage get-image-data -f $TESTDIR/big-item-1.2 $image big-item-1
	cmp $TESTDIR/big-item-1 $TESTDIR/big-item-1.2
	storage get-image-data -f $TESTDIR/big-item-2.2 $image big-item-2
	cmp $TESTDIR/big-item-2 $TESTDIR/big-item-2.2

	# Read the recorded digests of the items and compare them with the digests of the originals.
	run storage get-image-data-digest $image no-such-item
	[ "$status" -ne 0 ]
	run storage --debug=false get-image-data-digest $image big-item-1
	[ "$status" -eq 0 ]
	sum=$(sha256sum $TESTDIR/big-item-1)
	sum=sha256:"${sum%% *}"
	echo output:"$output":
	echo sum:"$sum":
	[ "$output" = "$sum" ]
	run storage --debug=false get-image-data-digest $image big-item-2
	[ "$status" -eq 0 ]
	sum=$(sha256sum $TESTDIR/big-item-2)
	sum=sha256:"${sum%% *}"
	echo output:"$output":
	echo sum:"$sum":
	[ "$output" = "$sum" ]
}

@test "container-data" {
	# Bail if "sha256sum" isn't available.
	if test -z "$(which sha256sum 2> /dev/null)" ; then
		skip "need sha256sum"
	fi

	# Create a layer.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	layer=$output

	# Create an image using that layer.
	run storage --debug=false create-image $layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	image=${output%%	*}

	# Create a container based on that image.
	run storage --debug=false create-container $image
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	container=${output%%	*}

	# Make sure the container can be located.
	storage exists -c $container

	# Make sure the container has no big data items associated with it.
	run storage --debug=false list-container-data $container
	[ "$status" -eq 0 ]
	[ "$output" = "" ]

	# Create two random files.
	createrandom $TESTDIR/big-item-1 1234
	createrandom $TESTDIR/big-item-2 5678

	# Set each of those files as a big data item named after the file.
	storage set-container-data -f $TESTDIR/big-item-1 $container big-item-1
	storage set-container-data -f $TESTDIR/big-item-2 $container big-item-2

	# Get a list of the items.  Make sure they're both listed.
	run storage --debug=false list-container-data $container
	[ "$status" -eq 0 ]
	[ "${#lines[*]}" -eq 2 ]
	[ "${lines[0]}" = "big-item-1" ]
	[ "${lines[1]}" = "big-item-2" ]

	# Check that the recorded sizes of the items match what we decided above.
	run storage get-container-data-size $container no-such-item
	[ "$status" -ne 0 ]
	run storage --debug=false get-container-data-size $container big-item-1
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1234 ]
	run storage --debug=false get-container-data-size $container big-item-2
	[ "$status" -eq 0 ]
	[ "$output" -eq 5678 ]

	# Save the contents of the big data items to disk and compare them with the originals.
	run storage --debug=false get-container-data $container no-such-item
	[ "$status" -ne 0 ]
	storage get-container-data -f $TESTDIR/big-item-1.2 $container big-item-1
	cmp $TESTDIR/big-item-1 $TESTDIR/big-item-1.2
	storage get-container-data -f $TESTDIR/big-item-2.2 $container big-item-2
	cmp $TESTDIR/big-item-2 $TESTDIR/big-item-2.2

	# Read the recorded digests of the items and compare them with the digests of the originals.
	run storage get-container-data-digest $container no-such-item
	[ "$status" -ne 0 ]
	run storage --debug=false get-container-data-digest $container big-item-1
	[ "$status" -eq 0 ]
	sum=$(sha256sum $TESTDIR/big-item-1)
	sum=sha256:"${sum%% *}"
	echo output:"$output":
	echo sum:"$sum":
	[ "$output" = "$sum" ]
	run storage --debug=false get-container-data-digest $container big-item-2
	[ "$status" -eq 0 ]
	sum=$(sha256sum $TESTDIR/big-item-2)
	sum=sha256:"${sum%% *}"
	echo output:"$output":
	echo sum:"$sum":
	[ "$output" = "$sum" ]
}
