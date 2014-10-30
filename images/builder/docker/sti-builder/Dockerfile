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
# The standard name for this image is openshift/origin-sti-builder
#
FROM openshift/origin-base

# TODO: When STI is vendored in OpenShift, it should be built as part of openshift/origin
# and simply made the CMD here.
RUN yum -y install golang golang-src golang-pkg-bin-linux-amd64 golang-pkg-linux-amd64 && \
    yum clean all && \
    mkdir -p /tmp/go/src/github.com/openshift && \
    git clone git://github.com/openshift/geard /tmp/go/src/github.com/openshift/geard && \
    export GOPATH=/tmp/go && \
    cd /tmp/go/src/github.com/openshift/geard && \
    ./contrib/build -n && \
    cp /tmp/go/bin/sti /usr/bin/sti

ENV HOME /root
ADD ./build.sh /opt/build.sh
CMD ["/opt/build.sh"]
