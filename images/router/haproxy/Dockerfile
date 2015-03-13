#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift3_beta/ose-haproxy-router
#
FROM openshift3_beta/ose-haproxy-router-base

RUN yum -y install openshift && yum clean all

ADD conf/ /var/lib/haproxy/conf/
ADD reload-haproxy /var/lib/haproxy/reload-haproxy
RUN ln -s /usr/bin/openshift /usr/bin/openshift-router

EXPOSE 80
ENTRYPOINT ["/usr/bin/openshift-router", "--template=/var/lib/haproxy/conf/haproxy-config.template", "--reload=/var/lib/haproxy/reload-haproxy"]
