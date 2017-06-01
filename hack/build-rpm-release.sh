#!/bin/bash

# This script generates release zips and RPMs into _output/releases.
# tito and other build dependencies are required on the host. We will
# be running `hack/build-cross.sh` under the covers, so we transitively
# consume all of the relevant envars.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::util::ensure::system_binary_exists tito
os::util::ensure::system_binary_exists createrepo
os::build::setup_env

if [[ "${OS_ONLY_BUILD_PLATFORMS:-}" == 'linux/amd64' ]]; then
	# when the user is asking for only Linux binaries, we will
	# furthermore not build cross-platform clients in tito
	make_redistributable=0
else
	make_redistributable=1
fi

os::log::info 'Building Origin release RPMs with tito...'
os::build::rpm::get_nvra_vars
tito tag --use-version="${OS_RPM_VERSION}" \
         --use-release="${OS_RPM_RELEASE}" \
         --no-auto-changelog --offline
tito_tmp_dir="${BASETMPDIR}/tito"
mkdir -p "${tito_tmp_dir}"
tito build --offline --srpm --rpmbuild-options="--define 'dist .el7'" --output="${tito_tmp_dir}"
tito build --output="${tito_tmp_dir}" --rpm --no-cleanup --quiet --offline \
           --rpmbuild-options="--define 'make_redistributable ${make_redistributable}' ${RPM_BUILD_OPTS:-}"
tito tag --undo --offline

os::log::info 'Unpacking tito artifacts for reuse...'
output_directories=( $( find "${tito_tmp_dir}" -type d -name "rpmbuild-${OS_RPM_NAME}*" ) )
if [[ "${#output_directories[@]}" -eq 0 ]]; then
	os::log::fatal 'After the tito build, no rpmbuild directory was found!'
elif [[ "${#output_directories[@]}" -gt 1 ]]; then
	# find the newest directory in the list
	output_directory="${output_directories[0]}"
	for directory in "${output_directories[@]}"; do
		if [[ "${directory}" -nt "${output_directory}" ]]; then
			output_directory="${directory}"
		fi
	done
	os::log::warning "After the tito build, more than one rpmbuild directory was found!
This script will unpack the most recently modified directory: ${output_directory}"
else
        output_directory="${output_directories[0]}"
fi

tito_output_directory="$( find "${output_directory}" -type d -path "*/BUILD/${OS_RPM_NAME}-${OS_RPM_VERSION}/_output/local" )"
if [[ -z "${tito_output_directory}" ]]; then
        os::log::fatal 'No _output artifact directory found in tito rpmbuild artifacts!'
fi

# clean up our local state so we can unpack the tito artifacts cleanly
make clean

# migrate the tito artifacts to the Origin directory
mkdir -p "${OS_OUTPUT}"
# mv exits prematurely with status 1 in the following scenario: running as root,
# attempting to move a [directory tree containing a] symlink to a destination on
# an NFS volume exported with root_squash set.  This can occur when running this
# script on a Vagrant box.  The error shown is "mv: failed to preserve ownership
# for $FILE: Operation not permitted".  As a workaround, if
# ${tito_output_directory} and ${OS_OUTPUT} are on different devices, use cp and
# rm instead.
if [[ $(stat -c %d "${tito_output_directory}") == $(stat -c %d "${OS_OUTPUT}") ]]; then
  mv "${tito_output_directory}"/* "${OS_OUTPUT}"
else
  cp -R "${tito_output_directory}"/* "${OS_OUTPUT}"
  rm -rf "${tito_output_directory}"/*
fi
mkdir -p "${OS_OUTPUT_RPMPATH}"
mv "${tito_tmp_dir}"/*src.rpm "${OS_OUTPUT_RPMPATH}"
mv "${tito_tmp_dir}"/*/*.rpm "${OS_OUTPUT_RPMPATH}"

repo_path="$( os::util::absolute_path "${OS_OUTPUT_RPMPATH}" )"
createrepo "${repo_path}"

echo "[${OS_RPM_NAME}-local-release]
baseurl = file://${repo_path}
gpgcheck = 0
name = OpenShift Release from Local Source
enabled = 1
" > "${repo_path}/${OS_RPM_NAME}-local-release.repo"

os::log::info "Repository file for \`yum\` or \`dnf\` placed at ${repo_path}/origin-local-release.repo
Install it with:
$ mv '${repo_path}/origin-local-release.repo' '/etc/yum.repos.d"
