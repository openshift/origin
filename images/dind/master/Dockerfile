#
# This image is for the master of an openshift dind dev cluster.
#
# The standard name for this image is openshift/dind-master
#

FROM openshift/dind-node

# Disable iptables on the master since it will prevent access to the
# openshift api from outside the master.
RUN systemctl disable iptables.service

COPY openshift-generate-master-config.sh /usr/local/bin/

COPY openshift-disable-master-node.sh /usr/local/bin/
COPY openshift-disable-master-node.service /etc/systemd/system/
RUN systemctl enable openshift-disable-master-node.service

COPY openshift-get-hosts.sh /usr/local/bin/
COPY openshift-add-to-hosts.sh /usr/local/bin/
COPY openshift-remove-from-hosts.sh /usr/local/bin/
COPY openshift-sync-etc-hosts.service /etc/systemd/system/
RUN systemctl enable openshift-sync-etc-hosts.service

COPY openshift-master.service /etc/systemd/system/
RUN systemctl enable openshift-master.service

RUN mkdir -p /etc/systemd/system/openshift-node.service.d
COPY master-node.conf /etc/systemd/system/openshift-node.service.d/
