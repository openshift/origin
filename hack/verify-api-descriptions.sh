#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

cd "${OS_ROOT}"

DEF_MISSING=false
UPDATE_WHITELIST=false

function verify-api-descriptions-for-spec ()
{
	SPEC=$1
	echo "Verifying Descriptions for Spec: ${SPEC}"
	OBJECTS=$(python hack/list-swagger-objects.py ${SPEC})
	for object in $OBJECTS
	do
		desc_location="api/definitions/${object}"
		if [ -d "${desc_location}" ]
		then
			if [ ! -s "${desc_location}/description.adoc" ]
			then
				if ! grep -qx "^${object}$" hack/api-description-whitelist.txt
				then
					echo "Description missing for: ${object}"
					DEF_MISSING=true
				fi
			else
				if grep -qx "^${object}$" hack/api-description-whitelist.txt
				then
					echo "Unnecessary whitelist entry for: ${object}"
					UPDATE_WHITELIST=true
				fi
			fi
		else
			if ! grep -q "${object}" hack/api-description-whitelist.txt
			then
				echo "Description missing for: ${object}"
				DEF_MISSING=true
			fi
		fi
	done
}

SPECS="${OS_ROOT}/api/swagger-spec/*.json"
for spec in $SPECS
do
	verify-api-descriptions-for-spec $spec
done

if $DEF_MISSING || $UPDATE_WHITELIST
then
	if $DEF_MISSING
	then
		echo "FAILURE: Add missing descriptions to api/definitions"
	else
		echo "FAILURE: Prune unnecessary whitelist entries"
	fi
	exit 1
else
	echo SUCCESS
fi