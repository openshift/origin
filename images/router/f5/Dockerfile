#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-f5-router
#
FROM openshift/origin-base

ADD bin/openshift /usr/bin/openshift
RUN ln -s /usr/bin/openshift /usr/bin/openshift-router

ENTRYPOINT ["/usr/bin/openshift-router", "--type=f5"]
