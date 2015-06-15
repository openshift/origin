#
# This is the integrated OpenShift Origin Docker registry. It is configured to
# publish metadata to OpenShift to provide automatic management of images on push.
#
# The standard name for this image is openshift3/ose-docker-registry
#
FROM openshift3/ose-base

ADD config.yml /config.yml
RUN yum install -y openshift-dockerregistry && \
    yum clean all

ENV REGISTRY_CONFIGURATION_PATH=/config.yml

RUN yum install -y tree findutils epel-release && \
    yum clean all

EXPOSE 5000
VOLUME /registry
CMD REGISTRY_URL=${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT} /usr/bin/dockerregistry /config.yml
