#!/bin/bash

# This library holds utility functions for building
# and placing Golang binaries for multiple arches.

# os::build::binaries_from_targets take a list of build targets and return the
# full go package to be built
function os::build::binaries_from_targets() {
  local target
  for target; do
    echo "${OS_GO_PACKAGE}/${target}"
  done
}
readonly -f os::build::binaries_from_targets

# Asks golang what it thinks the host platform is.  The go tool chain does some
# slightly different things when the target platform matches the host platform.
function os::build::host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}
readonly -f os::build::host_platform

# Create a user friendly version of host_platform for end users
function os::build::host_platform_friendly() {
  local platform=${1:-}
  if [[ -z "${platform}" ]]; then
    platform=$(os::build::host_platform)
  fi
  if [[ $platform == "windows/amd64" ]]; then
    echo "windows"
  elif [[ $platform == "darwin/amd64" ]]; then
    echo "mac"
  elif [[ $platform == "linux/386" ]]; then
    echo "linux-32bit"
  elif [[ $platform == "linux/amd64" ]]; then
    echo "linux-64bit"
  elif [[ $platform == "linux/ppc64le" ]]; then
    echo "linux-powerpc64"
  elif [[ $platform == "linux/arm64" ]]; then
    echo "linux-arm64"
  elif [[ $platform == "linux/s390x" ]]; then
    echo "linux-s390"
  else
    echo "$(go env GOHOSTOS)-$(go env GOHOSTARCH)"
  fi
}
readonly -f os::build::host_platform_friendly

# This converts from platform/arch to PLATFORM_ARCH, host platform will be
# considered if no parameter passed
function os::build::platform_arch() {
  local platform=${1:-}
  if [[ -z "${platform}" ]]; then
    platform=$(os::build::host_platform)
  fi

  echo $(echo ${platform} | tr '[:lower:]/' '[:upper:]_')
}
readonly -f os::build::platform_arch

# os::build::setup_env will check that the `go` commands is available in
# ${PATH}. If not running on Travis, it will also check that the Go version is
# good enough for the Kubernetes build.
#
# Output Vars:
#   export GOPATH - A modified GOPATH to our created tree along with extra
#     stuff.
#   export GOBIN - This is actively unset if already set as we want binaries
#     placed in a predictable place.
function os::build::setup_env() {
  os::util::ensure::system_binary_exists 'go'

  if [[ -z "$(which sha256sum)" ]]; then
    sha256sum() {
      return 0
    }
  fi

  # Travis continuous build uses a head go release that doesn't report
  # a version number, so we skip this check on Travis.  It's unnecessary
  # there anyway.
  if [[ "${TRAVIS:-}" != "true" ]]; then
    local go_version
    go_version=($(go version))
    if [[ "${go_version[2]}" < "go1.8" ]]; then
      os::log::fatal "Detected Go version: ${go_version[*]}.
Origin builds require Go version 1.8 or greater."
    fi
  fi
  # For any tools that expect this to be set (it is default in golang 1.6),
  # force vendor experiment.
  export GO15VENDOREXPERIMENT=1

  unset GOBIN

  # default to OS_OUTPUT_GOPATH if no GOPATH set
  if [[ -z "${GOPATH:-}" ]]; then
    export OS_OUTPUT_GOPATH=1
  fi

  # use the regular gopath for building
  if [[ -z "${OS_OUTPUT_GOPATH:-}" ]]; then
    export OS_TARGET_BIN=${GOPATH}/bin
    return
  fi

  # create a local GOPATH in _output
  GOPATH="${OS_OUTPUT}/go"
  OS_TARGET_BIN=${GOPATH}/bin
  local go_pkg_dir="${GOPATH}/src/${OS_GO_PACKAGE}"
  local go_pkg_basedir=$(dirname "${go_pkg_dir}")

  mkdir -p "${go_pkg_basedir}"
  rm -f "${go_pkg_dir}"

  # TODO: This symlink should be relative.
  ln -s "${OS_ROOT}" "${go_pkg_dir}"

  # lots of tools "just don't work" unless we're in the GOPATH
  cd "${go_pkg_dir}"

  # Append OS_EXTRA_GOPATH to the GOPATH if it is defined.
  if [[ -n ${OS_EXTRA_GOPATH:-} ]]; then
    GOPATH="${GOPATH}:${OS_EXTRA_GOPATH}"
    # TODO: needs to handle multiple directories
    OS_TARGET_BIN=${OS_EXTRA_GOPATH}/bin
  fi
  export GOPATH
  export OS_TARGET_BIN
}
readonly -f os::build::setup_env

# Build static binary targets.
#
# Input:
#   $@ - targets and go flags.  If no targets are set then all binaries targets
#     are built.
#   OS_BUILD_PLATFORMS - Incoming variable of targets to build for.  If unset
#     then just the host architecture is built.
function os::build::build_static_binaries() {
  CGO_ENABLED=0 os::build::build_binaries -installsuffix=cgo "$@"
}
readonly -f os::build::build_static_binaries

# Build binary targets specified
#
# Input:
#   $@ - targets and go flags.  If no targets are set then all binaries targets
#     are built.
#   OS_BUILD_PLATFORMS - Incoming variable of targets to build for.  If unset
#     then just the host architecture is built.
function os::build::build_binaries() {
  if [[ $# -eq 0 ]]; then
    return
  fi
  local -a binaries=( "$@" )
  # Create a sub-shell so that we don't pollute the outer environment
  ( os::build::internal::build_binaries "${binaries[@]+"${binaries[@]}"}" )
}

# Build binary targets specified. Should always be run in a sub-shell so we don't leak GOBIN
#
# Input:
#   $@ - targets and go flags.  If no targets are set then all binaries targets
#     are built.
#   OS_BUILD_PLATFORMS - Incoming variable of targets to build for.  If unset
#     then just the host architecture is built.
os::build::internal::build_binaries() {
    # Check for `go` binary and set ${GOPATH}.
    os::build::setup_env

    # Fetch the version.
    local version_ldflags
    version_ldflags=$(os::build::ldflags)

    local goflags
    # Use eval to preserve embedded quoted strings.
    eval "goflags=(${OS_GOFLAGS:-})"

    local arg
    for arg; do
      if [[ "${arg}" == -* ]]; then
        # Assume arguments starting with a dash are flags to pass to go.
        goflags+=("${arg}")
      fi
    done

    os::build::export_targets "$@"

    local -a nonstatics=()
    local -a tests=()
    for binary in "${binaries[@]}"; do
      if [[ "${binary}" =~ ".test"$ ]]; then
        tests+=($binary)
      else
        nonstatics+=($binary)
      fi
    done

    local host_platform=$(os::build::host_platform)
    local platform
    for platform in "${platforms[@]+"${platforms[@]}"}"; do
      echo "++ Building go targets for ${platform}:" "${targets[@]}"
      mkdir -p "${OS_OUTPUT_BINPATH}/${platform}"

      # output directly to the desired location
      if [[ $platform == $host_platform ]]; then
        export GOBIN="${OS_OUTPUT_BINPATH}/${platform}"
      else
        unset GOBIN
      fi

      local platform_gotags_envvar=OS_GOFLAGS_TAGS_$(os::build::platform_arch ${platform})
      local platform_gotags_test_envvar=OS_GOFLAGS_TAGS_TEST_$(os::build::platform_arch ${platform})

      # work around https://github.com/golang/go/issues/11887
      local local_ldflags="${version_ldflags}"
      if [[ "${platform}" == "darwin/amd64" ]]; then
        local_ldflags+=" -s"
      fi

      #Add Windows File Properties/Version Info and Icon Resource for oc.exe
      if [[ "$platform" == "windows/amd64" ]]; then
        os::build::generate_windows_versioninfo
      fi

      if [[ ${#nonstatics[@]} -gt 0 ]]; then
        GOOS=${platform%/*} GOARCH=${platform##*/} go install \
          -pkgdir "${OS_OUTPUT_PKGDIR}/${platform}" \
          -tags "${OS_GOFLAGS_TAGS-} ${!platform_gotags_envvar:-}" \
          -ldflags="${local_ldflags}" \
          "${goflags[@]:+${goflags[@]}}" \
          "${nonstatics[@]}"

        # GOBIN is not supported on cross-compile in Go 1.5+ - move to the correct target
        if [[ $platform != $host_platform ]]; then
          local platform_src="/${platform//\//_}"
          mv "${OS_TARGET_BIN}/${platform_src}/"* "${OS_OUTPUT_BINPATH}/${platform}/"
        fi
      fi

      if [[ "$platform" == "windows/amd64" ]]; then
        rm ${OS_ROOT}/cmd/oc/oc.syso
      fi

      for test in "${tests[@]:+${tests[@]}}"; do
        local outfile="${OS_OUTPUT_BINPATH}/${platform}/$(basename ${test})"
        # disabling cgo allows use of delve
        CGO_ENABLED="${OS_TEST_CGO_ENABLED:-}" GOOS=${platform%/*} GOARCH=${platform##*/} go test \
          -pkgdir "${OS_OUTPUT_PKGDIR}/${platform}" \
          -tags "${OS_GOFLAGS_TAGS-} ${!platform_gotags_test_envvar:-}" \
          -ldflags "${local_ldflags}" \
          -i -c -o "${outfile}" \
          "${goflags[@]:+${goflags[@]}}" \
          "$(dirname ${test})"
      done
    done
}
readonly -f os::build::build_binaries

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

 # Generates the set of target packages, binaries, and platforms to build for.
# Accepts binaries via $@, and platforms via OS_BUILD_PLATFORMS, or defaults to
# the current platform.
function os::build::export_targets() {
  targets=()
  local arg
  for arg; do
    if [[ "${arg}" != -* ]]; then
      targets+=("${arg}")
    fi
  done

  if [[ ${#targets[@]} -eq 0 ]]; then
    echo "No targets to export!"
    exit 1
  fi

  binaries=($(os::build::binaries_from_targets "${targets[@]}"))

  platforms=("${OS_BUILD_PLATFORMS[@]:+${OS_BUILD_PLATFORMS[@]}}")
}
readonly -f os::build::export_targets

# This will take $@ from $GOPATH/bin and copy them to the appropriate
# place in ${OS_OUTPUT_BINDIR}
#
# If OS_RELEASE_ARCHIVE is set, tar archives prefixed with OS_RELEASE_ARCHIVE for
# each of OS_BUILD_PLATFORMS are created.
#
# Ideally this wouldn't be necessary and we could just set GOBIN to
# OS_OUTPUT_BINDIR but that won't work in the face of cross compilation.  'go
# install' will place binaries that match the host platform directly in $GOBIN
# while placing cross compiled binaries into `platform_arch` subdirs.  This
# complicates pretty much everything else we do around packaging and such.
function os::build::place_bins() {
  (
    local host_platform
    host_platform=$(os::build::host_platform)

    if [[ "${OS_RELEASE_ARCHIVE-}" != "" ]]; then
      os::build::version::get_vars
      mkdir -p "${OS_OUTPUT_RELEASEPATH}"
    fi

    os::build::export_targets "$@"
    for platform in "${platforms[@]+"${platforms[@]}"}"; do
      # The substitution on platform_src below will replace all slashes with
      # underscores.  It'll transform darwin/amd64 -> darwin_amd64.
      local platform_src="/${platform//\//_}"

      # Skip this directory if the platform has no binaries.
      if [[ ! -d "${OS_OUTPUT_BINPATH}/${platform}" ]]; then
        continue
      fi

      # Create an array of binaries to release. Append .exe variants if the platform is windows.
      local -a binaries=()
      for binary in "${targets[@]}"; do
        binary=$(basename $binary)
        if [[ $platform == "windows/amd64" ]]; then
          binaries+=("${binary}.exe")
        else
          binaries+=("${binary}")
        fi
      done

      # If no release archive was requested, we're done.
      if [[ "${OS_RELEASE_ARCHIVE-}" == "" ]]; then
        continue
      fi

      # Create a temporary bin directory containing only the binaries marked for release.
      local release_binpath=$(mktemp -d openshift.release.${OS_RELEASE_ARCHIVE}.XXX)
      for binary in "${binaries[@]}"; do
        cp "${OS_OUTPUT_BINPATH}/${platform}/${binary}" "${release_binpath}/"
      done

      # Create binary copies where specified.
      local suffix=""
      if [[ $platform == "windows/amd64" ]]; then
        suffix=".exe"
      fi
      for linkname in "${OPENSHIFT_BINARY_COPY[@]}"; do
        local src="${release_binpath}/openshift${suffix}"
        if [[ -f "${src}" ]]; then
          ln "${release_binpath}/openshift${suffix}" "${release_binpath}/${linkname}${suffix}"
        fi
      done
      for linkname in "${OC_BINARY_COPY[@]}"; do
        local src="${release_binpath}/oc${suffix}"
        if [[ -f "${src}" ]]; then
          ln "${release_binpath}/oc${suffix}" "${release_binpath}/${linkname}${suffix}"
        fi
      done

      # Create the release archive.
      platform="$( os::build::host_platform_friendly "${platform}" )"
      if [[ ${OS_RELEASE_ARCHIVE} == "openshift-origin" ]]; then
        for file in "${OS_BINARY_RELEASE_CLIENT_EXTRA[@]}"; do
          cp "${file}" "${release_binpath}/"
        done
        if [[ $platform == "windows" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive::zip "${OS_BINARY_RELEASE_CLIENT_WINDOWS[@]}"
        elif [[ $platform == "mac" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive::zip "${OS_BINARY_RELEASE_CLIENT_MAC[@]}"
        elif [[ $platform == "linux-32bit" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive::tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
        elif [[ $platform == "linux-64bit" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive::tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive::tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        elif [[ $platform == "linux-powerpc64" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive::tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive::tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        elif [[ $platform == "linux-arm64" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive::tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive::tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        elif [[ $platform == "linux-s390" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive::tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive::tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        else
          echo "++ ERROR: No release type defined for $platform"
        fi
      else
        if [[ $platform == "linux-64bit" || $platform == "linux-powerpc64" || $platform == "linux-arm64" || $platform == "linux-s390" ]]; then
          os::build::archive::tar "./*"
        else
          echo "++ ERROR: No release type defined for $platform"
        fi
      fi
      rm -rf "${release_binpath}"
    done
  )
}
readonly -f os::build::place_bins

# os::build::release_sha calculates a SHA256 checksum over the contents of the
# built release directory.
function os::build::release_sha() {
  pushd "${OS_OUTPUT_RELEASEPATH}" &> /dev/null
  find . -maxdepth 1 -type f | xargs sha256sum > CHECKSUM
  popd &> /dev/null
}
readonly -f os::build::release_sha

# os::build::make_openshift_binary_symlinks makes symlinks for the openshift
# binary in _output/local/bin/${platform}
function os::build::make_openshift_binary_symlinks() {
  platform=$(os::build::host_platform)
  if [[ -f "${OS_OUTPUT_BINPATH}/${platform}/openshift" ]]; then
    for linkname in "${OPENSHIFT_BINARY_SYMLINKS[@]}"; do
      ln -sf openshift "${OS_OUTPUT_BINPATH}/${platform}/${linkname}"
    done
  fi
}
readonly -f os::build::make_openshift_binary_symlinks

# os::build::get_product_vars exports variables that we expect to change
# depending on the distribution of Origin
function os::build::get_product_vars() {
  export OS_BUILD_LDFLAGS_IMAGE_PREFIX="${OS_IMAGE_PREFIX:-"openshift/origin"}"
  export OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS="${OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS:-"centos7"}"
  export OS_BUILD_LDFLAGS_FEDERATION_SERVER_IMAGE_NAME="${OS_BUILD_LDFLAGS_FEDERATION_SERVER_IMAGE_NAME:-"${OS_BUILD_LDFLAGS_IMAGE_PREFIX}-federation"}"
  export OS_BUILD_LDFLAGS_FEDERATION_ETCD_IMAGE="${OS_BUILD_LDFLAGS_FEDERATION_ETCD_IMAGE:-"quay.io/coreos/etcd:v3.1.7"}"
}

# golang 1.5 wants `-X key=val`, but golang 1.4- REQUIRES `-X key val`
function os::build::ldflag() {
  local key=${1}
  local val=${2}

  GO_VERSION=($(go version))
  if [[ -n $(echo "${GO_VERSION[2]}" | grep -E 'go1.4') ]]; then
    echo "-X ${key} ${val}"
  else
    echo "-X ${key}=${val}"
  fi
}
readonly -f os::build::ldflag

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
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/vendor/k8s.io/client-go/pkg/version.gitTreeState" "clean"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/federation/kubefed.serverImageName" "${OS_BUILD_LDFLAGS_FEDERATION_SERVER_IMAGE_NAME}"))
  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/federation/kubefed.defaultEtcdImage" "${OS_BUILD_LDFLAGS_FEDERATION_ETCD_IMAGE}"))

  # The -ldflags parameter takes a single string, so join the output.
  echo "${ldflags[*]-}"
}
readonly -f os::build::ldflags

# os::build::require_clean_tree exits if the current Git tree is not clean.
function os::build::require_clean_tree() {
  if ! git diff-index --quiet HEAD -- || test $(git ls-files --exclude-standard --others | wc -l) != 0; then
    echo "You can't have any staged or dirty files in $(pwd) for this command."
    echo "Either commit them or unstage them to continue."
    exit 1
  fi
}
readonly -f os::build::require_clean_tree

# os::build::commit_range takes one or two arguments - if the first argument is an
# integer, it is assumed to be a pull request and the local origin/pr/# branch is
# used to determine the common range with the second argument. If the first argument
# is not an integer, it is assumed to be a Git commit range and output directly.
function os::build::commit_range() {
  local remote
  remote="${UPSTREAM_REMOTE:-origin}"
  if [[ "$1" =~ ^-?[0-9]+$ ]]; then
    local target
    target="$(git rev-parse ${remote}/pr/$1)"
    if [[ $? -ne 0 ]]; then
      echo "Branch does not exist, or you have not configured ${remote}/pr/* style branches from GitHub" 1>&2
      exit 1
    fi

    local base
    base="$(git merge-base ${target} $2)"
    if [[ $? -ne 0 ]]; then
      echo "Branch has no common commits with $2" 1>&2
      exit 1
    fi
    if [[ "${base}" == "${target}" ]]; then

      # DO NOT TRUST THIS CODE
      merged="$(git rev-list --reverse ${target}..$2 --ancestry-path | head -1)"
      if [[ -z "${merged}" ]]; then
        echo "Unable to find the commit that merged ${remote}/pr/$1" 1>&2
        exit 1
      fi
      #if [[ $? -ne 0 ]]; then
      #  echo "Unable to find the merge commit for $1: ${merged}" 1>&2
      #  exit 1
      #fi
      echo "++ pr/$1 appears to have merged at ${merged}" 1>&2
      leftparent="$(git rev-list --parents -n 1 ${merged} | cut -f2 -d ' ')"
      if [[ $? -ne 0 ]]; then
        echo "Unable to find the left-parent for the merge of for $1" 1>&2
        exit 1
      fi
      base="$(git merge-base ${target} ${leftparent})"
      if [[ $? -ne 0 ]]; then
        echo "Unable to find the common commit between ${leftparent} and $1" 1>&2
        exit 1
      fi
      echo "${base}..${target}"
      exit 0
      #echo "Branch has already been merged to upstream master, use explicit range instead" 1>&2
      #exit 1
    fi

    echo "${base}...${target}"
    exit 0
  fi

  echo "$1"
}
readonly -f os::build::commit_range
