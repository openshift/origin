#!/usr/bin/env bats

load helpers

@test "import-layer-ostree" {
	case "$STORAGE_DRIVER" in
	overlay*|vfs)
		;;
	*)
		skip "not supported by driver $STORAGE_DRIVER"
		;;
	esac

	# The checkdiffs function needs "tar".
	if test -z "$(which tar 2> /dev/null)" ; then
		skip "need tar"
	fi
	if test -z "$(which ostree 2> /dev/null)" ; then
		skip "need ostree"
	fi

	OSTREE_REPO=${TESTDIR}/ostree
	mkdir -p $OSTREE_REPO

	# Create and populate three interesting layers.
	populate

	OPTS="--storage-opt=.ostree_repo=$OSTREE_REPO,.skip_mount_home=true"

	# Extract the layers.
	storage diff $OPTS -u -f $TESTDIR/lower.tar $lowerlayer
	storage diff $OPTS -c -f $TESTDIR/middle.tar $midlayer
	storage diff $OPTS -u -f $TESTDIR/upper.tar $upperlayer

	# Delete the layers.
	storage delete-layer $OPTS $upperlayer
	storage delete-layer $OPTS $midlayer
	storage delete-layer $OPTS $lowerlayer

	# Import new layers using the layer diffs and mark them read-only.  Only read-only layers are deduplicated with OSTree.
	run storage --debug=false import-layer $OPTS -r -f $TESTDIR/lower.tar
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"

	run storage --debug=false import-layer $OPTS -r -f $TESTDIR/middle.tar "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midlayer="$output"

	run storage --debug=false import-layer $OPTS -r -f $TESTDIR/upper.tar "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer="$output"

	# The contents of these new layers should match what the old ones had.
	checkchanges
	checkdiffs
}
