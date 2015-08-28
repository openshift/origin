#!/bin/bash
#
# This scripts starts the OpenShift server where
# the OpenShift Docker registry and router are installed,
# and then the forcepull tests are launched.
# We intentionally do not run the force pull tests in parallel
# given the tagging based image corruption that occurs - do not
# want 2 tests corrupting an image differently at the same time.

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
cd "${OS_ROOT}"

source ${OS_ROOT}/hack/util.sh
source ${OS_ROOT}/hack/common.sh

exec env FOCUS="-focus=forcepull:" \
  /bin/bash -c ${OS_ROOT}/hack/test-extended/default/run.sh
