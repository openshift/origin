#
# This is the egress router L4 DNS proxy for OpenShift Origin
#
# The standard name for this image is openshift/origin-egress-dns-proxy

FROM openshift/origin-base

# HAProxy 1.6+ version is needed to leverage DNS resolution at runtime.
RUN INSTALL_PKGS="haproxy18 rsyslog" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    mkdir -p /var/lib/haproxy/{run,log} && \
    mkdir -p /etc/haproxy && \
    setcap 'cap_net_bind_service=ep' /usr/sbin/haproxy && \
    chown -R :0 /var/lib/haproxy && \
    chmod -R g+w /var/lib/haproxy && \
    touch /etc/haproxy/haproxy.cfg

ADD egress-dns-proxy.sh /bin/egress-dns-proxy.sh

ENTRYPOINT /bin/egress-dns-proxy.sh

