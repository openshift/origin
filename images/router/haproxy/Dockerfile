#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-haproxy-router
#
FROM openshift/origin

RUN mkdir -p /var/lib/haproxy/router/{certs,cacerts} && \
    setcap 'cap_net_bind_service=ep' /usr/sbin/haproxy

COPY . /var/lib/haproxy/

LABEL io.k8s.display-name="OpenShift Origin HAProxy Router" \
      io.k8s.description="This is a component of OpenShift Origin and contains an HAProxy instance that automatically exposes services within the cluster through routes, and offers TLS termination, reencryption, or SNI-passthrough on ports 80 and 443."
USER 1001
EXPOSE 80 443
WORKDIR /var/lib/haproxy/conf
ENV TEMPLATE_FILE=/var/lib/haproxy/conf/haproxy-config.template \
    RELOAD_SCRIPT=/var/lib/haproxy/reload-haproxy
ENTRYPOINT ["/usr/bin/openshift-router"]
