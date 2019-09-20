#!/bin/bash

MODE=set
PROFILE_OUT="${PWD}/profile.out"
ACC_OUT="${PWD}/coverage.txt"

testCover() {
	# set the return value to 0 (successful)
	retval=0
	# get the directory to check from the parameter. Default to '.'
	d=${1:-.}
	# skip if there are no Go files here
	ls $d/*.go &> /dev/null || return $retval
	# switch to the directory to check
	pushd $d > /dev/null
	# create the coverage profile
	coverageresult=$(go test $TAGS -coverprofile="${PROFILE_OUT}" -covermode=${MODE})
	# output the result so we can check the shell output
	echo ${coverageresult}
	# append the results to acc.out if coverage didn't fail, else set the retval to 1 (failed)
	( [[ ${coverageresult} == *FAIL* ]] && retval=1 ) || ( [ -f "${PROFILE_OUT}" ] && grep -v "mode: ${MODE}" "${PROFILE_OUT}" >> "${ACC_OUT}" )
	# return to our working dir
	popd > /dev/null
	# return our return value
	return $retval
}

# Init coverage.txt
echo "mode: ${MODE}" > $ACC_OUT

# Run test coverage on all directories containing go files except testlapack testblas and testgraph.
find . -type d -not -path '*testlapack*' -and -not -path '*testblas*' -and -not -path '*testgraph*' | while read d; do testCover $d || exit; done

# Upload the coverage profile to coveralls.io
[ -n "$COVERALLS_TOKEN" ] && ( goveralls -coverprofile=$ACC_OUT || echo -e '\n\e[31mCoveralls failed.\n' )

# Upload to coverage profile to codedov.io
[ -n "$CODECOV_TOKEN" ] && ( bash <(curl -s https://codecov.io/bash) || echo -e '\n\e[31mCodecov failed.\n' )
