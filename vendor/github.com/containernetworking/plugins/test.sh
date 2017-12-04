#!/usr/bin/env bash
#
# Run CNI plugin tests.
# 
# This needs sudo, as we'll be creating net interfaces.
#
set -e

source ./build.sh

echo "Running tests"

TESTABLE="plugins/ipam/dhcp plugins/ipam/host-local plugins/ipam/host-local/backend/allocator plugins/main/loopback plugins/main/ipvlan plugins/main/macvlan plugins/main/bridge plugins/main/ptp plugins/meta/flannel plugins/main/vlan plugins/sample pkg/ip pkg/ipam pkg/ns pkg/utils pkg/utils/hwaddr pkg/utils/sysctl plugins/meta/portmap"

# user has not provided PKG override
if [ -z "$PKG" ]; then
	TEST=$TESTABLE
	FMT=$TESTABLE

# user has provided PKG override
else
	TEST=$PKG

	# only run gofmt on packages provided by user
	FMT="$TEST"
fi

# split TEST into an array and prepend REPO_PATH to each local package
split=(${TEST// / })
TEST=${split[@]/#/${REPO_PATH}/}

sudo -E bash -c "umask 0; PATH=${GOROOT}/bin:$(pwd)/bin:${PATH} go test ${TEST}"

echo "Checking gofmt..."
fmtRes=$(gofmt -l $FMT)
if [ -n "${fmtRes}" ]; then
	echo -e "gofmt checking failed:\n${fmtRes}"
	exit 255
fi

echo "Checking govet..."
vetRes=$(go vet $TEST)
if [ -n "${vetRes}" ]; then
	echo -e "govet checking failed:\n${vetRes}"
	exit 255
fi
