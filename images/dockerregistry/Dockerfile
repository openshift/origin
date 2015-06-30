#
# This is the integrated OpenShift Origin Docker registry. It is configured to
# publish metadata to OpenShift to provide automatic management of images on push.
#
# The standard name for this image is openshift3/ose-docker-registry
#
FROM openshift3/ose-base

RUN yum install -y openshift-dockerregistry && \
    yum clean all

ENV REGISTRY_CONFIGURATION_PATH=/config.yml

EXPOSE 5000
VOLUME /registry
CMD REGISTRY_URL=${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT} /usr/bin/dockerregistry /config.yml

ADD config.yml /config.yml
