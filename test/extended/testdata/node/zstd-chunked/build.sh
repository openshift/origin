#!/bin/sh

set -euo pipefail

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
buildah --authfile /tmp/.push --storage-driver vfs push --compression-format zstd:chunked ${TAG}
