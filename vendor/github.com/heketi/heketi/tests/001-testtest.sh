#!/bin/bash

# This "test" exists merely to exercise the test "framework"

if [ "$HEKETI_TEST_TEST" ]; then
	exit "$HEKETI_TEST_TEST"
fi
exit 0
