#!/bin/sh

set -euo pipefail

# Note that in this case we're looking for a config map as the main build
# source. The build controller mounts config maps by name under
# /var/run/configs/openshift.io/build, and the builder image decides how to use
# the corresponding DestinationDir values from the Build object encoded in
# $BUILD, if it consults them at all.
#
# A Docker or Source builder would clone the git repository named in the
# $SOURCE_REPOSITORY env variable, copy the config map contents into a
# subdirectory of the cloned source tree (or a context subdirectory of it)
# named after the config map, and proceed from there.
#
# We'll just use the map contents from where the build controller put them.
cd /var/run/configs/openshift.io/build/custom-configmap

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
