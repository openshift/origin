#!/usr/bin/env bash
# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

REPOINFRA_ROOT=$(git rev-parse --show-toplevel)
TMP_GOPATH=$(mktemp -d)

GOBIN="${TMP_GOPATH}/bin" go get github.com/kubernetes/repo-infra/kazel

"${REPOINFRA_ROOT}/verify/go_install_from_commit.sh" \
  github.com/bazelbuild/bazel-gazelle/cmd/gazelle \
  0.8 \
  "${TMP_GOPATH}"

touch "${REPOINFRA_ROOT}/vendor/BUILD"

gazelle_diff=$("${TMP_GOPATH}/bin/gazelle" fix \
  -build_file_name=BUILD,BUILD.bazel \
  -external=vendored \
  -mode=diff \
  -repo_root="${REPOINFRA_ROOT}")

kazel_diff=$("${TMP_GOPATH}/bin/kazel" \
  -dry-run \
  -print-diff \
  -root="${REPOINFRA_ROOT}")

# check if there are vendor/*_test.go
# previously we used godeps which did this, but `dep` does not handle this
# properly yet. some of these tests don't build well. see:
# ref: https://github.com/kubernetes/test-infra/pull/5411
vendor_tests=$(find ${REPOINFRA_ROOT}/vendor/ -name "*_test.go" | wc -l)

if [[ -n "${gazelle_diff}" || -n "${kazel_diff}" || "${vendor_tests}" -ne "0" ]]; then
  echo "${gazelle_diff}"
  echo "${kazel_diff}"
  echo "number of vendor/*_test.go: ${vendor_tests} (want: 0)"
  echo
  echo "Run ./verify/update-bazel.sh"
  exit 1
fi
