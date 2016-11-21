#
# This is the image that executes a git clone inside Origin. It expects the
# following environment variables:
#
#   BUILD - JSON string containing the openshift build object
#
# The standard name for this image is openshift/origin-git-clone
#
FROM openshift/origin

LABEL io.k8s.display-name="OpenShift Origin Manage Dockerfile" \
      io.k8s.description="This is a component of OpenShift Origin and is responsible for updating the Dockerfile for docker builds."
ENTRYPOINT ["/usr/bin/openshift-manage-dockerfile"]
