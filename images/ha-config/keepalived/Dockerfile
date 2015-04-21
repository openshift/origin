#
# VIP failover monitoring container for OpenShift Origin.
#
# ImageName: openshift/origin-keepalived-ha-config
#
# FROM fedora
FROM openshift/origin-base

RUN yum -y install kmod keepalived iproute psmisc nc net-tools

ADD conf/ /var/lib/openshift/ha-config/keepalived/conf/
ADD lib/  /var/lib/openshift/ha-config/keepalived/lib/
ADD bin/  /var/lib/openshift/ha-config/keepalived/bin/
ADD monitor.sh /var/lib/openshift/ha-config/keepalived/

EXPOSE 1985
ENTRYPOINT ["/var/lib/openshift/ha-config/keepalived/monitor.sh"]
