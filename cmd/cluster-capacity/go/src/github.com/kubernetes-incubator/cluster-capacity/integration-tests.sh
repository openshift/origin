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

#! /bin/sh

# Assumptions:
# - cluster provisioned

KUBE_CONFIG=${KUBE_CONFIG:-~/.kube/config}

KUBECTL="kubectl --kubeconfig=${KUBE_CONFIG}"
CC="./cluster-capacity --kubeconfig ${KUBE_CONFIG}"
#### pre-tests checks

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

printError() {
  echo -e "${RED}$1${NC}"
}

printSuccess() {
  echo -e "${GREEN}$1${NC}"
}

echo "####PRE-TEST CHECKS"
# check the cluster is available
$KUBECTL version
if [ "$?" -ne 0 ]; then
  printError "Unable to connect to kubernetes cluster"
  exit 1
fi

# check the cluster-capacity namespace exists
$KUBECTL get ns cluster-capacity
if [ "$?" -ne 0 ]; then
  $KUBECTL create -f examples/namespace.yml
  if [ "$?" -ne 0 ]; then
    printError "Unable to create cluster-capacity namespace"
    exit 1
  fi
fi

echo ""
echo ""

#### TESTS
echo ""
echo ""
echo ""
echo ""
echo ""
echo "####RUNNING TESTS"
echo ""
echo "# Running simple estimation of examples/pod.yaml"
$CC --podspec=examples/pod.yaml --verbose| tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "# Running simple estimation of examples/pod.yaml in verbose mode"
$CC --podspec=examples/pod.yaml --verbose | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "# Decrease resource in the cluster by running new pods"
$KUBECTL create -f examples/rc.yml
if [ "$?" -ne 0 ]; then
  printError "Unable to create additional resources"
  exit 1
fi

while [ $($KUBECTL get pods | grep nginx | grep "Running" | wc -l) -ne 3 ]; do
  echo "waiting for pods to come up"
  sleep 1s
done

echo ""
echo "# Running simple estimation of examples/pod.yaml in verbose mode with less resources"
$CC --podspec=examples/pod.yaml --verbose | tee estimation.log
if [ -z "$(cat estimation.log | grep 'Termination reason')" ]; then
  printError "Missing termination reason"
  exit 1
fi

echo ""
echo "# Delete resource in the cluster by deleting rc"
$KUBECTL delete -f examples/rc.yml

printSuccess "#### All tests passed"

#### BOILERPLATE
echo ""
echo ""
echo ""
echo ""
echo ""
echo "####RUNNING BOILERPLATE"
./verify/verify-boilerplate.sh

