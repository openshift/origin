#!/usr/bin/env bats

load helpers

@test "status" {
	run storage --debug=false status
	echo :"$output":
	[ "$status" -eq 0 ]
	# Expect the first line of the output to be the storage root directory location.
	[ "${lines[0]/:*/}" = "Root" ]
	[ "${lines[0]/*: /}" = "${TESTDIR}/root" ]
	# Expect the second line of the output to be the storage runroot directory location.
	[ "${lines[1]/:*/}" = "Run Root" ]
	[ "${lines[1]/*: /}" = "${TESTDIR}/runroot" ]
	# Expect the third line of the output to be "Driver Name: $STORAGE_DRIVER".
	[ "${lines[2]/:*/}" = "Driver Name" ]
	[ "${lines[2]/*: /}" = "$STORAGE_DRIVER" ]
}
