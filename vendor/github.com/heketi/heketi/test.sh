#!/bin/bash

# main test runner for heketi
# Executes all executable scripts under tests dir
# in sorted order.

FAILURES=()

vecho () {
	if [[ "${verbose}" = "yes" ]] ; then
		echo "$*"
	fi
}

run_test() {
	cmd="${1}"
	vecho "-- Running: ${tname} --"
	"${cmd}"
	sts=$?
	if [[ ${sts} -ne 0 ]]; then
		vecho "failed ${cmd} [${sts}]"
		FAILURES+=("${cmd}")
		if [[ "${exitfirst}" = "yes" ]]; then
			exit 1
		fi
	fi
}

summary() {
	if [[ ${#FAILURES[@]} -gt 0 ]]; then
		echo "ERROR: failing tests:"
		for i in "${!FAILURES[@]}"; do
			echo "  ${FAILURES[i]}"
		done
		exit 1
	else
		echo "all tests passed"
		exit 0
	fi
}

show_help() {
	echo "$0 [options]"
	echo "  Options:"
	echo "    -c|--coverage TYPE  Run tests with given coverage type"
	echo "    -v|--verbose        Print verbose output"
	echo "    -x|--exitfirst      Exit on first test failure"
	echo "    -h|--help           Display help"
	echo ""
	echo "  Coverage Types:"
	echo "    html -    Generate html files (one per package) in the"
	echo "              coverage directory."
	echo "    stdout -  Print coverage information to the console."
	echo "    summary - Generate ONLY a packagecover.out to record"
	echo "              coverage stats for all tests run.*"
	echo "    * All modes generate the package cover information,"
	echo "      summary mode disables all additional output."
}

CLI="$(getopt -o c:xvh --long coverage:,exitfirst,verbose,help -n "$0" -- "$@")"
eval set -- "${CLI}"
while true ; do
	case "$1" in
		-c|--coverage)
			coverage="$2"
			case ${coverage} in
				stdout|html|summary);;
				*)
					echo "error: invalid coverage type ${coverage}."
					echo "       need one of: stdout, html, summary"
					exit 2
				;;
			esac
			shift
			shift
		;;
		-x|--exitfirst)
			exitfirst=yes
			shift
		;;
		-v|--verbose)
			verbose=yes
			shift
		;;
		-h|--help)
			show_help
			exit 0
		;;
		--)
			shift
			break
		;;
		*)
			echo "unknown option" >&2
			exit 2
		;;
	esac
done

trap summary EXIT

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

# environment vars exported for test scripts
# (this way test scripts dont need cli parsing, we do it here)
export HEKETI_TEST_EXITFIRST=${exitfirst}
export HEKETI_TEST_SCRIPT_DIR="${SCRIPT_DIR}"
export HEKETI_TEST_COVERAGE=${coverage}

cd "${SCRIPT_DIR}" || exit 1
for tname in $(ls tests | sort) ; do
	tpath="./tests/${tname}"
	if [[ ${tpath} =~ .*\.sh$ && -f ${tpath} && -x ${tpath} ]]; then
		run_test "${tpath}"
	fi
done
