#
# This is the NGINX router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-nginx-router
#
FROM openshift/origin

RUN INSTALL_PKGS="nginx" && \
    yum install -y "epel-release" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    mkdir -p /var/lib/nginx/router/{certs,cacerts} && \
    mkdir -p /var/lib/nginx/{conf,run,bin,log,logs} && \
    touch /var/lib/nginx/conf/{{os_http_be,os_edge_http_be,os_tcp_be,os_sni_passthrough,os_reencrypt,os_route_http_expose,os_route_http_redirect,cert_config,os_wildcard_domain}.map,nginx.config} && \
    setcap 'cap_net_bind_service=ep' /usr/sbin/nginx && \
    chown -R :0 /var/lib/nginx && \
    chown -R :0 /var/log/nginx && \
    chmod -R 777 /var/log/nginx && \
    chmod -R 777 /var/lib/nginx

COPY . /var/lib/nginx/

LABEL io.k8s.display-name="OpenShift Origin NGINX Router" \
      io.k8s.description="This is a component of OpenShift Origin and contains an NGINX instance that automatically exposes services within the cluster through routes, and offers TLS termination, reencryption, or SNI-passthrough on ports 80 and 443."
USER 1001
EXPOSE 80 443
WORKDIR /var/lib/nginx/conf
ENV TEMPLATE_FILE=/var/lib/nginx/conf/nginx-config.template \
    RELOAD_SCRIPT=/var/lib/nginx/reload-nginx
ENTRYPOINT ["/usr/bin/openshift-router"]
