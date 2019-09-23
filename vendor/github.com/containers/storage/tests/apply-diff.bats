#!/usr/bin/env bats

load helpers

@test "applydiff" {
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

	# Create new layers and populate them using the layer diffs.
	run storage --debug=false create-layer
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	lowerlayer="$output"
	storage applydiff -f $TESTDIR/lower.tar "$lowerlayer"

	run storage --debug=false create-layer "$lowerlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	midlayer="$output"
	storage applydiff -f $TESTDIR/middle.tar "$midlayer"

	run storage --debug=false create-layer "$midlayer"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	upperlayer="$output"
	storage applydiff -f $TESTDIR/upper.tar "$upperlayer"

	# The contents of these new layers should match what the old ones had.
	checkchanges
	checkdiffs
}
