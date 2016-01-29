#
# VIP failover monitoring container for OpenShift Origin.
#
# ImageName: openshift/origin-keepalived-ipfailover
#
FROM openshift/origin-base

# TODO: systemd update from centos 7.1 -> 7.2 is broken, remove this once 7.2
# base images land
RUN yum swap -y -- remove systemd-container\* -- install systemd systemd-libs

RUN yum -y install kmod keepalived iproute psmisc nc net-tools && \
    yum clean all

ADD conf/ /var/lib/openshift/ipfailover/keepalived/conf/
ADD lib/  /var/lib/openshift/ipfailover/keepalived/lib/
ADD bin/  /var/lib/openshift/ipfailover/keepalived/bin/
ADD monitor.sh /var/lib/openshift/ipfailover/keepalived/

EXPOSE 1985
ENTRYPOINT ["/var/lib/openshift/ipfailover/keepalived/monitor.sh"]
