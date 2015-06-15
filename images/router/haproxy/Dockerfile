#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-haproxy-router
#
FROM openshift/origin-haproxy-router-base

ADD conf/ /var/lib/haproxy/conf/
ADD reload-haproxy /var/lib/haproxy/reload-haproxy
ADD bin/openshift /usr/bin/openshift

#
# Note: /var is changed to 777 to allow access when running this container as a non-root uid
#       this is temporary and should be removed when the container is switch to an empty-dir
#       with gid support.
# Note2: cap_net_bind_service must be granted to haproxy to allow a non-root uid to bind to low ports
#
RUN ln -s /usr/bin/openshift /usr/bin/openshift-router && \
    chmod -R 777 /var && \
    setcap 'cap_net_bind_service=ep' /usr/sbin/haproxy
WORKDIR /var/lib/haproxy/conf

EXPOSE 80
ENTRYPOINT ["/usr/bin/openshift-router", "--template=/var/lib/haproxy/conf/haproxy-config.template", "--reload=/var/lib/haproxy/reload-haproxy"]
