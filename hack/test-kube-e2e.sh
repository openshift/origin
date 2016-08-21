#!/bin/bash

# DISCLAIMER: This script is intended only to simplify running the kube
# e2e tests against an openshift cluster.  No guarantees are made as
# to whether the tests will run successfully without modification.

# This script runs the kubernetes e2e tests against a deployed
# openshift cluster.  The path to a local kube repo must be supplied by
# the KUBE_ROOT environment variable.  All arguments to this script
# will be supplied to the underlying test runner.  Documentation for
# the test runner is available in the kube repo:
#
# https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/devel/development.md
#
# By default, cluster configuration will be read from
# [repopath]/openshift.local.config.  The environment variable
# OS_CONF_ROOT can be used to set the parent of openshift.local.config
# if it is not found at the repo root.
#
# Example usage:
#
# KUBE_ROOT=../kubernetes hack/test-kube-e2e.sh --ginkgo.focus="Network.*intra"
#
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

KUBE_ROOT=${KUBE_ROOT:-""}

if [ -z "${KUBE_ROOT}" ]; then
  >&2 echo "KUBE_ROOT must be set to run e2e tests"
  exit 1
fi

CONF_ROOT="${OS_CONF_ROOT:-${OS_ROOT}}"
CONF_PATH="${CONF_ROOT}/openshift.local.config"
KUBECONFIG="${CONF_PATH}/master/admin.kubeconfig"

if [[ ! -f "${KUBECONFIG}" ]]; then
  >&2 echo "${KUBECONFIG} not found.  Maybe override OS_CONF_ROOT?"
  exit 1
fi

# Configuring conformance mode skips test setup which allows the kube
# tests to target an openshift cluster.
declare -x KUBERNETES_CONFORMANCE_TEST="y"
declare -x KUBECONFIG
declare -x NUM_MINIONS=$(ls -d ${CONF_PATH}/node-* | wc -w)
declare -x KUBE_MASTER_IP=$(grep 'server' ${KUBECONFIG} | \
  sed 's|^[ \t]*server: https://||')
pushd ${KUBE_ROOT}
hack/ginkgo-e2e.sh $@
popd
