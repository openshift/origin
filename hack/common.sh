#!/bin/bash

# This script provides common script functions for the hacks
# Requires OS_ROOT to be set

set -o errexit
set -o nounset
set -o pipefail

# The root of the build/dist directory
OS_ROOT=$(
  unset CDPATH
  os_root=$(dirname "${BASH_SOURCE}")/..
  cd "${os_root}"
  pwd
)

OS_OUTPUT_SUBPATH="${OS_OUTPUT_SUBPATH:-_output/local}"
OS_OUTPUT="${OS_ROOT}/${OS_OUTPUT_SUBPATH}"
OS_OUTPUT_BINPATH="${OS_OUTPUT}/bin"
OS_LOCAL_BINPATH="${OS_OUTPUT}/go/bin"
OS_LOCAL_RELEASEPATH="${OS_OUTPUT}/releases"

readonly OS_GO_PACKAGE=github.com/openshift/origin
readonly OS_GOPATH="${OS_OUTPUT}/go"

readonly OS_IMAGE_COMPILE_PLATFORMS=(
  linux/amd64
)
readonly OS_IMAGE_COMPILE_TARGETS=(
  images/pod
  cmd/dockerregistry
  cmd/gitserver
)
readonly OS_SCRATCH_IMAGE_COMPILE_TARGETS=(
  examples/hello-openshift
  examples/deployment
)
readonly OS_IMAGE_COMPILE_BINARIES=("${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]##*/}" "${OS_IMAGE_COMPILE_TARGETS[@]##*/}")

readonly OS_CROSS_COMPILE_PLATFORMS=(
  linux/amd64
  darwin/amd64
  windows/amd64
)
readonly OS_CROSS_COMPILE_TARGETS=(
  cmd/openshift
  cmd/oc
)
readonly OS_CROSS_COMPILE_BINARIES=("${OS_CROSS_COMPILE_TARGETS[@]##*/}")

readonly OS_ALL_TARGETS=(
  "${OS_CROSS_COMPILE_TARGETS[@]}"
)
readonly OS_ALL_BINARIES=("${OS_ALL_TARGETS[@]##*/}")

#If you update this list, be sure to get the images/origin/Dockerfile
readonly OPENSHIFT_BINARY_SYMLINKS=(
  openshift-router
  openshift-deploy
  openshift-sti-build
  openshift-docker-build
  origin
  atomic-enterprise
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
)

# os::build::binaries_from_targets take a list of build targets and return the
# full go package to be built
os::build::binaries_from_targets() {
  local target
  for target; do
    echo "${OS_GO_PACKAGE}/${target}"
  done
}

# Asks golang what it thinks the host platform is.  The go tool chain does some
# slightly different things when the target platform matches the host platform.
os::build::host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}

# Build binaries targets specified
#
# Input:
#   $@ - targets and go flags.  If no targets are set then all binaries targets
#     are built.
#   OS_BUILD_PLATFORMS - Incoming variable of targets to build for.  If unset
#     then just the host architecture is built.
os::build::build_binaries() {
  # Create a sub-shell so that we don't pollute the outer environment
  (
    # Check for `go` binary and set ${GOPATH}.
    os::build::setup_env

    # Fetch the version.
    local version_ldflags
    version_ldflags=$(os::build::ldflags)

    # Use eval to preserve embedded quoted strings.
    local goflags
    eval "goflags=(${OS_GOFLAGS:-})"

    local -a targets=()
    local arg
    for arg; do
      if [[ "${arg}" == -* ]]; then
        # Assume arguments starting with a dash are flags to pass to go.
        goflags+=("${arg}")
      else
        targets+=("${arg}")
      fi
    done

    if [[ ${#targets[@]} -eq 0 ]]; then
      targets=("${OS_ALL_TARGETS[@]}")
    fi

    local -a platforms=("${OS_BUILD_PLATFORMS[@]:+${OS_BUILD_PLATFORMS[@]}}")
    if [[ ${#platforms[@]} -eq 0 ]]; then
      platforms=("$(os::build::host_platform)")
    fi

    local binaries
    binaries=($(os::build::binaries_from_targets "${targets[@]}"))

    local platform
    for platform in "${platforms[@]}"; do
      os::build::set_platform_envs "${platform}"
      echo "++ Building go targets for ${platform}:" "${targets[@]}"
      go install "${goflags[@]:+${goflags[@]}}" \
          -ldflags "${version_ldflags}" \
          "${binaries[@]}"
      os::build::unset_platform_envs "${platform}"
    done
  )
}

# Takes the platform name ($1) and sets the appropriate golang env variables
# for that platform.
os::build::set_platform_envs() {
  [[ -n ${1-} ]] || {
    echo "!!! Internal error.  No platform set in os::build::set_platform_envs"
    exit 1
  }

  export GOOS=${platform%/*}
  export GOARCH=${platform##*/}
}

# Takes the platform name ($1) and resets the appropriate golang env variables
# for that platform.
os::build::unset_platform_envs() {
  unset GOOS
  unset GOARCH
}

# Create the GOPATH tree under $OS_ROOT
os::build::create_gopath_tree() {
  local go_pkg_dir="${OS_GOPATH}/src/${OS_GO_PACKAGE}"
  local go_pkg_basedir=$(dirname "${go_pkg_dir}")

  mkdir -p "${go_pkg_basedir}"
  rm -f "${go_pkg_dir}"

  # TODO: This symlink should be relative.
  ln -s "${OS_ROOT}" "${go_pkg_dir}"
}

# os::build::setup_env will check that the `go` commands is available in
# ${PATH}. If not running on Travis, it will also check that the Go version is
# good enough for the Kubernetes build.
#
# Input Vars:
#   OS_EXTRA_GOPATH - If set, this is included in created GOPATH
#   OS_NO_GODEPS - If set, we don't add 'Godeps/_workspace' to GOPATH
#
# Output Vars:
#   export GOPATH - A modified GOPATH to our created tree along with extra
#     stuff.
#   export GOBIN - This is actively unset if already set as we want binaries
#     placed in a predictable place.
os::build::setup_env() {
  os::build::create_gopath_tree

  if [[ -z "$(which go)" ]]; then
    cat <<EOF

Can't find 'go' in PATH, please fix and retry.
See http://golang.org/doc/install for installation instructions.

EOF
    exit 2
  fi

  # Travis continuous build uses a head go release that doesn't report
  # a version number, so we skip this check on Travis.  It's unnecessary
  # there anyway.
  if [[ "${TRAVIS:-}" != "true" ]]; then
    local go_version
    go_version=($(go version))
    if [[ "${go_version[2]}" < "go1.4" ]]; then
      cat <<EOF

Detected go version: ${go_version[*]}.
OpenShift and Kubernetes requires go version 1.4 or greater.
Please install Go version 1.4 or later.

EOF
      exit 2
    fi
  fi

  GOPATH=${OS_GOPATH}

  # Append OS_EXTRA_GOPATH to the GOPATH if it is defined.
  if [[ -n ${OS_EXTRA_GOPATH:-} ]]; then
    GOPATH="${GOPATH}:${OS_EXTRA_GOPATH}"
  fi

  # Append the tree maintained by `godep` to the GOPATH unless OS_NO_GODEPS
  # is defined.
  if [[ -z ${OS_NO_GODEPS:-} ]]; then
    GOPATH="${GOPATH}:${OS_ROOT}/Godeps/_workspace"
  fi
  export GOPATH

  # Unset GOBIN in case it already exists in the current session.
  unset GOBIN
}

# This will take OS_RELEASE_BINARIES from $GOPATH/bin and copy them to the appropriate
# place in ${OS_OUTPUT_BINDIR}
#
# If OS_RELEASE_ARCHIVE is set, tar archives prefixed with OS_RELEASE_ARCHIVE for
# each OS_RELEASE_PLATFORMS are created.
#
# Ideally this wouldn't be necessary and we could just set GOBIN to
# OS_OUTPUT_BINDIR but that won't work in the face of cross compilation.  'go
# install' will place binaries that match the host platform directly in $GOBIN
# while placing cross compiled binaries into `platform_arch` subdirs.  This
# complicates pretty much everything else we do around packaging and such.
os::build::place_bins() {
  (
    local host_platform
    host_platform=$(os::build::host_platform)

    echo "++ Placing binaries"

    if [[ "${OS_RELEASE_ARCHIVE-}" != "" ]]; then
      os::build::get_version_vars
      mkdir -p "${OS_LOCAL_RELEASEPATH}"
    fi

    for platform in "${OS_RELEASE_PLATFORMS[@]-(host_platform)}"; do
      # The substitution on platform_src below will replace all slashes with
      # underscores.  It'll transform darwin/amd64 -> darwin_amd64.
      local platform_src="/${platform//\//_}"
      if [[ $platform == $host_platform ]]; then
        platform_src=""
      fi

      # Skip this directory if the platform has no binaries.
      local full_binpath_src="${OS_GOPATH}/bin${platform_src}"
      if [[ ! -d "${full_binpath_src}" ]]; then
        continue
      fi

      mkdir -p "${OS_OUTPUT_BINPATH}/${platform}"

      # Create an array of binaries to release. Append .exe variants if the platform is windows.
      local -a binaries=()
      local binary
      for binary in "${OS_RELEASE_BINARIES[@]}"; do
        binaries+=("${binary}")
        if [[ $platform == "windows/amd64" ]]; then
          binaries+=("${binary}.exe")
        fi
      done

      # Copy the only the specified release binaries to the shared OS_OUTPUT_BINPATH.
      local -a includes=()
      for binary in "${binaries[@]}"; do
        includes+=("--include=${binary}")
      done
      find "${full_binpath_src}" -maxdepth 1 -type f -exec \
        rsync "${includes[@]}" --exclude="*" -pt {} "${OS_OUTPUT_BINPATH}/${platform}" \;

      # If no release archive was requested, we're done.
      if [[ "${OS_RELEASE_ARCHIVE-}" == "" ]]; then
        continue
      fi

      # Create a temporary bin directory containing only the binaries marked for release.
      local release_binpath=$(mktemp -d openshift.release.${OS_RELEASE_ARCHIVE}.XXX)
      find "${full_binpath_src}" -maxdepth 1 -type f -exec \
        rsync "${includes[@]}" --exclude="*" -pt {} "${release_binpath}" \;

      # Create binary copies where specified.
      local suffix=""
      if [[ $platform == "windows/amd64" ]]; then
        suffix=".exe"
      fi
      for linkname in "${OPENSHIFT_BINARY_COPY[@]}"; do
        local src="${release_binpath}/openshift${suffix}"
        if [[ -f "${src}" ]]; then
          cp "${release_binpath}/openshift${suffix}" "${release_binpath}/${linkname}${suffix}"
        fi
      done

      # Create the release archive.
      local platform_segment="${platform//\//-}"
      local archive_name="${OS_RELEASE_ARCHIVE}-${OS_GIT_VERSION}-${OS_GIT_COMMIT}-${platform_segment}.tar.gz"

      echo "++ Creating ${archive_name}"
      tar -czf "${OS_LOCAL_RELEASEPATH}/${archive_name}" -C "${release_binpath}" .
      rm -rf "${release_binpath}"
    done
  )
}

# os::build::make_openshift_binary_symlinks makes symlinks for the openshift
# binary in _output/local/go/bin
os::build::make_openshift_binary_symlinks() {
  if [[ -f "${OS_LOCAL_BINPATH}/openshift" ]]; then
    for linkname in "${OPENSHIFT_BINARY_SYMLINKS[@]}"; do
      ln -sf "${OS_LOCAL_BINPATH}/openshift" "${OS_LOCAL_BINPATH}/${linkname}"
    done
  fi
}

# os::build::detect_local_release_tars verifies there is only one primary and one
# image binaries release tar in OS_LOCAL_RELEASEPATH for the given platform specified by
# argument 1, exiting if more than one of either is found.
#
# If the tars are discovered, their full paths are exported to the following env vars:
#
#   OS_PRIMARY_RELEASE_TAR
#   OS_IMAGE_RELEASE_TAR
os::build::detect_local_release_tars() {
  local platform="$1"

  local primary=$(find ${OS_LOCAL_RELEASEPATH} -maxdepth 1 -type f -name openshift-origin-*-${platform}-* | grep -v image)
  if [[ $(echo "${primary}" | wc -l) -ne 1 ]]; then
    echo "There should be exactly one ${platform} primary tar in $OS_LOCAL_RELEASEPATH"
    exit 2
  fi

  local image=$(find ${OS_LOCAL_RELEASEPATH} -maxdepth 1 -type f -name openshift-origin-image*-${platform}-*)
  if [[ $(echo "${image}" | wc -l) -ne 1 ]]; then
    echo "There should be exactly one ${platform} image tar in $OS_LOCAL_RELEASEPATH"
    exit 3
  fi

  export OS_PRIMARY_RELEASE_TAR="${primary}"
  export OS_IMAGE_RELEASE_TAR="${image}"
  export OS_RELEASE_COMMIT="$(cat ${OS_LOCAL_RELEASEPATH}/.commit)"
}

# os::build::get_version_vars loads the standard version variables as
# ENV vars
os::build::get_version_vars() {
  if [[ -n ${OS_VERSION_FILE-} ]]; then
    source "${OS_VERSION_FILE}"
    return
  fi
  os::build::os_version_vars
  os::build::kube_version_vars
}

# os::build::os_version_vars looks up the current Git vars
os::build::os_version_vars() {
  local git=(git --work-tree "${OS_ROOT}")

  if [[ -n ${OS_GIT_COMMIT-} ]] || OS_GIT_COMMIT=$("${git[@]}" rev-parse --short "HEAD^{commit}" 2>/dev/null); then
    if [[ -z ${OS_GIT_TREE_STATE-} ]]; then
      # Check if the tree is dirty.  default to dirty
      if git_status=$("${git[@]}" status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
        OS_GIT_TREE_STATE="clean"
      else
        OS_GIT_TREE_STATE="dirty"
      fi
    fi

    # Use git describe to find the version based on annotated tags.
    if [[ -n ${OS_GIT_VERSION-} ]] || OS_GIT_VERSION=$("${git[@]}" describe "${OS_GIT_COMMIT}^{commit}" 2>/dev/null); then
      if [[ "${OS_GIT_TREE_STATE}" == "dirty" ]]; then
        # git describe --dirty only considers changes to existing files, but
        # that is problematic since new untracked .go files affect the build,
        # so use our idea of "dirty" from git status instead.
        OS_GIT_VERSION+="-dirty"
      fi

      # Try to match the "git describe" output to a regex to try to extract
      # the "major" and "minor" versions and whether this is the exact tagged
      # version or whether the tree is between two tagged versions.
      if [[ "${OS_GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)([.-].*)?$ ]]; then
        OS_GIT_MAJOR=${BASH_REMATCH[1]}
        OS_GIT_MINOR=${BASH_REMATCH[2]}
        if [[ -n "${BASH_REMATCH[3]}" ]]; then
          OS_GIT_MINOR+="+"
        fi
      fi
    fi
  fi
}

# os::build::kube_version_vars returns the version of Kubernetes we have
# vendored.
os::build::kube_version_vars() {
  KUBE_GIT_VERSION=$(go run "${OS_ROOT}/hack/version.go" "${OS_ROOT}/Godeps/Godeps.json" "github.com/GoogleCloudPlatform/kubernetes/pkg/api" "comment")
  KUBE_GIT_COMMIT=$(go run "${OS_ROOT}/hack/version.go" "${OS_ROOT}/Godeps/Godeps.json" "github.com/GoogleCloudPlatform/kubernetes/pkg/api")
}

# Saves the environment flags to $1
os::build::save_version_vars() {
  local version_file=${1-}
  [[ -n ${version_file} ]] || {
    echo "!!! Internal error.  No file specified in os::build::save_version_vars"
    return 1
  }

  cat <<EOF >"${version_file}"
OS_GIT_COMMIT='${OS_GIT_COMMIT-}'
OS_GIT_TREE_STATE='${OS_GIT_TREE_STATE-}'
OS_GIT_VERSION='${OS_GIT_VERSION-}'
OS_GIT_MAJOR='${OS_GIT_MAJOR-}'
OS_GIT_MINOR='${OS_GIT_MINOR-}'
KUBE_GIT_COMMIT='${KUBE_GIT_COMMIT-}'
KUBE_GIT_VERSION='${KUBE_GIT_VERSION-}'
EOF
}

# os::build::ldflags calculates the -ldflags argument for building OpenShift
os::build::ldflags() {
  (
    # Run this in a subshell to prevent settings/variables from leaking.
    set -o errexit
    set -o nounset
    set -o pipefail

    cd "${OS_ROOT}"

    os::build::get_version_vars

    declare -a ldflags=()
    ldflags+=(-X "${OS_GO_PACKAGE}/pkg/version.majorFromGit" "${OS_GIT_MAJOR}")
    ldflags+=(-X "${OS_GO_PACKAGE}/pkg/version.minorFromGit" "${OS_GIT_MINOR}")
    ldflags+=(-X "${OS_GO_PACKAGE}/pkg/version.versionFromGit" "${OS_GIT_VERSION}")
    ldflags+=(-X "${OS_GO_PACKAGE}/pkg/version.commitFromGit" "${OS_GIT_COMMIT}")
    ldflags+=(-X "github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit" "${KUBE_GIT_COMMIT}")
    ldflags+=(-X "github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion" "${KUBE_GIT_VERSION}")

    # The -ldflags parameter takes a single string, so join the output.
    echo "${ldflags[*]-}"
  )
}

os::build::gen-docs() {
  local cmd="$1"
  local dest="$2"
  local skipprefix="${3:-}"

  # We do this in a tmpdir in case the dest has other non-autogenned files
  # We don't want to include them in the list of gen'd files
  local tmpdir="${OS_ROOT}/_tmp/gen_doc"
  mkdir -p "${tmpdir}"
  # generate the new files
  ${cmd} "${tmpdir}"
  # create the list of generated files
  ls "${tmpdir}" | LC_ALL=C sort > "${tmpdir}/.files_generated"

  # remove all old generated file from the destination
  while read file; do
    if [[ -e "${tmpdir}/${file}" && -n "${skipprefix}" ]]; then
      local original generated
      original=$(grep -v "^${skipprefix}" "${dest}/${file}") || :
      generated=$(grep -v "^${skipprefix}" "${tmpdir}/${file}") || :
      if [[ "${original}" == "${generated}" ]]; then
        # overwrite generated with original.
        mv "${dest}/${file}" "${tmpdir}/${file}"
      fi
    else
      rm "${dest}/${file}" || true
    fi
  done <"${dest}/.files_generated"

  # put the new generated file into the destination
  find "${tmpdir}" -exec rsync -pt {} "${dest}" \; >/dev/null
  #cleanup
  rm -rf "${tmpdir}"

  echo "Assets generated in ${dest}"
}