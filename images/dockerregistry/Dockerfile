#
# This is the official OpenShift Origin Docker Registry image.
#
# The standard name for this image is openshift3_beta/ose-docker-registry
#
FROM openshift3_beta/ose-base

ADD config.yml /config.yml
RUN yum install -y openshift-dockerregistry && \
    yum clean all

ENV REGISTRY_CONFIGURATION_PATH=/config.yml DISABLE_USER_AUTH=true

EXPOSE 5000
VOLUME /registry
CMD REGISTRY_URL=${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT} /usr/bin/dockerregistry /config.yml
