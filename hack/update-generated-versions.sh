#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

verify="${VERIFY:-}"

os::build::version::kubernetes_vars
if [[ "${KUBE_GIT_VERSION}" =~ ([0-9]+\.[0-9]+\.[0-9]+) ]]; then
	version_kubernetes=${BASH_REMATCH[1]}
else
  os::log::fatal "Unable to find kubernetes version from ${KUBE_GIT_VERSION}"
fi

for i in ${OS_ROOT}/images/hyperkube/Dockerfile*; do
  if ! grep -q "io.openshift.build.versions=" "${i}"; then
    os::log::fatal "$i does not contain an io.openshift.build.versions tag"
  fi
  if [[ -n "${verify}" ]]; then
    diff "${i}" <(sed -Ee "s|(io.openshift.build.versions=).*|\\1\"kubernetes=${version_kubernetes}\"|g" "${i}")
  else
    sed -i'' -Ee "s|(io.openshift.build.versions=).*|\\1\"kubernetes=${version_kubernetes}\"|g" "${i}"
  fi
done
