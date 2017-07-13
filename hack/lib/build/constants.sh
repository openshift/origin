#!/bin/bash

# This script provides constants for the Golang binary build process
readonly -A OS_BUILD_GOLANG_VERSION_ARCH_MAP=(
  ['1.8']='amd64 arm64 ppc64le'
)

readonly -A OS_BUILD_SYSARCH_TO_GOARCH_MAP=(
  ['x86_64']='amd64'
  ['aarch64']='arm64'
  ['ppc64le']='ppc64le'
  ['s390x']='s390x'
)

readonly -A OS_BUILD_GOARCH_TO_SYSARCH_MAP=(
  ['amd64']='x86_64'
  ['arm64']='aarch64'
  ['ppc64le']='ppc64le'
  ['s390x']='s390x'
)

readonly OS_BUILD_ENV_GOLANG_VERSIONS=("${!OS_BUILD_GOLANG_VERSION_ARCH_MAP[@]}")

# TODO: clean these up
readonly OS_BUILD_ENV_PLATFORMS=(
  linux/amd64
  linux/arm64
  linux/ppc64le
)

readonly OS_BUILD_ENV_ARCHES=("${OS_BUILD_ENV_PLATFORMS[@]##*/}")

readonly OS_BUILD_ENV_PLATFORMS_REDISTRIBUTABLE_ONLY=(
  darwin/amd64
  windows/amd64
)
readonly OS_BUILD_ENV_PLATFORMS_REDISTRIBUTABLE=("${OS_BUILD_ENV_PLATFORMS[@]}" "${OS_BUILD_ENV_PLATFORMS_REDISTRIBUTABLE_ONLY[@]}")
# end TODO cleanup

readonly OS_OUTPUT_BASEPATH="${OS_OUTPUT_BASEPATH:-_output}"
readonly OS_BASE_OUTPUT="${OS_ROOT}/${OS_OUTPUT_BASEPATH}"
readonly OS_OUTPUT_SCRIPTPATH="${OS_BASE_OUTPUT}/scripts"

readonly OS_OUTPUT_SUBPATH="${OS_OUTPUT_SUBPATH:-${OS_OUTPUT_BASEPATH}/local}"
readonly OS_OUTPUT="${OS_ROOT}/${OS_OUTPUT_SUBPATH}"
readonly OS_OUTPUT_RELEASEPATH="${OS_OUTPUT}/releases"
readonly OS_OUTPUT_RPMPATH="${OS_OUTPUT_RELEASEPATH}/rpms"
readonly OS_OUTPUT_BINPATH="${OS_OUTPUT}/bin"
readonly OS_OUTPUT_PKGDIR="${OS_OUTPUT}/pkgdir"

readonly OS_GO_PACKAGE=github.com/openshift/origin

readonly OS_SDN_COMPILE_TARGETS_LINUX=(
  pkg/sdn/plugin/sdn-cni-plugin
  vendor/github.com/containernetworking/cni/plugins/ipam/host-local
  vendor/github.com/containernetworking/cni/plugins/main/loopback
)
readonly OS_IMAGE_COMPILE_TARGETS_LINUX=(
  images/pod
  cmd/dockerregistry
  cmd/gitserver
  vendor/k8s.io/kubernetes/cmd/hyperkube
  "${OS_SDN_COMPILE_TARGETS_LINUX[@]}"
)
readonly OS_SCRATCH_IMAGE_COMPILE_TARGETS_LINUX=(
  examples/hello-openshift
)
readonly OS_IMAGE_COMPILE_BINARIES=("${OS_SCRATCH_IMAGE_COMPILE_TARGETS_LINUX[@]##*/}" "${OS_IMAGE_COMPILE_TARGETS_LINUX[@]##*/}")

readonly OS_REDISTRIBUTABLE_TARGETS=(
  cmd/openshift
  cmd/oc
  cmd/kubefed
)
readonly OS_REDISTRIBUTABLE_BINARIES=("${OS_REDISTRIBUTABLE_TARGETS[@]##*/}")

readonly OS_TEST_TARGETS=(
  test/extended/extended.test
)

#If you update this list, be sure to get the images/origin/Dockerfile
readonly OPENSHIFT_BINARY_SYMLINKS=(
  openshift-router
  openshift-deploy
  openshift-recycle
  openshift-sti-build
  openshift-docker-build
  origin
  osc
  oadm
  osadm
  kubectl
  kubernetes
  kubelet
  kube-proxy
  kube-apiserver
  kube-controller-manager
  kube-scheduler
)
readonly OPENSHIFT_BINARY_COPY=(
  oadm
  kubelet
  kube-proxy
  kube-apiserver
  kube-controller-manager
  kube-scheduler
)
readonly OC_BINARY_COPY=(
  kubectl
)
readonly OS_BINARY_RELEASE_CLIENT_WINDOWS=(
  oc.exe
  README.md
  ./LICENSE
)
readonly OS_BINARY_RELEASE_CLIENT_MAC=(
  oc
  README.md
  ./LICENSE
)
readonly OS_BINARY_RELEASE_CLIENT_LINUX=(
  ./oc
  ./README.md
  ./LICENSE
)
readonly OS_BINARY_RELEASE_SERVER_LINUX=(
  './*'
)
readonly OS_BINARY_RELEASE_CLIENT_EXTRA=(
  ${OS_ROOT}/README.md
  ${OS_ROOT}/LICENSE
)
