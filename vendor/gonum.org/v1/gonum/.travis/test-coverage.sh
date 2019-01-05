#!/bin/bash

PROFILE_OUT=$PWD/profile.out
ACC_OUT=$PWD/acc.out

testCover() {
	# set the return value to 0 (succesful)
	retval=0
	# get the directory to check from the parameter. Default to '.'
	d=${1:-.}
	# skip if there are no Go files here
	ls $d/*.go &> /dev/null || return $retval
	# switch to the directory to check
	pushd $d > /dev/null
	# create the coverage profile
	coverageresult=`go test -v $TAGS -coverprofile=$PROFILE_OUT`
	# output the result so we can check the shell output
	echo ${coverageresult}
	# append the results to acc.out if coverage didn't fail, else set the retval to 1 (failed)
	( [[ ${coverageresult} == *FAIL* ]] && retval=1 ) || ( [ -f $PROFILE_OUT ] && grep -v "mode: set" $PROFILE_OUT >> $ACC_OUT )
	# return to our working dir
	popd > /dev/null
	# return our return value
	return $retval
}

# Init acc.out
echo "mode: set" > $ACC_OUT

# Run test coverage on all directories containing go files except testlapack and testblas.
find . -type d -not -path '*testlapack*' -and -not -path '*testblas*' | while read d; do testCover $d || exit; done

# Upload the coverage profile to coveralls.io
[ -n "$COVERALLS_TOKEN" ] && goveralls -coverprofile=$ACC_OUT -service=travis-ci -repotoken $COVERALLS_TOKEN
