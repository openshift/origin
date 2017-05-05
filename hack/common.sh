#!/bin/bash

# This script provides common script functions for the hacks
# Requires OS_ROOT to be set

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
  if [[ -z "$(which go)" ]]; then
    cat <<EOF

Can't find 'go' in PATH, please fix and retry.
See http://golang.org/doc/install for installation instructions.

EOF
    exit 2
  fi

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
    if [[ "${go_version[2]}" < "go1.5" ]]; then
      cat <<EOF

Detected Go version: ${go_version[*]}.
Origin builds require Go version 1.6 or greater.

EOF
      exit 2
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

    # Temporarily enable swap for the duration of the build until we move
    # to Go 1.7
    os::build::enable_swap
    trap "os::build::disable_swap" EXIT

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
      os::build::get_version_vars
      mkdir -p "${OS_LOCAL_RELEASEPATH}"
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
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive_zip "${OS_BINARY_RELEASE_CLIENT_WINDOWS[@]}"
        elif [[ $platform == "mac" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive_zip "${OS_BINARY_RELEASE_CLIENT_MAC[@]}"
        elif [[ $platform == "linux-32bit" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive_tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
        elif [[ $platform == "linux-64bit" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive_tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive_tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        elif [[ $platform == "linux-powerpc64" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive_tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive_tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        elif [[ $platform == "linux-arm64" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive_tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive_tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        elif [[ $platform == "linux-s390" ]]; then
          OS_RELEASE_ARCHIVE="openshift-origin-client-tools" os::build::archive_tar "${OS_BINARY_RELEASE_CLIENT_LINUX[@]}"
          OS_RELEASE_ARCHIVE="openshift-origin-server" os::build::archive_tar "${OS_BINARY_RELEASE_SERVER_LINUX[@]}"
        else
          echo "++ ERROR: No release type defined for $platform"
        fi
      else
        if [[ $platform == "linux-64bit" || $platform == "linux-powerpc64" || $platform == "linux-arm64" || $platform == "linux-s390" ]]; then
          os::build::archive_tar "./*"
        else
          echo "++ ERROR: No release type defined for $platform"
        fi
      fi
      rm -rf "${release_binpath}"
    done
  )
}
readonly -f os::build::place_bins

function os::build::archive_name() {
  echo "${OS_RELEASE_ARCHIVE}-${OS_GIT_VERSION}-$1" | tr '+' '-'
}
readonly -f os::build::archive_name

function os::build::archive_zip() {
  local default_name
  default_name="$( os::build::archive_name "${platform}" ).zip"
  local archive_name="${archive_name:-$default_name}"
  echo "++ Creating ${archive_name}"
  for file in "$@"; do
    pushd "${release_binpath}" &> /dev/null
      sha256sum "${file}"
    popd &>/dev/null
    zip "${OS_LOCAL_RELEASEPATH}/${archive_name}" -qj "${release_binpath}/${file}"
  done
}
readonly -f os::build::archive_zip

function os::build::archive_tar() {
  local base_name
  base_name="$( os::build::archive_name "${platform}" )"
  local default_name="${base_name}.tar.gz"
  local archive_name="${archive_name:-$default_name}"
  echo "++ Creating ${archive_name}"
  pushd "${release_binpath}" &> /dev/null
  find . -type f -exec sha256sum {} \;
  if [[ -n "$(which bsdtar)" ]]; then
    bsdtar -czf "${OS_LOCAL_RELEASEPATH}/${archive_name}" -s ",^\.,${base_name}," $@
  else
    tar -czf --xattrs-exclude='LIBARCHIVE.xattr.security.selinux' "${OS_LOCAL_RELEASEPATH}/${archive_name}" --transform="s,^\.,${base_name}," $@
  fi
  popd &>/dev/null
}
readonly -f os::build::archive_tar

# Checks if the filesystem on a partition that the provided path points to is
# supporting hard links.
#
# Input:
#  $1 - the path where the hardlinks support test will be done.
# Returns:
#  0 - if hardlinks are supported
#  non-zero - if hardlinks aren't supported
function os::build::is_hardlink_supported() {
  local path="$1"
  # Determine if FS supports hard links
  local temp_file=$(TMPDIR="${path}" mktemp)
  ln "${temp_file}" "${temp_file}.link" &> /dev/null && unlink "${temp_file}.link" || local supported=$?
  rm -f "${temp_file}"
  return ${supported:-0}
}
readonly -f os::build::is_hardlink_supported

# Extract a tar.gz compressed archive in a given directory. If the
# archive contains hardlinks and the underlying filesystem is not
# supporting hardlinks then the a hard dereference will be done.
#
# Input:
#   $1 - path to archive file
#   $2 - directory where the archive will be extracted
function os::build::extract_tar() {
  local archive_file="$1"
  local change_dir="$2"

  if [[ -z "${archive_file}" ]]; then
    return 0
  fi

  local tar_flags="--strip-components=1"

  # Unpack archive
  echo "++ Extracting $(basename ${archive_file})"
  if [[ "${archive_file}" == *.zip ]]; then
    unzip -o "${archive_file}" -d "${change_dir}"
    return 0
  fi
  if os::build::is_hardlink_supported "${change_dir}" ; then
    # Ensure that tar won't try to set an owner when extracting to an
    # nfs mount. Setting ownership on an nfs mount is likely to fail
    # even for root.
    local mount_type=$(df -P -T "${change_dir}" | tail -n +2 | awk '{print $2}')
    if [[ "${mount_type}" = "nfs" ]]; then
      tar_flags="${tar_flags} --no-same-owner"
    fi
    tar mxzf "${archive_file}" ${tar_flags} -C "${change_dir}"
  else
    local temp_dir=$(TMPDIR=/dev/shm/ mktemp -d)
    tar mxzf "${archive_file}" ${tar_flags} -C "${temp_dir}"
    pushd "${temp_dir}" &> /dev/null
    tar cO --hard-dereference * | tar xf - -C "${change_dir}"
    popd &>/dev/null
    rm -rf "${temp_dir}"
  fi
}
readonly -f os::build::extract_tar

# os::build::release_sha calculates a SHA256 checksum over the contents of the
# built release directory.
function os::build::release_sha() {
  pushd "${OS_LOCAL_RELEASEPATH}" &> /dev/null
  sha256sum * > CHECKSUM
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

# os::build::detect_local_release_tars verifies there is only one primary and one
# image binaries release tar in OS_LOCAL_RELEASEPATH for the given platform specified by
# argument 1, exiting if more than one of either is found.
#
# If the tars are discovered, their full paths are exported to the following env vars:
#
#   OS_PRIMARY_RELEASE_TAR
#   OS_IMAGE_RELEASE_TAR
function os::build::detect_local_release_tars() {
  local platform="$1"

  if [[ ! -d "${OS_LOCAL_RELEASEPATH}" ]]; then
    echo "There are no release artifacts in ${OS_LOCAL_RELEASEPATH}"
    return 2
  fi
  if [[ ! -f "${OS_LOCAL_RELEASEPATH}/.commit" ]]; then
    echo "There is no release .commit identifier ${OS_LOCAL_RELEASEPATH}"
    return 2
  fi
  local primary=$(find ${OS_LOCAL_RELEASEPATH} -maxdepth 1 -type f -name openshift-origin-server-*-${platform}* \( -name *.tar.gz -or -name *.zip \))
  if [[ $(echo "${primary}" | wc -l) -ne 1 || -z "${primary}" ]]; then
    echo "There should be exactly one ${platform} server tar in $OS_LOCAL_RELEASEPATH"
    [[ -z "${WARN-}" ]] && return 2
  fi

  local client=$(find ${OS_LOCAL_RELEASEPATH} -maxdepth 1 -type f -name openshift-origin-client-tools-*-${platform}* \( -name *.tar.gz -or -name *.zip \))
  if [[ $(echo "${client}" | wc -l) -ne 1 || -z "${client}" ]]; then
    echo "There should be exactly one ${platform} client tar in $OS_LOCAL_RELEASEPATH"
    [[ -n "${WARN-}" ]] || return 2
  fi

  local image=$(find ${OS_LOCAL_RELEASEPATH} -maxdepth 1 -type f -name openshift-origin-image*-${platform}* \( -name *.tar.gz -or -name *.zip \))
  if [[ $(echo "${image}" | wc -l) -ne 1 || -z "${image}" ]]; then
    echo "There should be exactly one ${platform} image tar in $OS_LOCAL_RELEASEPATH"
    [[ -n "${WARN-}" ]] || return 2
  fi

  export OS_PRIMARY_RELEASE_TAR="${primary}"
  export OS_IMAGE_RELEASE_TAR="${image}"
  export OS_CLIENT_RELEASE_TAR="${client}"
  export OS_RELEASE_COMMIT="$(cat ${OS_LOCAL_RELEASEPATH}/.commit)"
}
readonly -f os::build::detect_local_release_tars

# os::build::get_version_vars loads the standard version variables as
# ENV vars
function os::build::get_version_vars() {
  if [[ -n "${OS_VERSION_FILE-}" ]]; then
    if [[ -f "${OS_VERSION_FILE}" ]]; then
      source "${OS_VERSION_FILE}"
      return
    fi
    if [[ ! -d "${OS_ROOT}/.git" ]]; then
      os::log::warning "No version file at ${OS_VERSION_FILE}"
      exit 1
    fi
    os::log::warning "No version file at ${OS_VERSION_FILE}, falling back to git versions"
  fi
  os::build::os_version_vars
  os::build::kube_version_vars
  os::build::etcd_version_vars
}
readonly -f os::build::get_version_vars

# os::build::os_version_vars looks up the current Git vars
function os::build::os_version_vars() {
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
    if [[ -n ${OS_GIT_VERSION-} ]] || OS_GIT_VERSION=$("${git[@]}" describe --long --tags --abbrev=7 "${OS_GIT_COMMIT}^{commit}" 2>/dev/null); then
      # Try to match the "git describe" output to a regex to try to extract
      # the "major" and "minor" versions and whether this is the exact tagged
      # version or whether the tree is between two tagged versions.
      if [[ "${OS_GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+)*([-].*)?$ ]]; then
        OS_GIT_MAJOR=${BASH_REMATCH[1]}
        OS_GIT_MINOR=${BASH_REMATCH[2]}
        if [[ -n "${BASH_REMATCH[4]}" ]]; then
          OS_GIT_MINOR+="+"
        fi
      fi

      # This translates the "git describe" to an actual semver.org
      # compatible semantic version that looks something like this:
      #   v1.1.0-alpha.0.6+84c76d1-345
      OS_GIT_VERSION=$(echo "${OS_GIT_VERSION}" | sed "s/-\([0-9]\{1,\}\)-g\([0-9a-f]\{7,40\}\)$/\+\2-\1/")
      # If this is an exact tag, remove the last segment.
      OS_GIT_VERSION=$(echo "${OS_GIT_VERSION}" | sed "s/-0$//")
      if [[ "${OS_GIT_TREE_STATE}" == "dirty" ]]; then
        # git describe --dirty only considers changes to existing files, but
        # that is problematic since new untracked .go files affect the build,
        # so use our idea of "dirty" from git status instead.
        OS_GIT_VERSION+="-dirty"
      fi
    fi
  fi
}
readonly -f os::build::os_version_vars

function os::build::etcd_version_vars() {
  ETCD_GIT_VERSION=$(go run "${OS_ROOT}/tools/godepversion/godepversion.go" "${OS_ROOT}/Godeps/Godeps.json" "github.com/coreos/etcd/etcdserver" "comment")
  ETCD_GIT_COMMIT=$(go run "${OS_ROOT}/tools/godepversion/godepversion.go" "${OS_ROOT}/Godeps/Godeps.json" "github.com/coreos/etcd/etcdserver")
}
readonly -f os::build::etcd_version_vars

# os::build::kube_version_vars returns the version of Kubernetes we have
# vendored.
function os::build::kube_version_vars() {
  KUBE_GIT_VERSION=$(go run "${OS_ROOT}/tools/godepversion/godepversion.go" "${OS_ROOT}/Godeps/Godeps.json" "k8s.io/kubernetes/pkg/api" "comment")
  KUBE_GIT_COMMIT=$(go run "${OS_ROOT}/tools/godepversion/godepversion.go" "${OS_ROOT}/Godeps/Godeps.json" "k8s.io/kubernetes/pkg/api")

  # This translates the "git describe" to an actual semver.org
  # compatible semantic version that looks something like this:
  #   v1.1.0-alpha.0.6+84c76d1142ea4d
  #
  # TODO: We continue calling this "git version" because so many
  # downstream consumers are expecting it there.
  KUBE_GIT_VERSION=$(echo "${KUBE_GIT_VERSION}" | sed "s/-\([0-9]\{1,\}\)-g\([0-9a-f]\{7,40\}\)$/\+\2/")

  # Try to match the "git describe" output to a regex to try to extract
  # the "major" and "minor" versions and whether this is the exact tagged
  # version or whether the tree is between two tagged versions.
  if [[ "${KUBE_GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+)*([-].*)?$ ]]; then
    KUBE_GIT_MAJOR=${BASH_REMATCH[1]}
    KUBE_GIT_MINOR=${BASH_REMATCH[2]}
    if [[ -n "${BASH_REMATCH[4]}" ]]; then
      KUBE_GIT_MINOR+="+"
    fi
  fi
}
readonly -f os::build::kube_version_vars

# Saves the environment flags to $1
function os::build::save_version_vars() {
  local version_file=${1-}
  if [[ -z ${version_file} ]]; then
    echo "!!! Internal error.  No file specified in os::build::save_version_vars"
    return 1
  fi

  cat <<EOF >"${version_file}"
OS_GIT_COMMIT='${OS_GIT_COMMIT-}'
OS_GIT_TREE_STATE='${OS_GIT_TREE_STATE-}'
OS_GIT_VERSION='${OS_GIT_VERSION-}'
OS_GIT_MAJOR='${OS_GIT_MAJOR-}'
OS_GIT_MINOR='${OS_GIT_MINOR-}'
KUBE_GIT_COMMIT='${KUBE_GIT_COMMIT-}'
KUBE_GIT_VERSION='${KUBE_GIT_VERSION-}'
ETCD_GIT_VERSION='${ETCD_GIT_VERSION-}'
ETCD_GIT_COMMIT='${ETCD_GIT_COMMIT-}'
EOF
}
readonly -f os::build::save_version_vars

# os::build::get_product_vars exports variables that we expect to change
# depending on the distribution of Origin
function os::build::get_product_vars() {
  export OS_BUILD_LDFLAGS_IMAGE_PREFIX="${OS_IMAGE_PREFIX:-"openshift/origin"}"
  export OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS="${OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS:-"centos7"}"
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

  os::build::get_version_vars
  os::build::get_product_vars

  local buildDate="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

  declare -a ldflags=()

  ldflags+=($(os::build::ldflag "${OS_GO_PACKAGE}/pkg/bootstrap/docker.defaultImageStreams" "${OS_BUILD_LDFLAGS_DEFAULT_IMAGE_STREAMS}"))
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

  # The -ldflags parameter takes a single string, so join the output.
  echo "${ldflags[*]-}"
}
readonly -f os::build::ldflags

# os::build::image builds an image from a directory, to a tag, with an optional dockerfile to
# use as the third argument. The environment variable OS_BUILD_IMAGE_ARGS adds additional
# options to the command. The default is to use the imagebuilder binary if it is available
# on the path with fallback to docker build if it is not available.
function os::build::image() {
  local directory=$1
  local tag=$2
  local dockerfile="${3-}"
  local extra_tag="${4-}"
  local options="${OS_BUILD_IMAGE_ARGS-}"
  local mode="${OS_BUILD_IMAGE_TYPE:-imagebuilder}"

  if [[ "${mode}" == "imagebuilder" ]]; then
    if os::util::find::system_binary 'imagebuilder'; then
      if [[ -n "${extra_tag}" ]]; then
        extra_tag="-t '${extra_tag}'"
      fi
      if [[ -n "${dockerfile}" ]]; then
        eval "imagebuilder -f '${dockerfile}' -t '${tag}' ${extra_tag} ${options} '${directory}'"
        return $?
      fi
      eval "imagebuilder -t '${tag}' ${extra_tag} ${options} '${directory}'"
      return $?
    fi

    os::log::warning "Unable to locate 'imagebuilder' on PATH, falling back to Docker build"
    # clear options since we were unable to select imagebuilder
    options=""
  fi

  if [[ -n "${dockerfile}" ]]; then
    eval "docker build -f '${dockerfile}' -t '${tag}' ${options} '${directory}'"
    if [[ -n "${extra_tag}" ]]; then
      docker tag "${tag}" "${extra_tag}"
    fi
    return $?
  fi
  eval "docker build -t '${tag}' ${options} '${directory}'"
  if [[ -n "${extra_tag}" ]]; then
    docker tag "${tag}" "${extra_tag}"
  fi
  return $?
}
readonly -f os::build::image

# os::build::enable_swap attempts to enable swap for the system if a) this is Linux and b)
# the amount of physical memory is less than 10GB. This is a stopgap until we have
# better control over memory use in Go 1.7+.
function os::build::enable_swap() {
  # if we aren't on linux or have more than 9GB of memory
  if [[ -n "${OS_BUILD_SWAP_DISABLE-}" || "$(go env GOHOSTOS)" != "linux" || "$( os::build::physmem_gb )" -gt 9 ]]; then
    return
  fi
  # if we don't have the swapon command available
  if ! swapon &>/dev/null; then
    return
  fi
  # if swap is already on
  if [[ "$( swapon --show --noheadings | wc -l )" -ne 0 ]]; then
    return
  fi
  echo "++ temporarily enabling swap space to assist in building in limited memory - use OS_BUILD_SWAP_DISABLE=1 to bypass"

  (
    set -e
    sudo cp /etc/fstab /origin-backupfstab
    sudo dd if=/dev/zero of=/origin-swapfile bs=1M count=${OS_BUILD_SWAP_SIZE:-2048}
    sudo chmod 600 /origin-swapfile
    sudo mkswap /origin-swapfile
    sudo swapon /origin-swapfile
    sudo /bin/sh -c 'echo "/origin-swapfile none swap defaults 0 0" >> /etc/fstab'
  ) &>/dev/null || true
}
readonly -f os::build::enable_swap

# os::build::disable_swap undoes the effects of os::build::enable_swap
function os::build::disable_swap() {
  # if we aren't on linux or have more than 9GB of memory
  if [[ -n "${OS_BUILD_SWAP_DISABLE-}" || "$(go env GOHOSTOS)" != "linux" ]]; then
    return
  fi
  # if we previously set up a swapfile
  if [[ ! -f /origin-swapfile ]]; then
    return
  fi
  (
    set +e
    sudo swapoff /origin-swapfile
    sudo cp /origin-backupfstab /etc/fstab
    sudo rm -f /origin-backupfstab /origin-swapfile
  ) || true
}
readonly -f os::build::disable_swap

# os::build::physmem_gb returns the approximate gigabytes of memory on the system
# This is copied verbatim from kube for the purposes of detecting how much available
# memory we have for building.
function os::build::physmem_gb() {
  local mem

  # Linux kernel version >=3.14, in kb
  if mem=$(grep MemAvailable /proc/meminfo | awk '{ print $2 }'); then
    echo $(( ${mem} / 1048576 ))
    return
  fi

  # Linux, in kb
  if mem=$(grep MemTotal /proc/meminfo | awk '{ print $2 }'); then
    echo $(( ${mem} / 1048576 ))
    return
  fi

  # OS X, in bytes. Note that get_physmem, as used, should only ever
  # run in a Linux container (because it's only used in the multiple
  # platform case, which is a Dockerized build), but this is provided
  # for completeness.
  if mem=$(sysctl -n hw.memsize 2>/dev/null); then
    echo $(( ${mem} / 1073741824 ))
    return
  fi

  # If we can't infer it, just give up and assume a low memory system
  echo 1
}
readonly -f os::build::physmem_gb

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

function os::build::gen-completions() {
  local dest="$1"
  local shell="$2"
  local skipprefix="${3:-}"

  # We do this in a tmpdir in case the dest has other non-autogenned files
  # We don't want to include them in the list of gen'd files
  local tmpdir="${OS_ROOT}/_tmp/gen_comp"
  mkdir -p "${tmpdir}"
  # generate the new files
  ${OS_OUTPUT_BINPATH}/${platform}/oc completion ${shell} > $tmpdir/oc
  ${OS_OUTPUT_BINPATH}/${platform}/openshift completion ${shell} > $tmpdir/openshift
  ${OS_OUTPUT_BINPATH}/${platform}/oadm completion ${shell} > $tmpdir/oadm
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
readonly -f os::build::gen-completions

function os::build::get-bin-output-path() {
  local os_root="${1:-}"

  if [[ -n "${os_root}" ]]; then
    os_root="${os_root}/"
  fi
  echo ${os_root}_output/local/bin/$(os::build::host_platform)
}
readonly -f os::build::get-bin-output-path
