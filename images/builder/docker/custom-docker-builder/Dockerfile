#
# This is the image that executes a Docker build inside Origin. It expects a set of
# environment variables to parameterize the build:
#
#   OUTPUT_REGISTRY - the Docker registry URL to push this image to
#   OUTPUT_IMAGE - the name to tag the image with
#   SOURCE_URI - a URI to fetch the build context from
#   SOURCE_REF - a reference to pass to Git for which commit to use (optional)
#
# This image expects to have the Docker socket bind-mounted into the container.
# If "/root/.dockercfg" is bind mounted in, it will use that as authorization to a
# Docker registry.
#
# The standard name for this image is openshift/origin-custom-docker-builder
#
FROM centos:centos7

RUN yum install -y --enablerepo=centosplus epel-release gettext tar automake make git docker

ENV HOME /root
ADD ./build.sh /tmp/build.sh
CMD ["/tmp/build.sh"]
