#!/bin/bash

# This script generates release zips and RPMs into _output/releases.
# tito and other build dependencies are required on the host. We will
# be running `hack/build-cross.sh` under the covers, so we transitively
# consume all of the relevant envars.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::build::setup_env
os::util::environment::setup_tmpdir_vars "build-rpm-release"

if [[ "${OS_ONLY_BUILD_PLATFORMS:-}" == 'linux/amd64' ]]; then
	# when the user is asking for only Linux binaries, we will
	# furthermore not build cross-platform clients in tito
	make_redistributable=0
else
	make_redistributable=1
fi

os::log::info 'Building Origin release RPMs with tito...'
tito_tmp_dir="${BASETMPDIR}/tito"
mkdir -p "${tito_tmp_dir}"
tito build --output="${tito_tmp_dir}" --rpm --test --no-cleanup \
           --rpmbuild-options="--define 'make_redistributable ${make_redistributable}'"

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

if ! tito_output_directory="$( find "${output_directory}" -type d -path '*/BUILD/origin-git-*/_output/local' )"; then
	os::log::error 'No _output artifact directory found in tito rpmbuild artifacts!'
	exit 1
fi

# clean up our local state so we can unpack the tito artifacts cleanly
make clean

# migrate the tito artifacts to the Origin directory
mkdir -p "${OS_OUTPUT}"
cp -r "${tito_output_directory}"/* "${OS_OUTPUT}"
mkdir -p "${OS_LOCAL_RELEASEPATH}"
cp "${tito_tmp_dir}"/x86_64/*.rpm "${OS_LOCAL_RELEASEPATH}"