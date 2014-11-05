#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-haproxy-router
#
FROM openshift/origin-haproxy-router-base

ADD bin/openshift-router /usr/bin/openshift-router
EXPOSE 80
ENTRYPOINT ["/usr/bin/openshift-router"]
