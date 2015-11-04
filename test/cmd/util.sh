#!/bin/bash
# This utility file contains functions that wrap commands to be tested and allow us to interface 
# our command-line tests with JUnit on Jenkins. All wrapper functions run commands in a sub-shell 
# and redirect all output. Tests in test-cmd *must* use these functions for testing.

###################################################################################################
########################           HIGH-LEVEL WRAPPER FUNCTIONS           #########################
###################################################################################################

# expect_success runs the cmd and expects an exit code of 0
function test::cmd::expect_success() {
	cmd=$1

	test::cmd::expect_code "${cmd}" 0
}

# expect_failure runs the cmd and expects an exit code of 1
function test::cmd::expect_failure() {
	cmd=$1

	test::cmd::expect_code "${cmd}" 1
}

# expect_success_and_text runs the cmd and expects an exit code of 0
# as well as running a grep test to find the given string in the output
function test::cmd::expect_success_and_text() {
	cmd=$1
	expected_text=$2

	test::cmd::expect_code_and_text "${cmd}" 0 "${expected_text}"
}

# expect_failure_and_text runs the cmd and expects an exit code of 1
# as well as running a grep test to find the given string in the output
function test::cmd::expect_failure_and_text() {
	cmd=$1
	expected_text=$2

	test::cmd::expect_code_and_text "${cmd}" 1 "${expected_text}"
}

# expect_success_and_not_text runs the cmd and expects an exit code of 0
# as well as running a grep test to ensure the given string is not in the output
function test::cmd::expect_success_and_not_text() {
	cmd=$1
	expected_text=$2

	test::cmd::expect_code_and_not_text "${cmd}" 0 "${expected_text}"
}

# expect_failure_and_not_text runs the cmd and expects an exit code of 1
# as well as running a grep test to ensure the given string is not in the output
function test::cmd::expect_failure_and_not_text() {
	cmd=$1
	expected_text=$2

	test::cmd::expect_code_and_not_text "${cmd}" 1 "${expected_text}"
}

###################################################################################################
########################          MEDIUM-LEVEL WRAPPER FUNCTIONS          #########################
###################################################################################################

# expect_code runs the cmd and expects a given exit code
function test::cmd::expect_code() {
	cmd=$1
	expected_cmd_code=$2

	test::cmd::expect_exit_code_run_grep "${cmd}" "${expected_cmd_code}"
}

# expect_code_and_text runs the cmd and expects the given exit code
# as well as running a grep test to find the given string in the output
function test::cmd::expect_code_and_text() {
	cmd=$1
	expected_cmd_code=$2
	expected_text=$3

	test::cmd::expect_exit_code_run_grep "${cmd}" "${expected_cmd_code}" "${expected_text}"
}

# expect_code_and_not_text runs the cmd and expects the given exit code
# as well as running a grep test to ensure the given string is not in the output
function test::cmd::expect_code_and_not_text() {
	cmd=$1
	expected_cmd_code=$2
	expected_text=$3
	expected_test_code=1 # failure

	test::cmd::expect_exit_code_run_grep "${cmd}" "${expected_cmd_code}" "${expected_text}" "${expected_test_code}"
}

###################################################################################################
########################             TEST EXECUTION FUNCTION              #########################
###################################################################################################

# expect_exit_code_and_text runs the provided test command and expects a specific exit code from that
# command as well as the success of a specified `grep` invocation. Output from the command to be 
# tested is suppressed unless either `VERBOSE=1` or the test fails.
function test::cmd::expect_exit_code_run_grep() {
	cmd=$1
	# default expected cmd code to 0 for success
	expected_cmd_code=${2:-'0'}
	# default grep args to a something that always passes, even for zero-length input
	grep_args=${3:-'$'} 
	# default expected test code to 0 for success
	expected_test_code=${4:-'0'}

	local startTime endTime timeElapsed

 	declare_test_start "${cmd}" "${expected_cmd_code}" "${grep_args}" "${expected_test_code}">> ${JUNIT_OUTPUT_FILE}
	echo "Running  ${cmd}..."
	
	startTime=$(ms_since_epoch)

	cmd_output=$(${cmd} 2>&1)
	cmd_result=$?

	test_output=$(echo "${cmd_output}" | grep -E "${grep_args}" 2>&1)
	test_result=$?

	endTime=$(ms_since_epoch)
	timeElapsed=$(bc <<< "scale=9; ${endTime} - ${startTime}")

	# use negated tests in order to get the output right for (( ))
	cmd_succeeded=$( [[ "${cmd_result}" -ne "${expected_cmd_code}" ]]; echo $? )
	test_succeeded=$( [[ "${test_result}" -ne "${expected_test_code}" ]]; echo $? )
	
	if (( $cmd_succeeded && $test_succeeded )); then
		# output for humans
		overwrite_green "SUCCESS: ${cmd}"

		if [[ -n ${VERBOSE-} ]]; then
			print_results "${cmd_output}" "${test_output}"
		fi

		# output for jUnit
		print_results "${cmd_output}" "${test_output}" >> ${JUNIT_OUTPUT_FILE}
		report_test_outcomes "PASS" "${timeElapsed}" >> ${JUNIT_OUTPUT_FILE}
		return 0
	else
		cause=$(assemble_causes "${cmd_succeeded}" "${test_succeeded}")
		
		# output for humans		
		overwrite_red "FAILURE: ${cmd}: ${cause}"
		print_results "${cmd_output}" "${test_output}" | tee -a ${JUNIT_OUTPUT_FILE}

		# output for jUnit
		report_test_outcomes "FAIL" "${timeElapsed}" >> ${JUNIT_OUTPUT_FILE}
		return 1
	fi
}

# ms_since_epoch returns the current time as number of milliseconds since the epoch
# with nano-second precision
function ms_since_epoch() {
	ns=$(date +%s%N)
	echo $(bc <<< "scale=9; ${ns}/1000000")
}

# colors for test output
readonly      reset=$(tput sgr0)
readonly        red=$(tput setaf 1)
readonly      green=$(tput setaf 2)
readonly clear_last=$(tput cuu 1 && tput el)

# overwrite_red overwrites the last line with the given intput in green
function overwrite_green() {
	echo ${clear_last}${green}${@}${reset}
}

# overwrite_red overwrites the last line with the given intput in red
function overwrite_red() {
	echo ${clear_last}${red}${@}${reset}
}

# print_results pretty-prints the output given to it
function print_results() {
	cmd_output=$1
	test_output=$2

	if [[ -n "${cmd_output}" ]]; then 
		echo "Output from the command:"
		echo "${cmd_output}"
	else 
		echo "There was no output from the command"                                      																																																																																																																
	fi

	if [[ -n "${test_output}" ]]; then 
		echo "Output from the output content test:"
		echo "${test_output}"
	else 
		echo "There was no output from the content test"
	fi	
}

# assemble_causes determines from the two input booleans which part of the test
# failed and generates a nice delimited list of failure causes
function assemble_causes() {
	cmd_succeeded=$1
	test_succeeded=$2

	causes=()
	if (( ! $cmd_succeeded )); then
		causes+=("the command returned the wrong error code")
	fi
	if (( ! $test_succeeded )); then
		causes+=("the output content test failed")
	fi

	echo $(list=$(printf '; %s' "${causes[@]}"); echo "${list:2}")
}

###################################################################################################
########################              JUNIT HELPER FUNCTIONS              #########################
###################################################################################################

# JUNIT_OUTPUT_FILE describes where jUnit-destined output should go. If unset, output is discarded
JUNIT_OUTPUT_FILE=${JUNIT_OUTPUT_FILE:-/dev/null}

# declare_test_start declares the beginning of a test for the jUnit output. This function test::cmd::walks 
# backward through ${BASH_SOURCE} in order to find the earliest file that isn't this one, and then
# determines that the file found is the source of the call to whichever test wrapper. This function
# then walks backwards through ${BASH_LINENO} to determine the line in that file that issued the test
# wrapper call and through ${FUNCNAME} to get the wrapper function test::cmd::called.
function test::junit::declare_test_start() {
	cmd=$1
	expected_cmd_code=$2
	grep_args=$3
	expected_test_code=$4

	callDepth=
	lenSources="${#BASH_SOURCE[@]}"
	for (( i=0; i<${lenSources}; i++ )); do
		if [ ! $(echo "${BASH_SOURCE[i]}" | grep -P "test/cmd/util.sh\$") ]; then
			callDepth=i
			break
		fi
	done

	testLocation="${BASH_SOURCE[${callDepth}]}:${BASH_LINENO[${callDepth}-1]}"
	wrapperFunc="${FUNCNAME[${callDepth}-1]}"

	args=$(assemble_args "${wrapperFunc}" "${cmd}" "${expected_cmd_code}" "${grep_args}" "${expected_test_code}")

	echo "==== BEGIN TEST AT ${testLocation}: ${wrapperFunc} ${args}  ===="
}

# assemble_args re-assembles the args given to the wrapper function
function assemble_args() {
	wrapperFunc=$1
	cmd=$2
	expected_cmd_code=$3
	grep_args=$4
	expected_test_code=$5

	case "${wrapperFunc}" in 
		"expect_code") 
			echo "'${cmd}' '${expected_cmd_code}'" ;;
		"expect_success") 
			echo "'${cmd}'" ;;
		"expect_failure") 
			echo "'${cmd}'" ;;
		"expect_code_and_text") 
			echo "'${cmd}' '${expected_cmd_code}' '${grep_args}'" ;;
		"expect_success_and_text") 
			echo "'${cmd}' '${grep_args}'" ;;
		"expect_failure_and_text")
			echo "'${cmd}' '${grep_args}'" ;;
		"expect_code_and_not_text")
			echo "'${cmd}' '${expected_cmd_code}' '${grep_args}'" ;;
		"expect_success_and_not_text") 
			echo "'${cmd}' '${grep_args}'" ;;
		"expect_failure_and_not_text")
			echo "'${cmd}' '${grep_args}'" ;;
		"expect_exit_code_run_grep")
			echo "'${cmd}' '${expected_cmd_code}' '${grep_args}' '${expected_test_code}'" ;;
	esac
}

# report_test_outcomes reports the outcomes of a teat for the jUnit output
function test::junit::report_test_outcomes() {
	result=$1
	timeElapsed=$2

	echo "==== END TEST: ${result} AFTER ${timeElapsed} MILLISECONDS ===="
}

# declare_package_start declares the beginning of a test suite for the jUnit output 
function test::junit::declare_package_start() {
	name=$1

	echo ">>>> BEGIN PACKAGE: ${name} <<<<"
}

# declare_package_start declares the end of a test suite for the jUnit output 
function test::junit::declare_package_end() {
	echo ">>>> END PACKAGE <<<<"
}
