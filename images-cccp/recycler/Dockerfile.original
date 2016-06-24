#
# This is the default OpenShift Origin persistent volume recycler image.
#
# The standard name for this image is openshift/origin-recycler
#
FROM scratch

ADD bin/recycle /usr/bin/recycle

ENTRYPOINT ["/usr/bin/recycle"]
