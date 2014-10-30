#
# This is the official OpenShift Origin image. It has as its entrypoint the OpenShift
# all-in-one binary.
#
# The standard name for this image is openshift/origin
#
FROM openshift/origin-base

ADD bin/openshift        /usr/bin/openshift
ADD bin/openshift-deploy /usr/bin/openshift-deploy
ADD bin/openshift-router /usr/bin/openshift-router

ENV HOME /root
ENTRYPOINT ["/usr/bin/openshift"]
