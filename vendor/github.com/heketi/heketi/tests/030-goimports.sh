#!/bin/bash

GOIMPORTSCHECK="$(command -v goimports 2>/dev/null)"

if [[ -z "${GOIMPORTSCHECK}" ]]; then
	echo "warning: could not find goimports ... will skip checks" >&2
	exit 0
fi

ERROR=0
for pkg in $(go list ./... | grep -v '/vendor/'); do
	dir="$GOPATH/src/$pkg"
	FILES=$(goimports -l -e "$dir"/*.go)
	if [ ${#FILES} -ne 0 ]; then
		for file in $FILES; do
			echo "$file"
		done
		ERROR=1
	fi
done

if [ $ERROR -ne 0 ]; then
	exit 1
fi
