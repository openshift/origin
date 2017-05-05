#!/bin/bash

# This script generates release zips and RPMs into _output/releases.
# tito and other build dependencies are required on the host. We will
# be running `hack/build-cross.sh` under the covers, so we transitively
# consume all of the relevant envars. We also consume:
#  - BUILD_TESTS: whether or not to build a test RPM, off by default
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
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
        os::log::error 'After the tito build, no rpmbuild directory was found!'
        exit 1
elif [[ "${#output_directories[@]}" -gt 1 ]]; then
        # find the newest directory in the list
        output_directory="${output_directories[0]}"
        for directory in "${output_directories[@]}"; do
                if [[ "${directory}" -nt "${output_directory}" ]]; then
                        output_directory="${directory}"
                fi
        done
        os::log::warning 'After the tito build, more than one rpmbuild directory was found!'
        os::log::warning 'This script will unpack the most recently modified directory: '"${output_directory}"
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
mv "${tito_output_directory}"/* "${OS_OUTPUT}"
mkdir -p "${OS_LOCAL_RELEASEPATH}/rpms"
mv "${tito_tmp_dir}"/*src.rpm "${OS_LOCAL_RELEASEPATH}/rpms"
mv "${tito_tmp_dir}"/*/*.rpm "${OS_LOCAL_RELEASEPATH}/rpms"

if command -v createrepo >/dev/null 2>&1; then
	repo_path="$( os::util::absolute_path "${OS_LOCAL_RELEASEPATH}/rpms" )"
	createrepo "${repo_path}"

	echo "[${OS_RPM_NAME}-local-release]
baseurl = file://${repo_path}
gpgcheck = 0
name = OpenShift Release from Local Source
enabled = 1
" > "${repo_path}/${OS_RPM_NAME}-local-release.repo"

	os::log::info "Repository file for \`yum\` or \`dnf\` placed at ${repo_path}/${OS_RPM_NAME}-local-release.repo"
	os::log::info "Install it with: "$'\n\t'"$ mv '${repo_path}/${OS_RPM_NAME}-local-release.repo' '/etc/yum.repos.d"
else
	os::log::warning "Repository file for \`yum\` or \`dnf\` could not be generated, install \`createrepo\`."
fi
