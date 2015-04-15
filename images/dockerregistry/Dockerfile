FROM openshift/origin-base

ADD config.yml /config.yml
ADD bin/dockerregistry /dockerregistry

ENV REGISTRY_CONFIGURATION_PATH=/config.yml

EXPOSE 5000
VOLUME /registry
CMD REGISTRY_URL=${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT} /dockerregistry /config.yml
