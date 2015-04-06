#!/bin/bash

# based on http://stackoverflow.com/questions/21126011/is-it-possible-to-post-coverage-for-multiple-packages-to-coveralls
# with script found at https://github.com/gopns/gopns/blob/master/test-coverage.sh

echo "mode: set" > acc.out
returnval=`go test -v -coverprofile=profile.out`
echo ${returnval}
if [[ ${returnval} != *FAIL* ]]
then
	if [ -f profile.out ]
	then
		cat profile.out | grep -v "mode: set" >> acc.out 
	fi
else
	exit 1
fi	

for Dir in $(find ./* -maxdepth 10 -type d ); 
do
	if ls $Dir/*.go &> /dev/null;
	then
		echo $Dir
		returnval=`go test -v -coverprofile=profile.out $Dir`
		echo ${returnval}
		if [[ ${returnval} != *FAIL* ]]
		then
    		if [ -f profile.out ]
    		then
        		cat profile.out | grep -v "mode: set" >> acc.out 
    		fi
    	else
    		exit 1
    	fi	
    fi
done
if [ -n "$COVERALLS_TOKEN" ]
then
	$HOME/gopath/bin/goveralls -coverprofile=acc.out -service=travis-ci -repotoken $COVERALLS_TOKEN
fi	

rm -rf ./profile.out
rm -rf ./acc.out
