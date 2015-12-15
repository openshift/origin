#
# VIP failover monitoring container for OpenShift Origin.
#
# ImageName: openshift/origin-keepalived-ipfailover
#
FROM openshift/origin-base

RUN install_and_verify.sh kmod keepalived iproute psmisc nmap-ncat net-tools

ADD conf/ /var/lib/openshift/ipfailover/keepalived/conf/
ADD lib/  /var/lib/openshift/ipfailover/keepalived/lib/
ADD bin/  /var/lib/openshift/ipfailover/keepalived/bin/
ADD monitor.sh /var/lib/openshift/ipfailover/keepalived/

EXPOSE 1985
ENTRYPOINT ["/var/lib/openshift/ipfailover/keepalived/monitor.sh"]
