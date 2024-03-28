#!/bin/bash

set -euo pipefail

# Figure out if we're in a user namespace with non-default ID mappings, and if
# so, output the same diagnostic that the docker-builder does when its log
# level is 2 or higher.
readidmap() {
	local idmap
	while read host container size ; do
		idmap="${idmap:+${idmap},}(${host}:${container}:${size})"
	done
	echo ["$idmap"]
}

UIDMAP=$(readidmap < /proc/self/uid_map)
GIDMAP=$(readidmap < /proc/self/gid_map)
if test "${BUILD_LOGLEVEL:-0}" -ge 2; then
	if test "$UIDMAP" != '[(0:0:4294967295)]' || test "$GIDMAP" != '[(0:0:4294967295)]' ; then
		echo Started in kernel user namespace as $(id -u):$(id -g) with UID map "$UIDMAP" and GID map "$GIDMAP".
	else
		echo "Started in node (default) kernel user namespace as $(id -u):$(id -g)."
	fi
fi

# Note that in this case the build inputs are part of the custom builder image, but normally this
# would be retrieved from an external source.
cd /tmp/input
# OUTPUT_REGISTRY and OUTPUT_IMAGE are env variables provided by the custom
# build framework
TAG="${OUTPUT_REGISTRY}/${OUTPUT_IMAGE}"

cp -R /var/run/configs/openshift.io/certs/certs.d/* /etc/containers/certs.d/

# buildah requires a slight modification to the push secret provided by the service account in order to use it for pushing the image
echo "{ \"auths\": $(cat /var/run/secrets/openshift.io/pull/.dockercfg)}" > /tmp/.pull
echo "{ \"auths\": $(cat /var/run/secrets/openshift.io/push/.dockercfg)}" > /tmp/.push

# performs the build of the new image defined by Dockerfile.sample
buildah --authfile /tmp/.pull --storage-driver vfs bud --isolation chroot -t ${TAG} .
# push the new image to the target for the build
buildah --authfile /tmp/.push --storage-driver vfs push ${TAG}

