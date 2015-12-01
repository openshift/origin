#!/bin/bash
# This utility file contains functions that wrap commands to be tested. All wrapper functions run commands
# in a sub-shell and redirect all output. Tests in test-cmd *must* use these functions for testing.

# We assume ${OS_ROOT} is set
source "${OS_ROOT}/hack/text.sh" 

# expect_success runs the cmd and expects an exit code of 0
function os::cmd::expect_success() {
	if [[ $# -ne 1 ]]; then echo "os::cmd::expect_success expects only one argument, got $#"; exit 1; fi
	cmd=$1

	os::cmd::internal::expect_exit_code_run_grep "${cmd}"
}

# expect_failure runs the cmd and expects a non-zero exit code
function os::cmd::expect_failure() {
	if [[ $# -ne 1 ]]; then echo "os::cmd::expect_failure expects only one argument, got $#"; exit 1; fi
	cmd=$1

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::failure_func"
}

# expect_success_and_text runs the cmd and expects an exit code of 0
# as well as running a grep test to find the given string in the output
function os::cmd::expect_success_and_text() {
	if [[ $# -ne 2 ]]; then echo "os::cmd::expect_success_and_text expects two arguments, got $#"; exit 1; fi
	cmd=$1
	expected_text=$2

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::success_func" "${expected_text}"
}

# expect_failure_and_text runs the cmd and expects a non-zero exit code
# as well as running a grep test to find the given string in the output
function os::cmd::expect_failure_and_text() {
	if [[ $# -ne 2 ]]; then echo "os::cmd::expect_failure_and_text expects two arguments, got $#"; exit 1; fi
	cmd=$1
	expected_text=$2

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::failure_func" "${expected_text}"
}

# expect_success_and_not_text runs the cmd and expects an exit code of 0
# as well as running a grep test to ensure the given string is not in the output
function os::cmd::expect_success_and_not_text() {
	if [[ $# -ne 2 ]]; then echo "os::cmd::expect_success_and_not_text expects two arguments, got $#"; exit 1; fi
	cmd=$1
	expected_text=$2

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::success_func" "${expected_text}" "os::cmd::internal::failure_func"
}

# expect_failure_and_not_text runs the cmd and expects a non-zero exit code
# as well as running a grep test to ensure the given string is not in the output
function os::cmd::expect_failure_and_not_text() {
	if [[ $# -ne 2 ]]; then echo "os::cmd::expect_failure_and_not_text expects two arguments, got $#"; exit 1; fi
	cmd=$1
	expected_text=$2

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::failure_func" "${expected_text}" "os::cmd::internal::failure_func"
}

# expect_code runs the cmd and expects a given exit code
function os::cmd::expect_code() {
	if [[ $# -ne 2 ]]; then echo "os::cmd::expect_code expects two arguments, got $#"; exit 1; fi
	cmd=$1
	expected_cmd_code=$2

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::specific_code_func ${expected_cmd_code}"
}

# expect_code_and_text runs the cmd and expects the given exit code
# as well as running a grep test to find the given string in the output
function os::cmd::expect_code_and_text() {
	if [[ $# -ne 3 ]]; then echo "os::cmd::expect_code_and_text expects three arguments, got $#"; exit 1; fi
	cmd=$1
	expected_cmd_code=$2
	expected_text=$3

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::specific_code_func ${expected_cmd_code}" "${expected_text}"
}

# expect_code_and_not_text runs the cmd and expects the given exit code
# as well as running a grep test to ensure the given string is not in the output
function os::cmd::expect_code_and_not_text() {
	if [[ $# -ne 3 ]]; then echo "os::cmd::expect_code_and_not_text expects three arguments, got $#"; exit 1; fi
	cmd=$1
	expected_cmd_code=$2
	expected_text=$3

	os::cmd::internal::expect_exit_code_run_grep "${cmd}" "os::cmd::internal::specific_code_func ${expected_cmd_code}" "${expected_text}" "os::cmd::internal::failure_func"
}

millisecond=1
second=$(( 1000 * millisecond ))
minute=$(( 60 * second ))

# os::cmd::try_until_success runs the cmd in a small interval until either the command succeeds or times out
# the default time-out for os::cmd::try_until_success is 60 seconds.
# the default interval for os::cmd::try_until_success is 200ms
function os::cmd::try_until_success() {
	if [[ $# -lt 1 ]]; then echo "os::cmd::try_until_success expects at least one arguments, got $#"; exit 1; fi
	cmd=$1
	duration=${2:-minute}
	interval=${3:-0.2}

	os::cmd::internal::run_until_exit_code "${cmd}" "os::cmd::internal::success_func" "${duration}" "${interval}"
}

# os::cmd::try_until_failure runs the cmd until either the command fails or times out
# the default time-out for os::cmd::try_until_failure is 60 seconds.
function os::cmd::try_until_failure() {
	if [[ $# -lt 1 ]]; then echo "os::cmd::try_until_success expects at least one argument, got $#"; exit 1; fi
	cmd=$1
	duration=${2:-$minute}
	interval=${3:-0.2}

	os::cmd::internal::run_until_exit_code "${cmd}" "os::cmd::internal::failure_func" "${duration}" "${interval}"
}

# os::cmd::try_until_text runs the cmd until either the command outputs the desired text or times out
# the default time-out for os::cmd::try_until_text is 60 seconds.
function os::cmd::try_until_text() {
	if [[ $# -lt 2 ]]; then echo "os::cmd::try_until_success expects at least two arguments, got $#"; exit 1; fi
	cmd=$1
	text=$2
	duration=${3:-minute}
	interval=${4:-0.2}

	echo os::cmd::internal::run_until_text "${cmd}" "${text}" "${duration}" "${interval}"

	os::cmd::internal::run_until_text "${cmd}" "${text}" "${duration}" "${interval}"
}

# Functions in the os::cmd::internal namespace are discouraged from being used outside of os::cmd

# In order to harvest stderr and stdout at the same time into different buckets, we need to stick them into files 
# in an intermediate step
os_cmd_internal_tmpdir="/tmp/openshift/test/cmd"
os_cmd_internal_tmpout="${os_cmd_internal_tmpdir}/tmp_stdout.log"
os_cmd_internal_tmperr="${os_cmd_internal_tmpdir}/tmp_stderr.log"

# os::cmd::internal::expect_exit_code_and_text runs the provided test command and expects a specific 
# exit code from that command as well as the success of a specified `grep` invocation. Output from the 
# command to be tested is suppressed unless either `VERBOSE=1` or the test fails. This function bypasses
# any error exiting settings or traps set by upstream callers by masking the return code of the command 
# with the return code of setting the result variable on failure.
function os::cmd::internal::expect_exit_code_run_grep() {
	cmd=$1
	# default expected cmd code to 0 for success
	cmd_eval_func=${2:-os::cmd::internal::success_func}
	# default to nothing 
	grep_args=${3:-} 
	# default expected test code to 0 for success
	test_eval_func=${4:-os::cmd::internal::success_func}

	os::cmd::internal::init_tempdir

	echo "Running  ${cmd}..."
	
	cmd_result=$( os::cmd::internal::run_collecting_output "${cmd}"; echo $? )
	cmd_succeeded=$( ${cmd_eval_func} "${cmd_result}"; echo $? )

	test_result=0
	if [[ -n "${grep_args}" ]]; then
		test_result=$( os::cmd::internal::run_collecting_output 'os::cmd::internal::get_results | grep -Eq "${grep_args}"'; echo $? )
		
	fi
	test_succeeded=$( ${test_eval_func} "${test_result}"; echo $? )

	os::text::clear_last_line

	if (( cmd_succeeded && test_succeeded )); then

		os::text::print_green_bold "SUCCESS: ${cmd}"
		if [[ -n ${VERBOSE-} ]]; then
			os::cmd::internal::print_results
		fi
		return 0
	else
		cause=$(os::cmd::internal::assemble_causes "${cmd_succeeded}" "${test_succeeded}")
		
		os::text::print_red_bold "FAILURE: ${cmd}: ${cause}"
		os::text::print_red "$(os::cmd::internal::print_results)"
		return 1
	fi
}

# os::cmd::internal::init_tempdir initializes the temporary directory 
function os::cmd::internal::init_tempdir() {
	mkdir -p "${os_cmd_internal_tmpdir}"
	rm -f "${os_cmd_internal_tmpdir}"/tmp_std{out,err}.log
}

# os::cmd::internal::run_collecting_output runs the command given, piping stdout and stderr into
# the given files, and returning the exit code of the command
function os::cmd::internal::run_collecting_output() {
	cmd=$1

	local result=
	$( eval "${cmd}" 1>>"${os_cmd_internal_tmpout}" 2>>"${os_cmd_internal_tmperr}" ) || result=$?
	result=${result:-0} # if we haven't set result yet, the command succeeded

	return "${result}"
} 

# os::cmd::internal::success_func determines if the input exit code denotes success
# this function returns 0 for false and 1 for true to be compatible with arithmetic tests
function os::cmd::internal::success_func() {
	exit_code=$1

	# use a negated test to get output correct for (( ))
	[[ "${exit_code}" -ne "0" ]]
	return $?
}

# os::cmd::internal::failure_func determines if the input exit code denotes failure
# this function returns 0 for false and 1 for true to be compatible with arithmetic tests
function os::cmd::internal::failure_func() {
	exit_code=$1

	# use a negated test to get output correct for (( ))
	[[ "${exit_code}" -eq "0" ]]
	return $?
}

# os::cmd::internal::specific_code_func determines if the input exit code matches the given code
# this function returns 0 for false and 1 for true to be compatible with arithmetic tests
function os::cmd::internal::specific_code_func() {
	expected_code=$1
	exit_code=$2

	# use a negated test to get output correct for (( ))
	[[ "${exit_code}" -ne "${expected_code}" ]]
	return $?
}

# os::cmd::internal::get_results prints the stderr and stdout files
function os::cmd::internal::get_results() {
	cat "${os_cmd_internal_tmpout}" "${os_cmd_internal_tmperr}"
}

# os::cmd::internal::print_results pretty-prints the stderr and stdout files
function os::cmd::internal::print_results() {
	if [[ -s "${os_cmd_internal_tmpout}" ]]; then 
		echo "Standard output from the command:"
		cat "${os_cmd_internal_tmpout}"
	else 
		echo "There was no output from the command."                                      																																																																																																																
	fi	

	if [[ -s "${os_cmd_internal_tmperr}" ]]; then 
		echo "Standard error from the command:"
		cat "${os_cmd_internal_tmperr}"
	else 
		echo "There was no error output from the command."                                      																																																																																																																
	fi	
}

# os::cmd::internal::assemble_causes determines from the two input booleans which part of the test
# failed and generates a nice delimited list of failure causes
function os::cmd::internal::assemble_causes() {
	cmd_succeeded=$1
	test_succeeded=$2

	causes=()
	if (( ! cmd_succeeded )); then
		causes+=("the command returned the wrong error code")
	fi
	if (( ! test_succeeded )); then
		causes+=("the output content test failed")
	fi

	list=$(printf '; %s' "${causes[@]}")
	echo "${list:2}"
}


# os::cmd::internal::run_until_exit_code runs the provided command until the exit code test given 
# succeeds or the timeout given runs out. Output from the command to be tested is suppressed unless
# either `VERBOSE=1` or the test fails. This function bypasses any error exiting settings or traps
# set by upstream callers by masking the return code of the command with the return code of setting
# the result variable on failure.
function os::cmd::internal::run_until_exit_code() {
	cmd=$1
	cmd_eval_func=$2
	duration=$3
	interval=$4

	os::cmd::internal::init_tempdir

	echo "Waiting on ${cmd}..."
	
	deadline=$(( $(date +%s000) + $duration ))
	while [ $(date +%s000) -lt $deadline ]; do	
		cmd_result=$( os::cmd::internal::run_collecting_output "${cmd}"; echo $? )
		cmd_succeeded=$( ${cmd_eval_func} "${cmd_result}"; echo $? )
		if (( cmd_succeeded )); then
			break
		fi
		sleep "${interval}"
	done

	os::text::clear_last_line

	if (( cmd_succeeded )); then

		os::text::print_green_bold "SUCCESS: ${cmd}"
		if [[ -n ${VERBOSE-} ]]; then
			os::cmd::internal::print_results
		fi
		return 0
	else
		cause=$(os::cmd::internal::assemble_try_until_code_causes "${cmd_succeeded}")

		os::text::print_red_bold "FAILURE: ${cmd}: ${cause}"
		os::text::print_red "$(os::cmd::internal::print_results)"
		return 1
	fi
}

# os::cmd::internal::assemble_try_until_code_causes determines from the input boolean which part of the try untik
# failed and generates a nice delimited list of failure causes
function os::cmd::internal::assemble_try_until_code_causes() {
	cmd_succeeded=$1

	causes=()
	if (( ! cmd_succeeded )); then
		causes+=("the command returned the wrong error code")
	else
		causes+=("the command timed out")
	fi

	list=$(printf '; %s' "${causes[@]}")
	echo "${list:2}"
}

# os::cmd::internal::run_until_text runs the provided command until the command output contains the
# given text or the timeout given runs out. Output from the command to be tested is suppressed unless
# either `VERBOSE=1` or the test fails. This function bypasses any error exiting settings or traps
# set by upstream callers by masking the return code of the command with the return code of setting
# the result variable on failure.
function os::cmd::internal::run_until_text() {
	cmd=$1
	text=$2
	duration=$3
	interval=$4

	os::cmd::internal::init_tempdir

	echo "Waiting on ${cmd}..."
	
	deadline=$(( $(date +%s000) + $duration ))
	while [ $(date +%s000) -lt $deadline ]; do	
		cmd_result=$( os::cmd::internal::run_collecting_output "${cmd}"; echo $? )
		test_result=$( os::cmd::internal::run_collecting_output 'os::cmd::internal::get_results | grep -Eq "${text}"'; echo $? )
		test_succeeded=$( ${test_eval_func} "${test_result}"; echo $? )

		if (( test_succeeded )); then
			break
		fi
		sleep "${interval}"
	done

	os::text::clear_last_line

	if (( test_succeeded )); then

		os::text::print_green_bold "SUCCESS: ${cmd}"
		if [[ -n ${VERBOSE-} ]]; then
			os::cmd::internal::print_results
		fi
		return 0
	else
		cause=$(os::cmd::internal::assemble_try_until_text_causes "${test_succeeded}")
		
		os::text::print_red_bold "FAILURE: ${cmd}: ${cause}"
		os::text::print_red "$(os::cmd::internal::print_results)"
		return 1
	fi
}

# os::cmd::internal::assemble_try_until_text_causes determines from the input boolean which part of the try untik
# failed and generates a nice delimited list of failure causes
function os::cmd::internal::assemble_try_until_text_causes() {
	test_succeeded=$1

	causes=()
	if (( ! test_succeeded )); then
		causes+=("the output content test failed")
	else
		causes+=("the command timed out")
	fi

	list=$(printf '; %s' "${causes[@]}")
	echo "${list:2}"
}