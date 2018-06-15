#!/bin/bash

STORAGE_BINARY=${STORAGE_BINARY:-$(dirname ${BASH_SOURCE})/../containers-storage}
TESTSDIR=${TESTSDIR:-$(dirname ${BASH_SOURCE})}

function setup() {
	suffix=$(dd if=/dev/urandom bs=12 count=1 status=none | base64 | tr +/ _.)
	TESTDIR=${BATS_TMPDIR}/tmp.${suffix}
	rm -fr ${TESTDIR}
	mkdir -p ${TESTDIR}/{root,runroot}
	REPO=${TESTDIR}/root
}

function teardown() {
	rm -fr ${TESTDIR}
}

function storage() {
	${STORAGE_BINARY} --debug --root ${TESTDIR}/root --runroot ${TESTDIR}/runroot "$@"
}
