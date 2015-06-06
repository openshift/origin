#
# This is the integrated OpenShift Origin Docker registry. It is configured to
# publish metadata to OpenShift to provide automatic management of images on push.
#
# The standard name for this image is openshift/origin-docker-registry
#
FROM openshift/origin-base

ADD config.yml /config.yml
ADD bin/dockerregistry /dockerregistry

ENV REGISTRY_CONFIGURATION_PATH=/config.yml

RUN yum install -y tree findutils epel-release && \
    yum clean all

EXPOSE 5000
VOLUME /registry
CMD REGISTRY_URL=${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT} /dockerregistry /config.yml
