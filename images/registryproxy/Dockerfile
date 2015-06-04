# This is a pod used to proxy the OpenShift Registry from the ServiceIP to the default interface.
#   The main use case is to expose the registry to a Host OS using a VM's Guest IP interface
#
# The standard name for this image is openshift/origin-registry-proxy
#
FROM openshift/origin-base

EXPOSE 5000
RUN yum install 3proxy -y && yum clean all
CMD [ "sh", "-cx", "/bin/tcppm -l ${DOCKER_REGISTRY_SERVICE_PORT} ${DOCKER_REGISTRY_SERVICE_HOST} ${DOCKER_REGISTRY_SERVICE_PORT}" ]
