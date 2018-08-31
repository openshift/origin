#!/bin/bash
#
# This abstracts starting up an extended server.

# If invoked with arguments, executes the test directly.
function os::test::extended::focus () {
	if [[ "$@[@]" =~ "ginkgo.focus" ]]; then
		os::log::fatal "the --ginkgo.focus flag is no longer supported, use FOCUS=foo <suite.sh> instead."
		exit 1
	fi
	if [[ "$@[@]" =~ "-suite" ]]; then
		os::log::fatal "the -suite flag is no longer supported, use SUITE=foo instead."
		exit 1
	fi
	if [[ -n "${FOCUS:-}" ]]; then
		exitstatus=0

		# first run anything that isn't explicitly declared [Serial], and matches the $FOCUS, in a parallel mode.
		os::log::info "Running parallel tests N=${PARALLEL_NODES:-<default>} with focus ${FOCUS}"
		TEST_REPORT_FILE_NAME=focus_parallel TEST_PARALLEL="${PARALLEL_NODES:-5}" os::test::extended::run -- -ginkgo.skip "\[Serial\]" -test.timeout 6h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

		# Then run everything that requires serial and matches the $FOCUS, serially.
		# there is bit of overlap here because not all serial tests declare [Serial], so they might have run in the
		# parallel section above.  Hopefully your focus was precise enough to exclude them, and we should be adding
		# the [Serial] tag to them as needed.
		os::log::info ""
		os::log::info "Running serial tests with focus ${FOCUS}"
		t=$FOCUS
		FOCUS="\[Serial\].*?${t}"
		TEST_REPORT_FILE_NAME=focus_serial os::test::extended::run -- -test.timeout 6h ${TEST_EXTENDED_ARGS-} || exitstatus=$?
		FOCUS="${t}.*?\[Serial\]"
		TEST_REPORT_FILE_NAME=focus_serial2 os::test::extended::run -- -test.timeout 6h ${TEST_EXTENDED_ARGS-} || exitstatus=$?

		exit $exitstatus
	fi
}

# Launches an extended server for OpenShift
# TODO: this should be doing less, because clusters should be stood up outside
#		and then tests are executed.	Tests that depend on fine grained setup should
#		be done in other contexts.
function os::test::extended::setup () {
	# build binaries
	os::util::ensure::built_binary_exists 'ginkgo' 'vendor/github.com/onsi/ginkgo/ginkgo'
	os::util::ensure::built_binary_exists 'extended.test' 'test/extended/extended.test'
	os::util::ensure::built_binary_exists 'oc'

	# ensure proper relative directories are set
	export KUBE_REPO_ROOT="${OS_ROOT}/vendor/k8s.io/kubernetes"

	os::util::environment::setup_time_vars

	# Allow setting $JUNIT_REPORT to toggle output behavior
	if [[ -n "${JUNIT_REPORT:-}" ]]; then
		# the Ginkgo tests also generate jUnit but expect different envars
		export TEST_REPORT_DIR="${ARTIFACT_DIR}/junit"
		mkdir -p $TEST_REPORT_DIR
	fi

	function cleanup() {
		return_code=$?
		os::test::junit::generate_report
		os::util::describe_return_code "${return_code}"
		exit "${return_code}"
	}
	trap "cleanup" EXIT

	os::log::info "Running tests against existing cluster..."
	return 0
}

# Run extended tests or print out a list of tests that need to be run
# Input:
# - FOCUS - the extended test focus
# - SKIP - the tests to skip
# - TEST_EXTENDED_SKIP - a global filter that allows additional tests to be omitted, will
#     be joined with SKIP
# - SHOW_ALL - if set, then only print out tests to be run
# - TEST_PARALLEL - if set, run the tests in parallel with the specified number of nodes
# - Arguments - arguments to pass to ginkgo
function os::test::extended::run () {
	local listArgs=()
	local runArgs=()

	if [[ -n "${FOCUS-}" ]]; then
		listArgs+=("--ginkgo.focus=${FOCUS}")
		runArgs+=("-focus=${FOCUS}")
	elif [[ -n "${SUITE-}" ]]; then
		listArgs+=("--ginkgo.focus=${SUITE}")
		runArgs+=("-focus=${SUITE}")
	fi

	local skip="${SKIP-}"
	# Allow additional skips to be provided on the command line
	if [[ -n "${TEST_EXTENDED_SKIP-}" ]]; then
		if [[ -n "${skip}" ]]; then
			skip="${skip}|${TEST_EXTENDED_SKIP}"
		else
			skip="${TEST_EXTENDED_SKIP}"
		fi
	fi
	if [[ -n "${skip}" ]]; then
		listArgs+=("--ginkgo.skip=${skip}")
		runArgs+=("-skip=${skip}")
	fi

	if [[ -n "${TEST_PARALLEL-}" ]]; then
		runArgs+=("-p" "-nodes=${TEST_PARALLEL}")
	fi

	if [[ -n "${SHOW_ALL-}" ]]; then
		PRINT_TESTS=1
		os::test::extended::test_list "${listArgs[@]:+"${listArgs[@]}"}"
		return
	fi

	os::test::extended::test_list "${listArgs[@]:+"${listArgs[@]}"}"

	if [[ "${TEST_COUNT}" -eq 0 ]]; then
		os::log::warning "No tests were selected"
		return
	fi

	ginkgo -v -noColor "${runArgs[@]:+"${runArgs[@]}"}" "$( os::util::find::built_binary extended.test )" "$@"
}

# Create a list of extended tests to be run with the given arguments
# Input:
# - Arguments to pass to ginkgo
# - SKIP_ONLY - If set, only selects tests to be skipped
# - PRINT_TESTS - If set, print the list of tests
# Output:
# - TEST_COUNT - the number of tests selected by the arguments
function os::test::extended::test_list () {
	local full_test_list=()
	local selected_tests=()

	while IFS= read -r; do
		full_test_list+=( "${REPLY}" )
	done < <(TEST_OUTPUT_QUIET=true extended.test "$@" --ginkgo.dryRun --ginkgo.noColor )
	if [[ "${REPLY}" ]]; then lines+=( "$REPLY" ); fi

	for test in "${full_test_list[@]}"; do
		if [[ -n "${SKIP_ONLY:-}" ]]; then
			if grep -q "35mskip" <<< "${test}"; then
				selected_tests+=( "${test}" )
			fi
		else
			if grep -q "1mok" <<< "${test}"; then
				selected_tests+=( "${test}" )
			fi
		fi
	done
	if [[ -n "${PRINT_TESTS:-}" ]]; then
		if [[ ${#selected_tests[@]} -eq 0 ]]; then
			os::log::warning "No tests were selected"
		else
			printf '%s\n' "${selected_tests[@]}" | sort
		fi
	fi
	export TEST_COUNT=${#selected_tests[@]}
}
readonly -f os::test::extended::test_list
