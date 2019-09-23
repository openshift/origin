#!/bin/bash

# Check for shell syntax & style.

test_syntax() {
	bash -n "${1}"
}

test_shellcheck() {
	if [[ "${SHELLCHECK}" ]]; then
		# only look for the "flag" on comment lines
		# without this we can match our own code :-)
		if grep -q '^#.*HEKETI-SKIP-SHELLCHECK' "${1}"; then
			return 0
		fi
		shellcheck -x -e SC2181,SC2029,SC1091,SC1090,SC2012 "${1}"
	else
		return 0
	fi
}

SHELLCHECK="$(which shellcheck 2>/dev/null)"

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

BASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -z "${SHELLCHECK}" ]]; then
	echo "warning: could not find shellcheck ... will skip checks" >&2
fi

cd "${BASE_DIR}" || exit 2
SCRIPTS=$(find . \( -path ./vendor -o -path ./.git \) -prune \
	-o -name '*.sh' -print | sort)

failed=0
for script in ${SCRIPTS}; do
	err=0
	test_syntax "${script}"
	[[ $? -ne 0 ]] && err=1
	test_shellcheck "${script}"
	[[ $? -ne 0 ]] && err=1
	((failed+=err))
	if [[ ${err} -ne 0 && ${HEKETI_TEST_EXITFIRST} = "yes" ]]; then
		echo "detected issues in ${script}" >&2
		exit ${failed}
	elif [[ ${err} -ne 0 ]]; then
		echo "detected issues in ${script}" >&2
	fi
done
exit ${failed}
