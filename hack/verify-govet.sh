#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::golang::verify_go_version

mkdir -p _output/govet

os::build::setup_env

govet_blacklist=(
	"pkg/auth/ldaputil/client.go:[0-9]+: assignment copies lock value to c: crypto/tls.Config contains sync.Once contains sync.Mutex"
	"pkg/.*/client/clientset_generated/internalclientset/fake/clientset_generated.go:[0-9]+: literal copies lock value from fakePtr: github.com/openshift/origin/vendor/k8s.io/kubernetes/pkg/client/testing/core.Fake"
	"pkg/.*/client/clientset_generated/release_1_3/fake/clientset_generated.go:30: literal copies lock value from fakePtr: github.com/openshift/origin/vendor/k8s.io/kubernetes/pkg/client/testing/core.Fake"
)

function govet_blacklist_contains() {
	local text=$1
	for blacklist_entry in "${govet_blacklist[@]}"; do
		if grep -Eqx "${blacklist_entry}" <<<"${text}"; then
			# the text we got matches this blacklist entry
			return 0
		fi
	done
	return 1
}

test_dirs="$(find_files | cut -d '/' -f 1-2 | sort -u)"
for test_dir in ${test_dirs}; do
	if ! result="$(go tool vet -shadow=false "${test_dir}" 2>&1)"; then
		while read -r line; do
			if ! govet_blacklist_contains "${line}"; then
				echo "${line}"
				FAILURE=true
			fi
		done <<<"${result}"
	fi
done

# For the sake of slowly white-listing `shadow` checks, we need to keep track of which
# directories we're searching through. The following are all of the directories we care about:
# all top-level directories except for 'pkg', and all second-level subdirectories of 'pkg'.
ALL_DIRS=$(find_files | grep -Eo "\./([^/]+|pkg/[^/]+)" | sort -u)

DIR_BLACKLIST='./hack
./pkg/api
./pkg/authorization
./pkg/build
./pkg/client
./pkg/cmd
./pkg/deploy
./pkg/diagnostics
./pkg/dockerregistry
./pkg/generate
./pkg/gitserver
./pkg/image
./pkg/oauth
./pkg/project
./pkg/quota
./pkg/router
./pkg/sdn
./pkg/security
./pkg/serviceaccounts
./pkg/template
./pkg/user
./pkg/util
./test
./third_party
./tools'

for test_dir in $ALL_DIRS
do
  # use `grep` failure to determine that a directory is not in the blacklist
  if ! echo "${DIR_BLACKLIST}" | grep -q "${test_dir}"; then
    go tool vet -shadow -shadowstrict $test_dir
    if [ "$?" -ne "0" ]
    then
      FAILURE=true
    fi
  fi
done

# We don't want to exit on the first failure of go vet, so just keep track of
# whether a failure occurred or not.
if [[ -n "${FAILURE:-}" ]]; then
	echo "FAILURE: go vet failed!"
	exit 1
else
	echo "SUCCESS: go vet succeded!"
	exit 0
fi
