#
# This is the integrated OpenShift Origin Docker registry. It is configured to
# publish metadata to OpenShift to provide automatic management of images on push.
#
# The standard name for this image is openshift/origin-docker-registry
#
FROM openshift/origin-base

RUN yum install -y tree findutils epel-release && \
    yum clean all

# The registry doesn't require a privileged user.
USER 1001

EXPOSE 5000
VOLUME /registry
ENV REGISTRY_CONFIGURATION_PATH /config.yml
CMD DOCKER_REGISTRY_URL=${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT} /dockerregistry ${REGISTRY_CONFIGURATION_PATH}

ADD config.yml $REGISTRY_CONFIGURATION_PATH
ADD bin/dockerregistry /dockerregistry
