# This image is intended for testing purposes, it has the same behavior as
# the origin-docker-builder image, but does so as a custom image so it can
# be used with Custom build strategies.  It expects a set of
# environment variables to parameterize the build:
#
#   OUTPUT_REGISTRY - the Docker registry URL to push this image to
#   OUTPUT_IMAGE - the name to tag the image with
#   SOURCE_URI - a URI to fetch the build context from
#   SOURCE_REF - a reference to pass to Git for which commit to use (optional)
#
# This image expects to have the Docker socket bind-mounted into the container.
# If "/root/.dockercfg" is bind mounted in, it will use that as authorization
# to a Docker registry.
#
# The standard name for this image is openshift/origin-custom-docker-builder
#
FROM openshift/origin-base

RUN INSTALL_PKGS="gettext automake make docker" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

LABEL io.k8s.display-name="OpenShift Origin Custom Builder Example" \
      io.k8s.description="This is an example of a custom builder for use with OpenShift Origin."
ENV HOME=/root
COPY build.sh /tmp/build.sh
CMD ["/tmp/build.sh"]
