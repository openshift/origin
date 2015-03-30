#
# Router failover (VIPs) monitoring container for OpenShift Origin.
#
# ImageName: openshift/origin-keepalived-failover-monitor
#
# FROM fedora
FROM openshift/origin-base

RUN yum -y install kmod keepalived iproute psmisc nc net-tools

ADD conf/ /var/lib/openshift/keepalived/failover-monitor/conf/
ADD lib/  /var/lib/openshift/keepalived/failover-monitor/lib/
ADD bin/  /var/lib/openshift/keepalived/failover-monitor/bin/
ADD tests/  /var/lib/openshift/keepalived/failover-monitor/tests/
ADD monitor.sh /var/lib/openshift/keepalived/failover-monitor/

ENTRYPOINT ["/var/lib/openshift/keepalived/failover-monitor/monitor.sh"]
