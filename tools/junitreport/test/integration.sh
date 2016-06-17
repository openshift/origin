#!/bin/bash

# This file runs the integration tests for the `junitreport` binary to ensure that correct jUnit XML is produced.

set -o errexit
set -o nounset
set -o pipefail

JUNITREPORT_ROOT=$(dirname "${BASH_SOURCE}")/..
pushd "${JUNITREPORT_ROOT}" > /dev/null

diff_args='-yd --tabsize=4 --suppress-common-lines'

TMPDIR="/tmp/junitreport/test/integration"
mkdir -p "${TMPDIR}"

echo "[INFO] Building junitreport binary for testing..."
go build .

for suite in test/*/; do
	suite_name=$( basename ${suite} )
	echo "[INFO] Testing suite ${suite_name}..."

	WORKINGDIR="${TMPDIR}/${suite_name}"
	mkdir -p "${WORKINGDIR}"

	# test every case with flat and nested suites
	for test in ${suite}/testdata/*.txt; do
		test_name=$( basename ${test} '.txt' )

		cat "${test}" | ./junitreport -type "${suite_name}" -suites flat > "${WORKINGDIR}/${test_name}_flat.xml"
		if ! diff ${diff_args} "${suite}/reports/${test_name}_flat.xml" "${WORKINGDIR}/${test_name}_flat.xml"; then
			echo "[FAIL] Test '${test_name}' in suite '${suite_name}' failed for flat suite builder."
			exit 1
		fi

		cat "${test}" | ./junitreport -type "${suite_name}" -suites nested > "${WORKINGDIR}/${test_name}_nested.xml"
		if ! diff ${diff_args} "${suite}/reports/${test_name}_nested.xml" "${WORKINGDIR}/${test_name}_nested.xml"; then
			echo "[FAIL] Test '${test_name}' in suite '${suite_name}' failed for nested suite builder."
			exit 1
		fi

		cat "${WORKINGDIR}/${test_name}_flat.xml" | ./junitreport summarize > "${WORKINGDIR}/${test_name}_summary.txt"
		if ! diff ${diff_args} "${suite}/summaries/${test_name}_summary.txt" "${WORKINGDIR}/${test_name}_summary.txt"; then
			echo "[FAIL] Test '${test_name}' in suite '${suite_name}' failed to summarize flat XML."
		fi

		cat "${WORKINGDIR}/${test_name}_nested.xml" | ./junitreport summarize > "${WORKINGDIR}/${test_name}_summary.txt"
		if ! diff ${diff_args} "${suite}/summaries/${test_name}_summary.txt" "${WORKINGDIR}/${test_name}_summary.txt"; then
			echo "[FAIL] Test '${test_name}' in suite '${suite_name}' failed to summarize nested XML."
		fi
	done

	echo "[PASS] Test output type passed: ${suite_name}"
done

echo "[INFO] Testing restricted roots with nested suites..."
# test some cases with nested suites and given roots
cat "test/gotest/testdata/1.txt" | ./junitreport -type gotest -suites nested -roots package/name > "${TMPDIR}/gotest/1_nested_restricted.xml"
if ! diff ${diff_args} "test/gotest/reports/1_nested_restricted.xml" "${TMPDIR}/gotest/1_nested_restricted.xml"; then
	echo "[FAIL] Test '1' in suite 'gotest' failed for nested suite builder with restricted roots: 'package/name'."
	exit 1
fi

cat "test/oscmd/testdata/1.txt" | ./junitreport -type oscmd -suites nested -roots package/name > "${TMPDIR}/oscmd/1_nested_restricted.xml"
if ! diff ${diff_args} "test/oscmd/reports/1_nested_restricted.xml" "${TMPDIR}/oscmd/1_nested_restricted.xml"; then
	echo "[FAIL] Test '1' in suite 'oscmd' failed for nested suite builder with restricted roots: 'package/name'."
	exit 1
fi

cat "test/gotest/testdata/9.txt" | ./junitreport -type gotest -suites nested -roots package/name,package/other > "${TMPDIR}/gotest/9_nested_restricted.xml"
if ! diff ${diff_args} "test/gotest/reports/9_nested_restricted.xml" "${TMPDIR}/gotest/9_nested_restricted.xml"; then
	echo "[FAIL] Test '9' in suite 'gotest' failed for nested suite builder with restricted roots: 'package/name,package/other'."
	exit 1
fi

cat "test/oscmd/testdata/4.txt" | ./junitreport -type oscmd -suites nested -roots package/name,package/other > "${TMPDIR}/oscmd/4_nested_restricted.xml"
if ! diff ${diff_args} "test/oscmd/reports/4_nested_restricted.xml" "${TMPDIR}/oscmd/4_nested_restricted.xml"; then
	echo "[FAIL] Test '4' in suite 'oscmd' failed for nested suite builder with restricted roots: 'package/name,package/other'."
	exit 1
fi
echo "[PASS] Suite passed: restricted roots"

echo "[PASS] junitreport testing successful"
popd > /dev/null
