#!/usr/bin/env bats

load helpers

@test "absolute-paths" {
	cd ${TESTDIR}
	storage --graph tmp1a/deep/root --run tmp1b/deep/runroot layers
	storage --graph ./tmp2a/deep/root --run ./tmp2b/deep/runroot layers
	storage --graph tmp1a/deep/root --run tmp1b/deep/runroot shutdown
	storage --graph ./tmp2a/deep/root --run ./tmp2b/deep/runroot shutdown
}
