#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::build::setup_env
os::util::ensure::built_binary_exists 'prepare-fork-repo' 'tools/prepare-fork-repo'

# ORIGIN_COMMIT must be set to a hash in openshift/origin repository that will be used
# as a starting point for synchronization. Usually this refers to a Kubernetes rebase commit.
# Every commit that modify files in vendor/ directories listed below will be then synchronized
# into fork repo.
commit="${ORIGIN_COMMIT}"

# NOTE: This should be updated during release process.
# NOTE: Add new repositories from glide.yaml
# Syntax: <repo>#<base-branch>#<new-branch>
#
# Where the 'repo' is github.com/openshift/<repo>.
# The 'base-branch' is a branch name used as a base for the new branch.
# If base and new branch are the same, no branch will be created, but commit
# will be added to the branch.
repos=(
    kubernetes-gengo#openshift-3.9#openshift-3.9
    google-certificate-transparency#master#openshift-3.9
    containers-image#openshift-3.8#openshift-3.9
    opencontainers-runc#openshift-3.9#openshift.3.9
    # TODO: We should probably use openshift versioning here:
    emicklei-go-restful-swagger12#release-1.0.1#release-1.0.1
    google-cadvisor#release-v0.28.3#release-v0.28.3
    docker-distribution#release-2.6.0#release-2.6.0
    onsi-ginkgo#release-v1.2.0#release-v1.2.0
    cloudflare-cfssl#stable-20160905#stable-20160905
    skynetservices-skydns#release-2.5.3a#release-2.5.3a
)

for repo in "${repos[@]}"; do
    parts=(${repo//#/ })
    prepare-fork-repo -repo="${parts[0]}" \
                      -base-branch="${parts[1]}" \
                      -branch="${parts[2]}" \
                      -commit="Origin-commit: ${commit}"
done
