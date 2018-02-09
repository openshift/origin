#!/usr/bin/env bash
# Copyright 2017 The Kubernetes Authors.
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

result=0

function cleanup() {
    popd
    rm -r contrib/examples/consumer/vendor

    if [[ "${result:-}" != "0" ]]; then
        echo "A downstream consumer of our client library cannot use dep to vendor Service Catalog. You may need to add a constraint to Gopkg.toml to address."
        exit ${result}
    fi
}

pushd contrib/examples/consumer
trap "cleanup" EXIT

dep ensure
go build .

echo "Verified that our Gopkg.toml is sufficient for a downstream consumer of our client library."
