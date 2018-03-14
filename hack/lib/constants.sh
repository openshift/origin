#!/bin/bash

# This script provides constants for the Golang binary build process

readonly OS_GO_PACKAGE=github.com/openshift/origin

readonly OS_BUILD_ENV_GOLANG="${OS_BUILD_ENV_GOLANG:-1.9}"
readonly OS_BUILD_ENV_IMAGE="${OS_BUILD_ENV_IMAGE:-openshift/origin-release:golang-${OS_BUILD_ENV_GOLANG}}"
readonly OS_REQUIRED_GO_VERSION="go${OS_BUILD_ENV_GOLANG}"
readonly OS_GLIDE_MINOR_VERSION="13"
readonly OS_REQUIRED_GLIDE_VERSION="0.$OS_GLIDE_MINOR_VERSION"

readonly OS_GOFLAGS_TAGS="include_gcs include_oss containers_image_openpgp"
readonly OS_GOFLAGS_TAGS_LINUX_AMD64="gssapi"
readonly OS_GOFLAGS_TAGS_LINUX_S390X="gssapi"
readonly OS_GOFLAGS_TAGS_LINUX_ARM64="gssapi"
readonly OS_GOFLAGS_TAGS_LINUX_PPC64LE="gssapi"

readonly OS_OUTPUT_BASEPATH="${OS_OUTPUT_BASEPATH:-_output}"
readonly OS_BASE_OUTPUT="${OS_ROOT}/${OS_OUTPUT_BASEPATH}"
readonly OS_OUTPUT_SCRIPTPATH="${OS_OUTPUT_SCRIPTPATH:-"${OS_BASE_OUTPUT}/scripts"}"

readonly OS_OUTPUT_SUBPATH="${OS_OUTPUT_SUBPATH:-${OS_OUTPUT_BASEPATH}/local}"
readonly OS_OUTPUT="${OS_ROOT}/${OS_OUTPUT_SUBPATH}"
readonly OS_OUTPUT_RELEASEPATH="${OS_OUTPUT}/releases"
readonly OS_OUTPUT_RPMPATH="${OS_OUTPUT_RELEASEPATH}/rpms"
readonly OS_OUTPUT_BINPATH="${OS_OUTPUT}/bin"
readonly OS_OUTPUT_PKGDIR="${OS_OUTPUT}/pkgdir"

readonly OS_SDN_COMPILE_TARGETS_LINUX=(
  pkg/network/sdn-cni-plugin
  vendor/github.com/containernetworking/plugins/plugins/ipam/host-local
  vendor/github.com/containernetworking/plugins/plugins/main/loopback
)
readonly OS_IMAGE_COMPILE_TARGETS_LINUX=(
  "${OS_SDN_COMPILE_TARGETS_LINUX[@]}"
)
readonly OS_SCRATCH_IMAGE_COMPILE_TARGETS_LINUX=(
  images/pod
  examples/hello-openshift
)
readonly OS_IMAGE_COMPILE_BINARIES=("${OS_SCRATCH_IMAGE_COMPILE_TARGETS_LINUX[@]##*/}" "${OS_IMAGE_COMPILE_TARGETS_LINUX[@]##*/}")

readonly OS_CROSS_COMPILE_TARGETS=(
  cmd/hypershift
  cmd/openshift
  cmd/oc
  cmd/oadm
  cmd/template-service-broker
  vendor/k8s.io/kubernetes/cmd/hyperkube
)
readonly OS_CROSS_COMPILE_BINARIES=("${OS_CROSS_COMPILE_TARGETS[@]##*/}")

readonly OS_TEST_TARGETS=(
  test/extended/extended.test
)

readonly OS_GOVET_BLACKLIST=(
	"pkg/.*/generated/internalclientset/fake/clientset_generated.go:[0-9]+: literal copies lock value from fakePtr: github.com/openshift/origin/vendor/k8s.io/client-go/testing.Fake"
	"pkg/.*/generated/clientset/fake/clientset_generated.go:[0-9]+: literal copies lock value from fakePtr: github.com/openshift/origin/vendor/k8s.io/client-go/testing.Fake"
	"pkg/build/vendor/github.com/docker/docker/client/hijack.go:[0-9]+: assignment copies lock value to c: crypto/tls.Config contains sync.Once contains sync.Mutex"
	"cmd/cluster-capacity/.*"
	"pkg/build/builder/vendor/.*"
	"pkg/cmd/server/start/.*"
)

#If you update this list, be sure to get the images/origin/Dockerfile
readonly OPENSHIFT_BINARY_SYMLINKS=(
  openshift-router
  openshift-deploy
  openshift-recycle
  openshift-sti-build
  openshift-docker-build
  openshift-git-clone
  openshift-manage-dockerfile
  openshift-extract-image-content
  origin
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

# os::build::get_product_vars exports variables that we expect to change
# depending on the distribution of Origin
function os::build::get_product_vars() {
  export OS_BUILD_LDFLAGS_IMAGE_PREFIX="${OS_IMAGE_PREFIX:-"openshift/origin"}"
  export OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS="${OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS:-"centos7"}"
  export OS_BUILD_LDFLAGS_FEDERATION_SERVER_IMAGE_NAME="${OS_BUILD_LDFLAGS_FEDERATION_SERVER_IMAGE_NAME:-"${OS_BUILD_LDFLAGS_IMAGE_PREFIX}-federation"}"
  export OS_BUILD_LDFLAGS_FEDERATION_ETCD_IMAGE="${OS_BUILD_LDFLAGS_FEDERATION_ETCD_IMAGE:-"quay.io/coreos/etcd:v3.1.7"}"
}

# os::build::ldflags calculates the -ldflags argument for building OpenShift
function os::build::ldflags() {
  # Run this in a subshell to prevent settings/variables from leaking.
  set -o errexit
  set -o nounset
  set -o pipefail

  cd "${OS_ROOT}"

  os::build::version::get_vars
  os::build::get_product_vars

  local buildDate="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

  declare -a ldflags=()

  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/oc/bootstrap/docker.defaultImageStreams" "${OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/cmd/util/variable.DefaultImagePrefix" "${OS_BUILD_LDFLAGS_IMAGE_PREFIX}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.majorFromGit" "${OS_GIT_MAJOR}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.minorFromGit" "${OS_GIT_MINOR}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.versionFromGit" "${OS_GIT_VERSION}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.commitFromGit" "${OS_GIT_COMMIT}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/version.buildDate" "${buildDate}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/kubernetes/pkg/version.gitCommit" "${KUBE_GIT_COMMIT}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/kubernetes/pkg/version.gitVersion" "${KUBE_GIT_VERSION}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/kubernetes/pkg/version.buildDate" "${buildDate}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/kubernetes/pkg/version.gitTreeState" "clean"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/client-go/pkg/version.gitCommit" "${KUBE_GIT_COMMIT}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/client-go/pkg/version.gitVersion" "${KUBE_GIT_VERSION}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/client-go/pkg/version.buildDate" "${buildDate}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/client-go/pkg/version.gitTreeState" "clean")
)

  # The -ldflags parameter takes a single string, so join the output.
  echo "${ldflags[*]-}"
}
readonly -f os::build::ldflags

# os::util::list_go_src_files lists files we consider part of our project
# source code, useful for tools that iterate over source to provide vet-
# ting or linting, etc.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::list_go_src_files() {
	find . -not \( \
		\( \
		-wholename './_output' \
		-o -wholename './.*' \
		-o -wholename './pkg/assets/bindata.go' \
		-o -wholename './pkg/assets/*/bindata.go' \
		-o -wholename './pkg/oc/bootstrap/bindata.go' \
		-o -wholename './openshift.local.*' \
		-o -wholename './test/extended/testdata/bindata.go' \
		-o -wholename '*/vendor/*' \
		-o -wholename './cmd/service-catalog/*' \
		-o -wholename './cmd/cluster-capacity/*' \
		-o -wholename './assets/bower_components/*' \
		\) -prune \
	\) -name '*.go' | sort -u
}
readonly -f os::util::list_go_src_files

# os::util::list_go_src_dirs lists dirs in origin/ and cmd/ dirs excluding
# cmd/cluster-capacity and cmd/service-catalog and doc.go useful for tools that
# iterate over source to provide vetting or linting, or for godep-save etc.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::list_go_src_dirs() {
	os::util::list_go_src_files | cut -d '/' -f 1-2 | grep -v ".go$" | grep -v "^./cmd" | LC_ALL=C sort -u
	os::util::list_go_src_files | grep "^./cmd/"| cut -d '/' -f 1-3 | grep -v ".go$" | LC_ALL=C sort -u
}
readonly -f os::util::list_go_src_dirs

# os::util::list_go_deps outputs the list of dependencies for the project.
function os::util::list_go_deps() {
  go list -f '{{.ImportPath}}{{.Imports}}' ./pkg/... ./cmd/... | tr '[]' '  ' |
    grep -vE '^github.com/openshift/origin/cmd/(service-catalog|cluster-capacity)' |
    sed -e 's|github.com/openshift/origin/vendor/||g' |
    sed -e 's|github.com/openshift/origin/pkg/build/vendor/||g'
}

# os::util::list_test_packages_under lists all packages containing Golang test files that we
# want to run as unit tests under the given base dir in the source tree
function os::util::list_test_packages_under() {
    local basedir=$*

    # we do not quote ${basedir} to allow for multiple arguments to be passed in as well as to allow for
    # arguments that use expansion, e.g. paths containing brace expansion or wildcards
    # we do not quote ${basedir} to allow for multiple arguments to be passed in as well as to allow for
    # arguments that use expansion, e.g. paths containing brace expansion or wildcards
    find ${basedir} -not \(                   \
        \(                                    \
              -path 'vendor'                  \
              -o -path '*_output'             \
              -o -path '*.git'                \
              -o -path '*openshift.local.*'   \
              -o -path '*vendor/*'            \
              -o -path '*assets/node_modules' \
              -o -path '*test/*'              \
              -o -path '*cmd/cluster-capacity' \
              -o -path '*cmd/service-catalog' \
              -o -path '*pkg/proxy' \
        \) -prune                             \
    \) -name '*_test.go' | xargs -n1 dirname | sort -u | xargs -n1 printf "${OS_GO_PACKAGE}/%s\n"

    local kubernetes_path="vendor/k8s.io/kubernetes"

    if [[ -n "${TEST_KUBE-}" ]]; then
      # we need to find all of the kubernetes test suites, excluding those we directly whitelisted before, the end-to-end suite, and
      # the go2idl tests which we currently do not support
      # etcd3 isn't supported yet and that test flakes upstream
      # cmd wasn't done before using glide and constantly flakes
      find -L vendor/k8s.io/{apimachinery,apiserver,client-go,kube-aggregator,kubernetes} -not \( \
        \(                                                                                          \
          -path "${kubernetes_path}/staging"                                                        \
          -o -path "${kubernetes_path}/cmd"                                                         \
          -o -path "${kubernetes_path}/test"                                                        \
          -o -path "${kubernetes_path}/cmd/libs/go2idl/client-gen/testoutput/testgroup/unversioned" \
          -o -path "${kubernetes_path}/pkg/storage/etcd3"                                           \
          -o -path "${kubernetes_path}/third_party/golang/go/build"                                 \
        \) -prune                                                                                   \
      \) -name '*_test.go' | cut -f 2- -d / | xargs -n1 dirname | sort -u | xargs -n1 printf "${OS_GO_PACKAGE}/vendor/%s\n"
    else
      echo "${OS_GO_PACKAGE}/vendor/k8s.io/kubernetes/pkg/api/..."
      echo "${OS_GO_PACKAGE}/vendor/k8s.io/kubernetes/pkg/apis/..."
    fi
}
readonly -f os::util::list_test_packages_under

# Generates the .syso file used to add compile-time VERSIONINFO metadata to the
# Windows binary.
function os::build::generate_windows_versioninfo() {
  os::build::version::get_vars
  local major="${OS_GIT_MAJOR}"
  local minor="${OS_GIT_MINOR%+}"
  local patch="${OS_GIT_PATCH}"
  local windows_versioninfo_file=`mktemp --suffix=".versioninfo.json"`
  cat <<EOF >"${windows_versioninfo_file}"
{
       "FixedFileInfo":
       {
               "FileVersion": {
                       "Major": ${major},
                       "Minor": ${minor},
                       "Patch": ${patch}
               },
               "ProductVersion": {
                       "Major": ${major},
                       "Minor": ${minor},
                       "Patch": ${patch}
               },
               "FileFlagsMask": "3f",
               "FileFlags ": "00",
               "FileOS": "040004",
               "FileType": "01",
               "FileSubType": "00"
       },
       "StringFileInfo":
       {
               "Comments": "",
               "CompanyName": "Red Hat, Inc.",
               "InternalName": "openshift client",
               "FileVersion": "${OS_GIT_VERSION}",
               "InternalName": "oc",
               "LegalCopyright": "© Red Hat, Inc. Licensed under the Apache License, Version 2.0",
               "LegalTrademarks": "",
               "OriginalFilename": "oc.exe",
               "PrivateBuild": "",
               "ProductName": "OpenShift Client",
               "ProductVersion": "${OS_GIT_VERSION}",
               "SpecialBuild": ""
       },
       "VarFileInfo":
       {
               "Translation": {
                       "LangID": "0409",
                       "CharsetID": "04B0"
               }
       }
}
EOF
  goversioninfo -o ${OS_ROOT}/cmd/oc/oc.syso ${windows_versioninfo_file}
}
readonly -f os::build::generate_windows_versioninfo

# Removes the .syso file used to add compile-time VERSIONINFO metadata to the
# Windows binary.
function os::build::clean_windows_versioninfo() {
  rm ${OS_ROOT}/cmd/oc/oc.syso
}
readonly -f os::build::clean_windows_versioninfo

# OS_ALL_IMAGES is the list of images built by os::build::images.
readonly OS_ALL_IMAGES=(
  origin
  origin-base
  origin-pod
  origin-deployer
  origin-docker-builder
  origin-keepalived-ipfailover
  origin-sti-builder
  origin-haproxy-router
  origin-f5-router
  origin-egress-router
  origin-egress-http-proxy
  origin-egress-dns-proxy
  origin-recycler
  origin-cluster-capacity
  origin-service-catalog
  origin-template-service-broker
  hello-openshift
  openvswitch
  node
)

# os::build::images builds all images in this repo.
function os::build::images() {
  # Create link to file if the FS supports hardlinks, otherwise copy the file
  function ln_or_cp {
    local src_file=$1
    local dst_dir=$2
    if os::build::archive::internal::is_hardlink_supported "${dst_dir}" ; then
      ln -f "${src_file}" "${dst_dir}"
    else
      cp -pf "${src_file}" "${dst_dir}"
    fi
  }

  # Link or copy image binaries to the appropriate locations.
  ln_or_cp "${OS_OUTPUT_BINPATH}/linux/amd64/hello-openshift" examples/hello-openshift/bin

  # determine the correct tag prefix
  tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

  # images that depend on "${tag_prefix}-source"
  ( os::build::image "${tag_prefix}-pod"                   images/pod ) &
  ( os::build::image "${tag_prefix}-cluster-capacity"      images/cluster-capacity ) &
  ( os::build::image "${tag_prefix}-service-catalog"       images/service-catalog ) &
  ( os::build::image "${tag_prefix}-template-service-broker"  images/template-service-broker ) &


  for i in `jobs -p`; do wait $i; done

  # images that depend on "${tag_prefix}-base"
  ( os::build::image "${tag_prefix}"                       images/origin ) &
  ( os::build::image "${tag_prefix}-egress-router"         images/egress/router ) &
  ( os::build::image "${tag_prefix}-egress-http-proxy"     images/egress/http-proxy ) &
  ( os::build::image "${tag_prefix}-egress-dns-proxy"      images/egress/dns-proxy ) &
  ( os::build::image "${tag_prefix}-federation"            images/federation ) &

  for i in `jobs -p`; do wait $i; done

  # images that depend on "${tag_prefix}
  ( os::build::image "${tag_prefix}-haproxy-router"        images/router/haproxy ) &
  ( os::build::image "${tag_prefix}-keepalived-ipfailover" images/ipfailover/keepalived ) &
  ( os::build::image "${tag_prefix}-deployer"              images/deployer ) &
  ( os::build::image "${tag_prefix}-recycler"              images/recycler ) &
  ( os::build::image "${tag_prefix}-docker-builder"        images/builder/docker/docker-builder ) &
  ( os::build::image "${tag_prefix}-sti-builder"           images/builder/docker/sti-builder ) &
  ( os::build::image "${tag_prefix}-f5-router"             images/router/f5 ) &
  ( os::build::image "openshift/node"                      images/node ) &

  for i in `jobs -p`; do wait $i; done

  # images that depend on "openshift/node"
  ( os::build::image "openshift/openvswitch"               images/openvswitch ) &

  # extra images (not part of infrastructure)
  ( os::build::image "openshift/hello-openshift"           examples/hello-openshift ) &

  for i in `jobs -p`; do wait $i; done
}
readonly -f os::build::images
