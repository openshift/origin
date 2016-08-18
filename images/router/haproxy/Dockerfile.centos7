#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-haproxy-router
#
FROM openshift/origin-haproxy-router-base

RUN INSTALL_PKGS="origin" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

ADD conf/ /var/lib/haproxy/conf/
ADD reload-haproxy /var/lib/haproxy/reload-haproxy

#
# Note: /var is changed to 777 to allow access when running this container as a non-root uid
#       this is temporary and should be removed when the container is switch to an empty-dir
#       with gid support.
# Note2: cap_net_bind_service must be granted to haproxy to allow a non-root uid to bind to low ports
#
RUN chmod -R 777 /var && \
    setcap 'cap_net_bind_service=ep' /usr/sbin/haproxy
WORKDIR /var/lib/haproxy/conf

EXPOSE 80
ENV TEMPLATE_FILE=/var/lib/haproxy/conf/haproxy-config.template \
    RELOAD_SCRIPT=/var/lib/haproxy/reload-haproxy
ENTRYPOINT ["/usr/bin/openshift-router"]
