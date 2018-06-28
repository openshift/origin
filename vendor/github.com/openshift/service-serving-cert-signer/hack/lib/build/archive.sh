#!/bin/bash

# This library holds utility functions for archiving
# built binaries and releases.

function os::build::archive::name() {
  echo "${OS_RELEASE_ARCHIVE}-${OS_GIT_VERSION}-$1" | tr '+' '-'
}
readonly -f os::build::archive::name

function os::build::archive::zip() {
  local default_name
  default_name="$( os::build::archive::name "${platform}" ).zip"
  local archive_name="${archive_name:-$default_name}"
  echo "++ Creating ${archive_name}"
  for file in "$@"; do
    pushd "${release_binpath}" &> /dev/null
      sha256sum "${file}"
    popd &>/dev/null
    zip "${OS_OUTPUT_RELEASEPATH}/${archive_name}" -qj "${release_binpath}/${file}"
  done
}
readonly -f os::build::archive::zip

function os::build::archive::tar() {
  local base_name
  base_name="$( os::build::archive::name "${platform}" )"
  local default_name="${base_name}.tar.gz"
  local archive_name="${archive_name:-$default_name}"
  echo "++ Creating ${archive_name}"
  pushd "${release_binpath}" &> /dev/null
  find . -type f -exec sha256sum {} \;
  if [[ -n "$(which bsdtar)" ]]; then
    bsdtar -czf "${OS_OUTPUT_RELEASEPATH}/${archive_name}" -s ",^\.,${base_name}," $@
  else
    tar -czf --xattrs-exclude='LIBARCHIVE.xattr.security.selinux' "${OS_OUTPUT_RELEASEPATH}/${archive_name}" --transform="s,^\.,${base_name}," $@
  fi
  popd &>/dev/null
}
readonly -f os::build::archive::tar

# Checks if the filesystem on a partition that the provided path points to is
# supporting hard links.
#
# Input:
#  $1 - the path where the hardlinks support test will be done.
# Returns:
#  0 - if hardlinks are supported
#  non-zero - if hardlinks aren't supported
function os::build::archive::internal::is_hardlink_supported() {
  local path="$1"
  # Determine if FS supports hard links
  local temp_file=$(TMPDIR="${path}" mktemp)
  ln "${temp_file}" "${temp_file}.link" &> /dev/null && unlink "${temp_file}.link" || local supported=$?
  rm -f "${temp_file}"
  return ${supported:-0}
}
readonly -f os::build::archive::internal::is_hardlink_supported

# Extract a tar.gz compressed archive in a given directory. If the
# archive contains hardlinks and the underlying filesystem is not
# supporting hardlinks then the a hard dereference will be done.
#
# Input:
#   $1 - path to archive file
#   $2 - directory where the archive will be extracted
function os::build::archive::extract_tar() {
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
  if os::build::archive::internal::is_hardlink_supported "${change_dir}" ; then
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
readonly -f os::build::archive::extract_tar
