#!/bin/bash

# This script provides common script functions for the hacks
# Requires S2I_ROOT to be set

set -o errexit
set -o nounset
set -o pipefail

# The root of the build/dist directory
S2I_ROOT=$(
  unset CDPATH
  sti_root=$(dirname "${BASH_SOURCE}")/..
  cd "${sti_root}"
  pwd
)

S2I_OUTPUT_SUBPATH="${S2I_OUTPUT_SUBPATH:-_output/local}"
S2I_OUTPUT="${S2I_ROOT}/${S2I_OUTPUT_SUBPATH}"
S2I_OUTPUT_BINPATH="${S2I_OUTPUT}/bin"
S2I_OUTPUT_PKGDIR="${S2I_OUTPUT}/pkgdir"
S2I_LOCAL_BINPATH="${S2I_OUTPUT}/go/bin"
S2I_LOCAL_RELEASEPATH="${S2I_OUTPUT}/releases"
RELEASE_LDFLAGS=${RELEASE_LDFLAGS:-""}


readonly S2I_GO_PACKAGE=github.com/openshift/source-to-image
readonly S2I_GOPATH="${S2I_OUTPUT}/go"

readonly S2I_CROSS_COMPILE_PLATFORMS=(
  linux/amd64
  darwin/amd64
  windows/amd64
  linux/386
)
readonly S2I_CROSS_COMPILE_TARGETS=(
  cmd/s2i
)
readonly S2I_CROSS_COMPILE_BINARIES=("${S2I_CROSS_COMPILE_TARGETS[@]##*/}")

readonly S2I_ALL_TARGETS=(
  "${S2I_CROSS_COMPILE_TARGETS[@]}"
)

readonly S2I_BINARY_SYMLINKS=(
  sti
)
readonly S2I_BINARY_RELEASE_WINDOWS=(
  sti.exe
  s2i.exe
)

# s2i::build::binaries_from_targets take a list of build targets and return the
# full go package to be built
s2i::build::binaries_from_targets() {
  local target
  for target; do
    echo "${S2I_GO_PACKAGE}/${target}"
  done
}

# Asks golang what it thinks the host platform is.  The go tool chain does some
# slightly different things when the target platform matches the host platform.
s2i::build::host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}


# Build binaries targets specified
#
# Input:
#   $@ - targets and go flags.  If no targets are set then all binaries targets
#     are built.
#   S2I_BUILD_PLATFORMS - Incoming variable of targets to build for.  If unset
#     then just the host architecture is built.
s2i::build::build_binaries() {
  # Create a sub-shell so that we don't pollute the outer environment
  (
    # Check for `go` binary and set ${GOPATH}.
    s2i::build::setup_env

    # Fetch the version.
    local version_ldflags
    version_ldflags=$(s2i::build::ldflags)

    s2i::build::export_targets "$@"

    local platform
    for platform in "${platforms[@]}"; do
      s2i::build::set_platform_envs "${platform}"
      echo "++ Building go targets for ${platform}:" "${targets[@]}"
      CGO_ENABLED=0 go install "${goflags[@]:+${goflags[@]}}" \
          -pkgdir "${S2I_OUTPUT_PKGDIR}" \
          -ldflags "${version_ldflags} ${RELEASE_LDFLAGS}" \
          "${binaries[@]}"
      s2i::build::unset_platform_envs "${platform}"
    done
  )
}

# Generates the set of target packages, binaries, and platforms to build for.
# Accepts binaries via $@, and platforms via S2I_BUILD_PLATFORMS, or defaults to
# the current platform.
s2i::build::export_targets() {
  # Use eval to preserve embedded quoted strings.
  local goflags
  eval "goflags=(${S2I_GOFLAGS:-})"

  targets=()
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
    targets=("${S2I_ALL_TARGETS[@]}")
  fi

  binaries=($(s2i::build::binaries_from_targets "${targets[@]}"))

  platforms=("${S2I_BUILD_PLATFORMS[@]:+${S2I_BUILD_PLATFORMS[@]}}")
  if [[ ${#platforms[@]} -eq 0 ]]; then
    platforms=("$(s2i::build::host_platform)")
  fi
}


# Takes the platform name ($1) and sets the appropriate golang env variables
# for that platform.
s2i::build::set_platform_envs() {
  [[ -n ${1-} ]] || {
    echo "!!! Internal error.  No platform set in s2i::build::set_platform_envs"
    exit 1
  }

  export GOOS=${platform%/*}
  export GOARCH=${platform##*/}
}

# Takes the platform name ($1) and resets the appropriate golang env variables
# for that platform.
s2i::build::unset_platform_envs() {
  unset GOOS
  unset GOARCH
}


# Create the GOPATH tree under $S2I_ROOT
s2i::build::create_gopath_tree() {
  local go_pkg_dir="${S2I_GOPATH}/src/${S2I_GO_PACKAGE}"
  local go_pkg_basedir=$(dirname "${go_pkg_dir}")

  mkdir -p "${go_pkg_basedir}"
  rm -f "${go_pkg_dir}"

  # TODO: This symlink should be relative.
  if [[ "$OSTYPE" == "cygwin" ]]; then
    S2I_ROOT_cyg=$(cygpath -w ${S2I_ROOT})
    go_pkg_dir_cyg=$(cygpath -w ${go_pkg_dir})
    cmd /c "mklink ${go_pkg_dir_cyg} ${S2I_ROOT_cyg}" &>/dev/null
  else
    ln -s "${S2I_ROOT}" "${go_pkg_dir}"
  fi
}


# s2i::build::setup_env will check that the `go` commands is available in
# ${PATH}. If not running on Travis, it will also check that the Go version is
# good enough for the Kubernetes build.
#
# Input Vars:
#   S2I_EXTRA_GOPATH - If set, this is included in created GOPATH
#   S2I_NO_GODEPS - If set, we don't add 'vendor' to GOPATH
#
# Output Vars:
#   export GOPATH - A modified GOPATH to our created tree along with extra
#     stuff.
#   export GOBIN - This is actively unset if already set as we want binaries
#     placed in a predictable place.
s2i::build::setup_env() {
  s2i::build::create_gopath_tree

  if [[ -z "$(which go)" ]]; then
    cat <<EOF

Can't find 'go' in PATH, please fix and retry.
See http://golang.org/doc/install for installation instructions.

EOF
    exit 2
  fi

  # For any tools that expect this to be set (it is default in golang 1.6),
  # force vendor experiment.
  export GO15VENDOREXPERIMENT=1

  GOPATH=${S2I_GOPATH}

  # Append S2I_EXTRA_GOPATH to the GOPATH if it is defined.
  if [[ -n ${S2I_EXTRA_GOPATH:-} ]]; then
    GOPATH="${GOPATH}:${S2I_EXTRA_GOPATH}"
  fi

  # Append the tree maintained by `godep` to the GOPATH unless S2I_NO_GODEPS
  # is defined.
  if [[ -z ${S2I_NO_GODEPS:-} ]]; then
    GOPATH="${GOPATH}:${S2I_ROOT}/vendor"
  fi

  if [[ "$OSTYPE" == "cygwin" ]]; then
    GOPATH=$(cygpath -w -p $GOPATH)
  fi

  export GOPATH

  # Unset GOBIN in case it already exists in the current session.
  unset GOBIN
}

# This will take binaries from $GOPATH/bin and copy them to the appropriate
# place in ${S2I_OUTPUT_BINDIR}
#
# If S2I_RELEASE_ARCHIVE is set to a directory, it will have tar archives of
# each S2I_RELEASE_PLATFORMS created
#
# Ideally this wouldn't be necessary and we could just set GOBIN to
# S2I_OUTPUT_BINDIR but that won't work in the face of cross compilation.  'go
# install' will place binaries that match the host platform directly in $GOBIN
# while placing cross compiled binaries into `platform_arch` subdirs.  This
# complicates pretty much everything else we do around packaging and such.
s2i::build::place_bins() {
  (
    local host_platform
    host_platform=$(s2i::build::host_platform)

    echo "++ Placing binaries"

    if [[ "${S2I_RELEASE_ARCHIVE-}" != "" ]]; then
      s2i::build::get_version_vars
      mkdir -p "${S2I_LOCAL_RELEASEPATH}"
    fi

    s2i::build::export_targets "$@"

    for platform in "${platforms[@]}"; do
      # The substitution on platform_src below will replace all slashes with
      # underscores.  It'll transform darwin/amd64 -> darwin_amd64.
      local platform_src="/${platform//\//_}"
      if [[ $platform == $host_platform ]]; then
        platform_src=""
      fi

      # Skip this directory if the platform has no binaries.
      local full_binpath_src="${S2I_GOPATH}/bin${platform_src}"
      if [[ ! -d "${full_binpath_src}" ]]; then
        continue
      fi

      mkdir -p "${S2I_OUTPUT_BINPATH}/${platform}"

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

      # Move the specified release binaries to the shared S2I_OUTPUT_BINPATH.
      for binary in "${binaries[@]}"; do
        mv "${full_binpath_src}/${binary}" "${S2I_OUTPUT_BINPATH}/${platform}/"
      done

      # If no release archive was requested, we're done.
      if [[ "${S2I_RELEASE_ARCHIVE-}" == "" ]]; then
        continue
      fi

      # Create a temporary bin directory containing only the binaries marked for release.
      local release_binpath=$(mktemp -d sti.release.${S2I_RELEASE_ARCHIVE}.XXX)
      for binary in "${binaries[@]}"; do
        cp "${S2I_OUTPUT_BINPATH}/${platform}/${binary}" "${release_binpath}/"
      done

      # Create binary copies where specified.
      local suffix=""
      if [[ $platform == "windows/amd64" ]]; then
        suffix=".exe"
      fi
      for linkname in "${S2I_BINARY_SYMLINKS[@]}"; do
        local src="${release_binpath}/s2i${suffix}"
        if [[ -f "${src}" ]]; then
          ln -s "s2i${suffix}" "${release_binpath}/${linkname}${suffix}"
        fi
      done

      # Create the release archive.
      local platform_segment="${platform//\//-}"
      if [[ $platform == "windows/amd64" ]]; then
        local archive_name="${S2I_RELEASE_ARCHIVE}-${S2I_GIT_VERSION}-${S2I_GIT_COMMIT}-${platform_segment}.zip"
        echo "++ Creating ${archive_name}"
        for file in "${S2I_BINARY_RELEASE_WINDOWS[@]}"; do
          zip "${S2I_LOCAL_RELEASEPATH}/${archive_name}" -qj "${release_binpath}/${file}"
        done
      else
        local archive_name="${S2I_RELEASE_ARCHIVE}-${S2I_GIT_VERSION}-${S2I_GIT_COMMIT}-${platform_segment}.tar.gz"
        echo "++ Creating ${archive_name}"
        tar -czf "${S2I_LOCAL_RELEASEPATH}/${archive_name}" -C "${release_binpath}" .
      fi
      rm -rf "${release_binpath}"
    done
  )
}

# s2i::build::make_binary_symlinks makes symlinks for the sti
# binary in _output/local/go/bin
s2i::build::make_binary_symlinks() {
  platform=$(s2i::build::host_platform)
  if [[ -f "${S2I_OUTPUT_BINPATH}/${platform}/s2i" ]]; then
    for linkname in "${S2I_BINARY_SYMLINKS[@]}"; do
      if [[ $platform == "windows/amd64" ]]; then
        cp "${S2I_OUTPUT_BINPATH}/${platform}/s2i.exe" "${S2I_OUTPUT_BINPATH}/${platform}/${linkname}.exe"
      else
        ln -sf s2i "${S2I_OUTPUT_BINPATH}/${platform}/${linkname}"
      fi
    done
  fi
}

# s2i::build::detect_local_release_tars verifies there is only one primary and one
# image binaries release tar in S2I_LOCAL_RELEASEPATH for the given platform specified by
# argument 1, exiting if more than one of either is found.
#
# If the tars are discovered, their full paths are exported to the following env vars:
#
#   S2I_PRIMARY_RELEASE_TAR
s2i::build::detect_local_release_tars() {
  local platform="$1"

  if [[ ! -d "${S2I_LOCAL_RELEASEPATH}" ]]; then
    echo "There are no release artifacts in ${S2I_LOCAL_RELEASEPATH}"
    exit 2
  fi
  if [[ ! -f "${S2I_LOCAL_RELEASEPATH}/.commit" ]]; then
    echo "There is no release .commit identifier ${S2I_LOCAL_RELEASEPATH}"
    exit 2
  fi
  local primary=$(find ${S2I_LOCAL_RELEASEPATH} -maxdepth 1 -type f -name source-to-image-*-${platform}*)
  if [[ $(echo "${primary}" | wc -l) -ne 1 ]]; then
    echo "There should be exactly one ${platform} primary tar in $S2I_LOCAL_RELEASEPATH"
    exit 2
  fi

  export S2I_PRIMARY_RELEASE_TAR="${primary}"
  export S2I_RELEASE_COMMIT="$(cat ${S2I_LOCAL_RELEASEPATH}/.commit)"
}


# s2i::build::get_version_vars loads the standard version variables as
# ENV vars
s2i::build::get_version_vars() {
  if [[ -n ${S2I_VERSION_FILE-} ]]; then
    source "${S2I_VERSION_FILE}"
    return
  fi
  s2i::build::sti_version_vars
}

# s2i::build::sti_version_vars looks up the current Git vars
s2i::build::sti_version_vars() {
  local git=(git --work-tree "${S2I_ROOT}")

  if [[ -n ${S2I_GIT_COMMIT-} ]] || S2I_GIT_COMMIT=$("${git[@]}" rev-parse --short "HEAD^{commit}" 2>/dev/null); then
    if [[ -z ${S2I_GIT_TREE_STATE-} ]]; then
      # Check if the tree is dirty.  default to dirty
      if git_status=$("${git[@]}" status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
        S2I_GIT_TREE_STATE="clean"
      else
        S2I_GIT_TREE_STATE="dirty"
      fi
    fi

    # Use git describe to find the version based on annotated tags.
    if [[ -n ${S2I_GIT_VERSION-} ]] || S2I_GIT_VERSION=$("${git[@]}" describe --tags "${S2I_GIT_COMMIT}^{commit}" 2>/dev/null); then
      if [[ "${S2I_GIT_TREE_STATE}" == "dirty" ]]; then
        # git describe --dirty only considers changes to existing files, but
        # that is problematic since new untracked .go files affect the build,
        # so use our idea of "dirty" from git status instead.
        S2I_GIT_VERSION+="-dirty"
      fi

      # Try to match the "git describe" output to a regex to try to extract
      # the "major" and "minor" versions and whether this is the exact tagged
      # version or whether the tree is between two tagged versions.
      if [[ "${S2I_GIT_VERSION}" =~ ^v([0-9]+)\.([0-9]+)([.-].*)?$ ]]; then
        S2I_GIT_MAJOR=${BASH_REMATCH[1]}
        S2I_GIT_MINOR=${BASH_REMATCH[2]}
        if [[ -n "${BASH_REMATCH[3]}" ]]; then
          S2I_GIT_MINOR+="+"
        fi
      fi
    fi
  fi
}

# Saves the environment flags to $1
s2i::build::save_version_vars() {
  local version_file=${1-}
  [[ -n ${version_file} ]] || {
    echo "!!! Internal error.  No file specified in s2i::build::save_version_vars"
    return 1
  }

  cat <<EOF >"${version_file}"
S2I_GIT_COMMIT='${S2I_GIT_COMMIT-}'
S2I_GIT_TREE_STATE='${S2I_GIT_TREE_STATE-}'
S2I_GIT_VERSION='${S2I_GIT_VERSION-}'
S2I_GIT_MAJOR='${S2I_GIT_MAJOR-}'
S2I_GIT_MINOR='${S2I_GIT_MINOR-}'
EOF
}

# golang 1.5 wants `-X key=val`, but golang 1.4- REQUIRES `-X key val`
s2i::build::ldflag() {
  local key=${1}
  local val=${2}

  GO_VERSION=($(go version))
  if [[ -n $(echo "${GO_VERSION[2]}" | grep -E 'go1.4') ]]; then
    echo "-X ${S2I_GO_PACKAGE}/pkg/version.${key} ${val}"
  else
    echo "-X ${S2I_GO_PACKAGE}/pkg/version.${key}=${val}"
  fi
}

# s2i::build::ldflags calculates the -ldflags argument for building STI
s2i::build::ldflags() {
  (
    # Run this in a subshell to prevent settings/variables from leaking.
    set -o errexit
    set -o nounset
    set -o pipefail

    cd "${S2I_ROOT}"

    s2i::build::get_version_vars

    declare -a ldflags=()
    ldflags+=($(s2i::build::ldflag "majorFromGit" "${S2I_GIT_MAJOR}"))
    ldflags+=($(s2i::build::ldflag "minorFromGit" "${S2I_GIT_MINOR}"))
    ldflags+=($(s2i::build::ldflag "versionFromGit" "${S2I_GIT_VERSION}"))
    ldflags+=($(s2i::build::ldflag "commitFromGit" "${S2I_GIT_COMMIT}"))
    # The -ldflags parameter takes a single string, so join the output.
    echo "${ldflags[*]-}"
  )
}
