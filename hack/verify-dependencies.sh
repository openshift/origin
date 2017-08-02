#!/bin/bash
# this verify script is establishing the logical dependencies we have in our source tree
# and will allow us to ratchet down those dependencies as we fix them up.



set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::build::setup_env

function print_forbidden_imports () {
    set -o errexit # this was unset by ||
    local PACKAGE="$1"
    shift
    local RE=""
    local SEP=""
    for CLAUSE in "$@"; do
        RE+="${SEP}${CLAUSE}"
        SEP='\|'
    done
    local FORBIDDEN=$(
        go list -f $'{{with $package := .ImportPath}}{{range $.Imports}}{{$package}} imports {{.}}\n{{end}}{{end}}' ./${PACKAGE}/... |
        sed 's|^github.com/openshift/origin/vendor/||;s| github.com/openshift/origin/vendor/| |' |
        grep -v " github.com/openshift/origin/${PACKAGE}" |
        grep " github.com/openshift/origin/" |
        sed 's|github.com/openshift/origin/||g' |
        grep -v -e "imports \(${RE}\)"
    )
    if [ -n "${FORBIDDEN}" ]; then
        echo "${PACKAGE} has a forbidden dependency:"
        echo
        echo "${FORBIDDEN}" | sed 's/^/  /'
        echo
        return 1
    fi
    local TEST_FORBIDDEN=$(
        go list -f $'{{with $package := .ImportPath}}{{range $.TestImports}}{{$package}} imports {{.}}\n{{end}}{{end}}' ./${PACKAGE}/... |
        sed 's|^github.com/openshift/origin/vendor/||;s| github.com/openshift/origin/vendor/| |' |
        grep -v " github.com/openshift/origin/${PACKAGE}" |
        grep " github.com/openshift/origin/" |
        sed 's|github.com/openshift/origin/||g' |
        grep -v -e "imports \(${RE}\)"
    )
    if [ -n "${TEST_FORBIDDEN}" ]; then
        echo "${PACKAGE} has a forbidden dependency in test code:"
        echo
        echo "${TEST_FORBIDDEN}" | sed 's/^/  /'
        echo
        return 1
    fi
    return 0
}

RC=0
print_forbidden_imports pkg/image pkg/client || RC=1
if [ ${RC} != 0 ]; then
    exit ${RC}
fi

# TODO enable this as we start creating our staging repos
# if grep -rq '// import "github.com/openshift/origin/' 'staging/'; then
# 	echo 'file has "// import "github.com/openshift/origin/"'
# 	exit 1
# fi


exit 0
