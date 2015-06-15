#
# This is the image that executes a Docker build inside Origin. It expects the
# following environment variables:
#
#   BUILD - JSON string containing the openshift build object
#
# This image expects to have the Docker socket bind-mounted into the container.
# If "/root/.dockercfg" is bind mounted in, it will use that as authorization to a
# Docker registry.
#
# The standard name for this image is openshift3/ose-docker-builder
#
FROM openshift3/ose

ENV HOME /root
ENTRYPOINT ["/usr/bin/openshift-docker-build"]
