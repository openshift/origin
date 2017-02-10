#!/bin/bash

# This script sets up a go workspace locally and generates the documents and manuals.

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::built_binary_exists 'gendocs'
os::util::ensure::built_binary_exists 'genman'

OUTPUT_DIR_REL=${1:-""}
OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/generated"
MAN_OUTPUT_DIR="${OS_ROOT}/${OUTPUT_DIR_REL}/docs/man/man1"

mkdir -p "${OUTPUT_DIR}" || echo $? > /dev/null
mkdir -p "${MAN_OUTPUT_DIR}" || echo $? > /dev/null

function generate_manual_pages() {
  local dest="$1"
  local cmdName="$2"
  local filestore=".files_generated_${cmdName}"
  local skipprefix="${3:-}"

  # We do this in a tmpdir in case the dest has other non-autogenned files
  # We don't want to include them in the list of gen'd files
  local tmpdir="${OS_ROOT}/_tmp/gen_man"
  mkdir -p "${tmpdir}"
  # generate the new files
  genman "${tmpdir}" "${cmdName}"
  # create the list of generated files
  ls "${tmpdir}" | LC_ALL=C sort > "${tmpdir}/${filestore}"

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
  done <"${dest}/${filestore}"

  # put the new generated file into the destination
  find "${tmpdir}" -exec rsync -pt {} "${dest}" \; >/dev/null
  #cleanup
  rm -rf "${tmpdir}"

  echo "Assets generated in ${dest}"
}
readonly -f generate_manual_pages

function generate_documentation() {
  local dest="$1"
  local skipprefix="${1:-}"

  # We do this in a tmpdir in case the dest has other non-autogenned files
  # We don't want to include them in the list of gen'd files
  local tmpdir="${OS_ROOT}/_tmp/gen_doc"
  mkdir -p "${tmpdir}"
  # generate the new files
  gendocs "${tmpdir}"
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
readonly -f generate_documentation

generate_documentation "${OUTPUT_DIR}"
generate_manual_pages "${MAN_OUTPUT_DIR}" "oc"
generate_manual_pages "${MAN_OUTPUT_DIR}" "openshift"
generate_manual_pages "${MAN_OUTPUT_DIR}" "oadm"