#!/bin/bash
#
# This script will run all test scripts that are in test/extended.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

test_scripts=`find test/extended -maxdepth 1 -name "*.sh" -not  \( -name "all.sh" \)`

OVERALL_RETURN=0
for test_script in $test_scripts; do
	STARTTIME=$(date +%s)
	echo "${STARTTIME} starting ${test_script}";

	set +e
	# use a subshell to prevent `exit` calls from killing this script
	(${test_script})
	CURR_RETURN=$?
	set -e

	if [ "${CURR_RETURN}" -ne "0" ]; then
		OVERALL_RETURN=${CURR_RETURN}
	fi
	ENDTIME=$(date +%s); echo "${test_script} took $(($ENDTIME - $STARTTIME)) seconds and returned with ${CURR_RETURN}";
done

exit ${OVERALL_RETURN}
