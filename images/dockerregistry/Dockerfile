#
# This is the integrated OpenShift Origin Docker registry. It is configured to
# publish metadata to OpenShift to provide automatic management of images on push.
#
# The standard name for this image is openshift/origin-docker-registry
#
FROM openshift/origin-base

COPY config.yml $REGISTRY_CONFIGURATION_PATH
COPY bin/dockerregistry /dockerregistry

LABEL io.k8s.display-name="OpenShift Origin Image Registry" \
      io.k8s.description="This is a component of OpenShift Origin and exposes a Docker registry that is integrated with the cluster for authentication and management."

# The registry doesn't require a root user.
USER 1001
EXPOSE 5000
VOLUME /registry
ENV REGISTRY_CONFIGURATION_PATH=/config.yml

CMD DOCKER_REGISTRY_URL=${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT} /dockerregistry ${REGISTRY_CONFIGURATION_PATH}
