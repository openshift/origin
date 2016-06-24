#
# This is the egress router for OpenShift Origin
#
# The standard name for this image is openshift/origin-egress-router

FROM openshift/origin-base

RUN yum install -y iproute && \
    yum clean all

ADD egress-router.sh /bin/egress-router.sh

ENTRYPOINT /bin/egress-router.sh
