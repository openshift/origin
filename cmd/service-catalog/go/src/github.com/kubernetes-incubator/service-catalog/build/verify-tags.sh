#!/bin/bash

# Copyright 2014 The Kubernetes Authors.
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

# This script makes sure each versioned API in service catalog
#has as json tag

set -o errexit
set -o nounset
set -o pipefail

result=0

find_files() {
  find . -not \( \
      \( \
        -wholename './output' \
        -o -wholename './_gopath' \
        -o -wholename './release' \
        -o -wholename './target' \
        -o -wholename '*/vendor/*' \
      \) -prune \
    \) \
    \( -wholename '*pkg/api/v*/types.go' \
       -o -wholename '*pkg/apis/*/v*/types.go' \
       -o -wholename '*pkg/api/unversioned/types.go' \
    \)
}

if [[ $# -eq 0 ]]; then
  versioned_api_files=$(find_files | egrep "pkg/(.[^/]*)+/((v.[^/]*)|unversioned)/types\.go")
else
  versioned_api_files="${*}"
fi

for file in $versioned_api_files; do
  if cat -n "${file}" | sed -n '/genclient/,$p' | grep -v "^\s*[0-9]*\s$" | egrep -v "(\/\/|{|}|\(|\)|type|=)" | grep -v json; then 
    echo "Versioned APIs should contain json tags for fields in file ${file}"
    result=1
  fi
done
exit ${result}
