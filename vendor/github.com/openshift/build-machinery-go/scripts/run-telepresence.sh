#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script executes telepresence against an operator deployment.
#
#   KUBECONFIG=... TP_DEPLOYMENT_YAML=... TP_CMD_PATH=... run-telepresence.sh
#
# The script is parameterized by env vars prefixed with 'TP_'.
#
# Dependencies:
#
# - oc
# - jq (fedora: dnf install jq; macos: brew install jq)
# - telepresence (https://www.telepresence.io/reference/install)
#   - >= 0.105 is compatible with OCP
# - delve (go get github.com/go-delve/delve/cmd/dlv)

KUBECONFIG="${KUBECONFIG:-}"
if [[ "${KUBECONFIG}" == "" ]]; then
  >&2 echo "KUBECONFIG is not set"
  exit 1
fi

# The yaml defining the operator's deployment resource. Will be parsed
# for configuration like resource name, namespace, and command.
TP_DEPLOYMENT_YAML="${TP_DEPLOYMENT_YAML:-}"
if [[ ! "${TP_DEPLOYMENT_YAML}" ]]; then
  >&2 echo "TP_DEPLOYMENT_YAML is not set"
  exit 1
fi

# The path to the golang package to run or debug
# (e.g. `/my/project/cmd/operator`)
TP_CMD_PATH="${TP_CMD_PATH:-}"
if [[ ! "${TP_CMD_PATH}" ]]; then
  >&2 echo "TP_CMD_PATH is not set"
  exit 1
fi

# By default the operator will be run via 'go run'. Providing TP_DEBUG=y
# will run `dlv debug` instead.
TP_DEBUG="${TP_DEBUG:-}"

# Whether to add an entry in /etc/hosts for the api server host. This
# is necessary if targeting a cluster deployed in aws.
TP_ADD_AWS_HOST_ENTRY="${TP_ADD_AWS_HOST_ENTRY:-y}"

# The arguments for the command that will be executed will be parsed
# from the deployment by default, but can be overridden by setting
# this var. The name of the command itself is implicitly determined by
# `go run` or `dlv debug` from TP_CMD_PATH.
TP_CMD_ARGS="${TP_CMD_ARGS:-}"

# Will add a -v=<value> argument to the command to set the logging
# verbosity. If a verbosity argument was already supplied, this value
# will override it.
TP_VERBOSITY="${TP_VERBOSITY:-}"

# The command telepresence should run in the local deployment
# environment. By default it will run or debug the operator but it can
# be useful to run a shell (e.g. `bash`) for troubleshooting.
TP_RUN_CMD="${TP_RUN_CMD:-}"
if [[ ! "${TP_RUN_CMD}" ]]; then
  # Default to a recursive call
  TP_RUN_CMD="${0}"
  if [[ ! -x "${TP_RUN_CMD}" ]]; then
    # Prefix the recursive call with bash if the script is not
    # executable as will be the case when build-machinery-go is
    # vendored.
    TP_RUN_CMD="bash ${TP_RUN_CMD}"
  fi
fi

# Whether this script should run telepresence or is being run by
# telepresence. Used as an internal control var, not necessary to set
# manually.
_TP_INTERNAL_RUN="${_TP_INTERNAL_RUN:-}"

# Some operators (e.g. auth operator) specify build flags to generate
# different binaries for ocp or okd.
TP_BUILD_FLAGS="${TP_BUILD_FLAGS:-}"

# Simplify querying the deployment yaml with jq
function jq_deployment () {
  local jq_arg="${1}"
  cat "${TP_DEPLOYMENT_YAML}"\
    | python -c 'import json, sys, yaml ; y=yaml.safe_load(sys.stdin.read()) ; print(json.dumps(y))'\
    | jq -r "${jq_arg}"
}

NAMESPACE="$(jq_deployment '.metadata.namespace')"
NAME="$(jq_deployment '.metadata.name')"

# If not provided, the lock configmap will be defaulted to the name of
# the deployment suffixed by `-lock`.
TP_LOCK_CONFIGMAP="${TP_LOCK_CONFIGMAP:-${NAME}-lock}"

if [ "${_TP_INTERNAL_RUN}" ]; then
  # Delete the leader election lock to ensure that the local process
  # becomes the leader as quickly as possible.
  oc delete configmap "${TP_LOCK_CONFIGMAP}" --namespace "${NAMESPACE}" --ignore-not-found=true

  if [[ ! "${TP_CMD_ARGS}" ]]; then
    # Parse the arguments from the deployment
    TP_CMD_ARGS="$(jq_deployment '.spec.template.spec.containers[0].command[1:] | join(" ")' )"
    TP_CMD_ARGS+=" $(jq_deployment '.spec.template.spec.containers[0].args | join(" ")' )"
  fi

  if [[ "${TP_VERBOSITY}" ]]; then
    # Setting log level last ensures that any existing -v argument will be overridden.
    TP_CMD_ARGS+=" -v=${TP_VERBOSITY}"
  fi

  pushd "${TP_CMD_PATH}" > /dev/null
    if [[ "${TP_DEBUG}" ]]; then
      if [[ "${TP_BUILD_FLAGS}" ]]; then
        TP_BUILD_FLAGS="--build-flags=${TP_BUILD_FLAGS}"
      fi
      dlv debug ${TP_BUILD_FLAGS} -- ${TP_CMD_ARGS}
    else
      go run ${TP_BUILD_FLAGS} . ${TP_CMD_ARGS}
    fi
  popd > /dev/null
else
  if [[ "${TP_ADD_AWS_HOST_ENTRY}" ]]; then
    # Add an entry in /etc/hosts for the api endpoint in the configured
    # KUBECONFIG (which is assumed to have at most one server).
    #
    # This supports using telepresence with kube clusters running in
    # aws. telepresence will proxy dns requests to the cluster and will
    # return an internal aws address for the api endpoint otherwise.
    SERVER_HOST="$(grep server "${KUBECONFIG}" | sed -e 's+    server: https://\(.*\):.*+\1+')"
    SERVER_IP="$(dig "${SERVER_HOST}" +short | head -n 1)"
    ENTRY="${SERVER_IP} ${SERVER_HOST}"
    if ! grep "${ENTRY}" /etc/hosts > /dev/null; then
      >&2 echo "Attempting to add '${ENTRY}' to /etc/hosts to ensure access to an aws cluster. This requires sudo."
      >&2 echo "If this cluster is not in aws, specify TP_ADD_AWS_HOST_ENTRY="
      grep -q "${SERVER_HOST}" /etc/hosts && \
        (cp -f /etc/hosts /tmp/etc-hosts && \
           sed -i 's+.*'"${SERVER_HOST}"'$+'"${ENTRY}"'+' /tmp/etc-hosts && \
           sudo cp /tmp/etc-hosts /etc/hosts)\
          || echo "${ENTRY}" | sudo tee -a /etc/hosts > /dev/null
    fi
  fi

  # Ensure pod volumes are symlinked to the expected location
  if [[ ! -L '/var/run/configmaps' ]]; then
     >&2 echo "Attempting to symlink /tmp/tel_root/var/run/configmaps to /var/run/configmaps. This requires sudo."
     sudo ln -s /tmp/tel_root/var/run/configmaps /var/run/configmaps
  fi
  if [[ ! -L '/var/run/secrets' ]]; then
    >&2 echo "Attempting to symlink /tmp/tel_root/var/run/secrets to /var/run/secrets. This requires sudo."
    sudo ln -s /tmp/tel_root/var/run/secrets /var/run/secrets
  fi

  KIND="$(jq_deployment '.kind')"
  GROUP="$(jq_deployment '.apiVersion')"

  # Ensure the operator is not managed by CVO
  oc patch clusterversion/version --type='merge' -p "$(cat <<- EOF
spec:
  overrides:
  - group: ${GROUP}
    kind: ${KIND}
    name: ${NAME}
    namespace: ${NAMESPACE}
    unmanaged: true
EOF
)"

  # Ensure the operator is managed again on shutdown
  function cleanup {
    oc patch clusterversion/version --type='merge' -p "$(cat <<- EOF
spec:
  overrides:
  - group: ${GROUP}
    kind: ${KIND}
    name: ${NAME}
    namespace: ${NAMESPACE}
    unmanaged: false
EOF
)"
  }
  trap cleanup EXIT

  # Ensure that traffic for all machines in the cluster is also
  # proxied so that the local operator will be able to access them.
  ALSO_PROXY="$(oc get machines -A -o json | jq -jr '.items[] | .status.addresses[0].address | @text "--also-proxy=\(.) "')"

  TELEPRESENCE_USE_OCP_IMAGE=NO _TP_INTERNAL_RUN=y telepresence --namespace="${NAMESPACE}"\
    --swap-deployment "${NAME}" ${ALSO_PROXY} --mount=/tmp/tel_root --run ${TP_RUN_CMD}
fi
