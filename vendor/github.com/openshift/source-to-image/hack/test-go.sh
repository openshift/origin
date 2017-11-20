#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${S2I_ROOT}"

s2i::build::setup_env

find_test_dirs() {
  cd "${S2I_ROOT}"
  find . -not \( \
      \( \
        -wholename './Godeps' \
        -o -wholename './release' \
        -o -wholename './target' \
        -o -wholename './vendor' \
        -o -wholename '*/_output/*' \
        -o -wholename './.git' \
      \) -prune \
    \) -name '*_test.go' -print0 | xargs -0n1 dirname | sort -u | xargs -n1 printf "${S2I_GO_PACKAGE}/%s\n"
}

S2I_RACE=${S2I_RACE--race}
S2I_COVER=${S2I_COVER--cover}
S2I_TIMEOUT=${S2I_TIMEOUT--timeout 60s}

if [ "${1-}" != "" ]; then
  test_packages="$S2I_GO_PACKAGE/$1"
else
  test_packages=`find_test_dirs`
fi

OUTPUT_COVERAGE=${OUTPUT_COVERAGE-}

if [[ -n "${OUTPUT_COVERAGE}" ]]; then
  # Iterate over packages to run coverage
  test_packages=( $test_packages )
  for test_package in "${test_packages[@]}"
  do
    mkdir -p "$OUTPUT_COVERAGE/$test_package"
    PROFILEPATH=${OUTPUT_COVERAGE}/${test_package}/profile.out
    if [[ ${OSTYPE} == "cygwin" ]]; then
      PROFILEPATH=$(cygpath -w ${PROFILEPATH})
    fi
    S2I_COVER_PROFILE="-coverprofile=${PROFILEPATH}"

    go test $S2I_RACE $S2I_TIMEOUT $S2I_COVER "$S2I_COVER_PROFILE" "$test_package" "${@:2}"
  done

  echo 'mode: atomic' > ${OUTPUT_COVERAGE}/profiles.out
  find $OUTPUT_COVERAGE -name profile.out | xargs sed '/^mode: atomic$/d' >> ${OUTPUT_COVERAGE}/profiles.out
  PROFILES_OUT=${OUTPUT_COVERAGE}/profiles.out
  COVERAGE_HTML=${OUTPUT_COVERAGE}/coverage.html
  if [[ ${OSTYPE} == "cygwin" ]]; then
    PROFILES_OUT=$(cygpath -w ${PROFILES_OUT})
    COVERAGE_HTML=$(cygpath -w ${COVERAGE_HTML})
  fi

  go tool cover -html=${PROFILES_OUT} -o ${COVERAGE_HTML}

  # remove ${OUTPUT_COVERAGE}/github.com
  rm -rf $OUTPUT_COVERAGE/${S2I_GO_PACKAGE%%/*}
else
  go test $S2I_RACE $S2I_TIMEOUT $S2I_COVER "${@:2}" $test_packages
fi
