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
os::build::get_version_vars
if [[ "${OS_GIT_TREE_STATE}" == "dirty" ]]; then
	os::log::fatal "Cannot build RPMs with a dirty git tree. Commit your changes and try again."
fi
if [[ "${OS_GIT_VERSION}" =~ ^v([0-9](\.[0-9]+)*)(.*) ]]; then
	# we need to translate from the semantic version
	# provided by the Origin build scripts to the
	# version that RPM will expect.
	rpm_version="${BASH_REMATCH[1]}"
	rpm_release="0${BASH_REMATCH[3]//-/.}"
fi
tito tag --use-version="${rpm_version}" \
         --use-release="${rpm_release}" \
         --no-auto-changelog --offline
tito_tmp_dir="${BASETMPDIR}/tito"
mkdir -p "${tito_tmp_dir}"
tito build --output="${tito_tmp_dir}" --rpm --no-cleanup --quiet --offline \
           --rpmbuild-options="--define 'make_redistributable ${make_redistributable}'"
tito tag --undo --offline

os::log::info 'Unpacking tito artifacts for reuse...'
output_directories=( $( find "${tito_tmp_dir}" -type d -name 'rpmbuild-origin*' ) )
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
	os::log::warn 'After the tito build, more than one rpmbuild directory was found!'
	os::log::warn 'This script will unpack the most recently modified directory: '"${output_directory}"
else
	output_directory="${output_directories[0]}"
fi

tito_output_directory="$( find "${output_directory}" -type d -path "*/BUILD/origin-${rpm_version}/_output/local" )"
if [[ -z "${tito_output_directory}" ]]; then
        os::log::fatal 'No _output artifact directory found in tito rpmbuild artifacts!'
fi

# clean up our local state so we can unpack the tito artifacts cleanly
make clean

# migrate the tito artifacts to the Origin directory
mkdir -p "${OS_OUTPUT}"
mv "${tito_output_directory}"/* "${OS_OUTPUT}"
mkdir -p "${OS_LOCAL_RELEASEPATH}/rpms"
mv "${tito_tmp_dir}"/x86_64/*.rpm "${OS_LOCAL_RELEASEPATH}/rpms"

if command -v createrepo >/dev/null 2>&1; then
	repo_path="$( os::util::absolute_path "${OS_LOCAL_RELEASEPATH}/rpms" )"
	createrepo "${repo_path}"

	echo "[origin-local-release]
baseurl = file://${repo_path}
gpgcheck = 0
name = OpenShift Origin Release from Local Source
" > "${repo_path}/origin-local-release.repo"

	os::log::info "Repository file for \`yum\` or \`dnf\` placed at ${repo_path}/origin-local-release.repo"
	os::log::info "Install it with: "$'\n\t'"$ mv '${repo_path}/origin-local-release.repo' '/etc/yum.repos.d"
else
	os::log::warn "Repository file for \`yum\` or \`dnf\` could not be generated, install \`createrepo\`."
fi
