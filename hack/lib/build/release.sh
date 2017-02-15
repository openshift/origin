#!/bin/bash

# This library holds utility functions for building releases.

# os::build::release::check_for_rpms checks that an RPM release has been built
function os::build::release::check_for_rpms() {
	if [[ ! -d "${OS_LOCAL_RPMPATH}" || ! -s "${OS_LOCAL_RELEASEPATH}/CHECKSUM" ]]; then
		relative_rpmpath="$( os::util::repository_relative_path "${OS_LOCAL_RPMPATH}" )"
		relative_binpath="$( os::util::repository_relative_path "${OS_OUTPUT_BINPATH}" )"
		os::log::fatal "No release RPMs have been built! RPMs are necessary to build container images.
Build them with:
  $ OS_BUILD_ENV_PRESERVE=${relative_binpath}:${relative_rpmpath} hack/env make build-rpms"
	fi
}