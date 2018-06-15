#
# This is the NGINX router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-nginx-router
#
FROM openshift/origin-control-plane

ENV NGINX_VERSION 1.13.12-1.el7_4.ngx

COPY nginx.repo /etc/yum.repos.d/ 

RUN yum install -y nginx-${NGINX_VERSION} && \
    yum clean all && \
    mkdir -p /var/lib/nginx/router/{certs,cacerts} && \
    mkdir -p /var/lib/nginx/{conf,run,log,cache} && \
    touch /var/lib/nginx/conf/nginx.conf && \
    setcap 'cap_net_bind_service=ep' /usr/sbin/nginx && \
    chown -R :0 /var/lib/nginx && \
    chmod -R g+w /var/lib/nginx && \
    ln -sf /var/lib/nginx/log/error.log /var/log/nginx/error.log && \
    rm /etc/yum.repos.d/nginx.repo

COPY . /var/lib/nginx/

LABEL io.k8s.display-name="OpenShift Origin NGINX Router" \
      io.k8s.description="This is a component of OpenShift Origin and contains an NGINX instance that automatically exposes services within the cluster through routes, and offers TLS termination, reencryption, or SNI-passthrough on ports 80 and 443."
USER 1001
EXPOSE 80 443
WORKDIR /var/lib/nginx/conf
ENV TEMPLATE_FILE=/var/lib/nginx/conf/nginx-config.template \
    RELOAD_SCRIPT=/var/lib/nginx/reload-nginx
ENTRYPOINT ["/usr/bin/openshift-router", "--working-dir=/var/lib/nginx/router"]
