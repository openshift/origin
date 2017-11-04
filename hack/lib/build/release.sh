#!/bin/bash

# This library holds utility functions for building releases.

# os::build::release::check_for_rpms checks that an RPM release has been built
function os::build::release::check_for_rpms() {
	if [[ ! -d "${OS_OUTPUT_RPMPATH}" || ! -d "${OS_OUTPUT_RPMPATH}/repodata" ]]; then
		relative_release_path="$( os::util::repository_relative_path "${OS_OUTPUT_RELEASEPATH}" )"
		relative_bin_path="$( os::util::repository_relative_path "${OS_OUTPUT_BINPATH}" )"
		os::log::fatal "No release RPMs have been built! RPMs are necessary to build container images.
Build them with:
  $ OS_BUILD_ENV_PRESERVE=${relative_bin_path}:${relative_release_path} hack/env make build-rpms"
	fi
}
