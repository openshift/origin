#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-haproxy-router
#
FROM openshift/origin-cli

RUN yum -y install wget gcc make pcre-devel zlib-devel openssl-devel systemd-devel && \
    wget http://www.haproxy.org/download/1.8/src/haproxy-1.8.26.tar.gz && \
    tar xvzf haproxy-1.8.26.tar.gz && \
    groupadd haproxy && \
    useradd -g haproxy haproxy && \
    cd haproxy-1.8.26 && make CPU="generic" TARGET="linux2628" USE_OPENSSL=1 USE_PCRE=1 USE_ZLIB=1 USE_LINUX_TPROXY=1 DEFINE=-DTCP_USER_TIMEOUT=18 USE_SYSTEMD=1 && make install && \
    cd .. && rm -rf haproxy-1.8.26 && \
    mkdir -p /var/lib/containers/router/ && \
    mkdir -p /var/lib/haproxy/{conf,run,bin,log} && \
    touch /var/lib/haproxy/conf/{host_be.map,host_be_ws.map,host_be_ressl.map,host_be_sni.map,haproxy.config} && \
    yum -y remove wget gcc make pcre-devel zlib-devel openssl-devel systemd-devel && \
    INSTALL_PKGS="rsyslog sysvinit-tools" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    mv ./usr/local/sbin/haproxy /usr/sbin/haproxy && \
    mkdir -p /var/lib/haproxy/router/{certs,cacerts,whitelists} && \
    mkdir -p /var/lib/haproxy/{conf/.tmp,run,bin,log} && \
    touch /var/lib/haproxy/conf/{{os_http_be,os_edge_reencrypt_be,os_tcp_be,os_sni_passthrough,os_route_http_redirect,cert_config,os_wildcard_domain}.map,haproxy.config} && \
    setcap 'cap_net_bind_service=ep' /usr/sbin/haproxy && \
    chown -R :0 /var/lib/haproxy && \
    chmod -R g+w /var/lib/haproxy

COPY . /var/lib/haproxy/

LABEL io.k8s.display-name="OpenShift Origin HAProxy Router" \
      io.k8s.description="This is a component of OpenShift Origin and contains an HAProxy instance that automatically exposes services within the cluster through routes, and offers TLS termination, reencryption, or SNI-passthrough on ports 80 and 443." \
      io.openshift.tags="openshift,router,haproxy"
USER 1001
EXPOSE 80 443
WORKDIR /var/lib/haproxy/conf
ENV TEMPLATE_FILE=/var/lib/haproxy/conf/haproxy-config.template \
    RELOAD_SCRIPT=/var/lib/haproxy/reload-haproxy
ENTRYPOINT ["/usr/bin/openshift-router"]
