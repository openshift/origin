#
# This is the image that executes a git clone inside Origin. It expects the
# following environment variables:
#
#   BUILD - JSON string containing the openshift build object
#
# The standard name for this image is openshift/origin-git-clone
#
FROM openshift/origin

LABEL io.k8s.display-name="OpenShift Origin Extract Image Content" \
      io.k8s.description="This is a component of OpenShift Origin and is responsible for extracting build input content from images."
ENTRYPOINT ["/usr/bin/openshift-extract-image-content"]
