#!/usr/bin/env bats

load helpers

@test "import-layer" {
	# The checkdiffs function needs "tar".
	if test -z "$(which tar 2> /dev/null)" ; then
		skip "need tar"
	fi

	# Create and populate three interesting layers.
	populate

	# Extract the layers.
	storage diff -u -f $TESTDIR/lower.tar $lowerlayer
	storage diff -c -f $TESTDIR/middle.tar $midlayer
	storage diff -u -f $TESTDIR/upper.tar $upperlayer

	# Delete the layers.
	storage delete-layer $upperlayer
	storage delete-layer $midlayer
	storage delete-layer $lowerlayer

	# Import new layers using the layer diffs.
	run storage --debug=false import-layer -f $TESTDIR/lower.tar
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"

	run storage --debug=false import-layer -f $TESTDIR/middle.tar "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midlayer="$output"

	run storage --debug=false import-layer -f $TESTDIR/upper.tar "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer="$output"

	# The contents of these new layers should match what the old ones had.
	checkchanges
	checkdiffs
}
