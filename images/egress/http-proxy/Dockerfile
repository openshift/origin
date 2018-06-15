#
# This is the egress router HTTP proxy for OpenShift Origin
#
# The standard name for this image is openshift/origin-egress-http-proxy

FROM openshift/origin-base

RUN INSTALL_PKGS="squid" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    rmdir /var/log/squid /var/spool/squid && \
    rm -f /etc/squid/squid.conf

ADD egress-http-proxy.sh /bin/egress-http-proxy.sh

ENTRYPOINT /bin/egress-http-proxy.sh
