#!/bin/bash

runselftest() {
	exec &> selftest.log

	SELF_TEST_EXIT=0 ../run.sh --test TestSelfTest
	if [[ $? -ne 0 ]]; then
		exit 1
	fi

	SELF_TEST_EXIT=1 ../run.sh --test TestSelfTest
	if [[ $? -ne 1 ]]; then
		exit 1
	fi

	../run.sh --test TestMagicGoose
	if [[ $? -ne 1 ]]; then
		exit 1
	fi

	rm -rf "$PWD/faketests"
	mkdir -p "$PWD/faketests/TestOne"
	mkdir -p "$PWD/faketests/TestTwo"
	mkdir -p "$PWD/faketests/TestThree"
	# generate run scripts
	echo -e '#!/bin/sh\nexit 0' > "$PWD/faketests/TestOne/run.sh"
	echo -e '#!/bin/sh\nexit 1' > "$PWD/faketests/TestTwo/run.sh"
	echo -e '#!/bin/sh\nexit 0' > "$PWD/faketests/TestThree/run.sh"
	# generate teardown scripts
	echo -e '#!/bin/sh\nexit 0' > "$PWD/faketests/TestOne/teardown.sh"
	echo -e '#!/bin/sh\nexit 0' > "$PWD/faketests/TestTwo/teardown.sh"
	echo -e '#!/bin/sh\nexit 0' > "$PWD/faketests/TestThree/teardown.sh"
	chmod +x "$PWD/faketests/TestOne/run.sh" \
		"$PWD/faketests/TestTwo/run.sh" \
		"$PWD/faketests/TestThree/run.sh" \
		"$PWD/faketests/TestOne/teardown.sh" \
		"$PWD/faketests/TestTwo/teardown.sh" \
		"$PWD/faketests/TestThree/teardown.sh"

	../run.sh -d "$PWD/faketests" --test TestOne --test TestTwo --test TestThree
	if [[ $? -ne 1 ]]; then
		exit 1
	fi

	# make test two a passing test
	echo -e '#!/bin/sh\nexit 0' > "$PWD/faketests/TestTwo/run.sh"
	../run.sh -d "$PWD/faketests" --test TestOne --test TestTwo --test TestThree
	if [[ $? -ne 0 ]]; then
		exit 1
	fi
}

main() {
	if [[ "$SELF_TEST_EXIT" ]]; then
		echo "Going to exit with code $SELF_TEST_EXIT"
		exit "$SELF_TEST_EXIT"
	fi
	runselftest
}

main
