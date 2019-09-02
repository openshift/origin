#!/usr/bin/env bash
set -euo pipefail

# switch into the repo root directory
cd "$(dirname $0)"

echo -n "Running tests "
function testrun {
    bash -c "umask 0; PATH=$PATH go test $@"
}
if [ ! -z "${COVERALLS:-""}" ]; then
    # coverage profile only works per-package
    PKGS="$(go list ./... | xargs echo)"
    echo "with coverage profile generation..."
    i=0
    for t in ${PKGS}; do
        testrun "-covermode set -coverprofile ${i}.coverprofile ${t}"
        i=$((i+1))
    done
    gover
    goveralls -service=travis-ci -coverprofile=gover.coverprofile
else
    echo "without coverage profile generation..."
    testrun "./..."
fi

echo "Checking gofmt..."
fmtRes=$(go fmt ./...)
if [ -n "${fmtRes}" ]; then
	echo -e "go fmt checking failed:\n${fmtRes}"
	exit 255
fi

echo "Checking govet..."
vetRes=$(go vet ./...)
if [ -n "${vetRes}" ]; then
	echo -e "go vet checking failed:\n${vetRes}"
	exit 255
fi

echo "Checking license header..."
licRes=$(
       for file in $(find . -type f -iname '*.go'); do
               head -n1 "${file}" | grep -Eq "(Copyright|generated)" || echo -e "  ${file}"
       done
)
if [ -n "${licRes}" ]; then
       echo -e "license header checking failed:\n${licRes}"
       exit 255
fi

echo "Success"
