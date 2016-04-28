#
# This is the default OpenShift Origin persistent volume recycler image.
#
# The standard name for this image is openshift/origin-recycler
#
FROM openshift/origin

LABEL io.k8s.display-name="OpenShift Origin Volume Recycler" \
      io.k8s.description="This is a component of OpenShift Origin and is used to prepare persistent volumes for reuse after they are deleted."
ENTRYPOINT ["/usr/bin/openshift-recycle"]
