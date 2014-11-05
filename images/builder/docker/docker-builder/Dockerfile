#
# This is the image that executes a Docker build inside Origin. It expects a set of
# environment variables to parameterize the build:
#
#   BUILD_TAG - the tag to assign the image after it is built
#   REGISTRY - the Docker registry URL to push this image to (optional)
#   SOURCE_URI - a URI to fetch the build context from
#   SOURCE_REF - a reference to pass to Git for which commit to use (optional)
#   CONTEXT_DIR - a subdirectory of the retrieved source to run the build from
#
# This image expects to have the Docker socket bind-mounted into the container.
# If "/root/.dockercfg" is bind mounted in, it will use that as authorization to a
# Docker registry.
#
# The standard name for this image is openshift/origin-docker-builder
#
FROM openshift/origin-base

ENV HOME /root
ADD ./build.sh /tmp/build.sh
CMD ["/tmp/build.sh"]
