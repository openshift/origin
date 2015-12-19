#
# This is an openvswitch image meant to enable OpenShift OVS based SDN
#
# The standard name for this image is openshift/openvswitch
#

FROM openshift/origin-base

MAINTAINER Scott Dodson <sdodson@redhat.com>

ADD https://copr.fedoraproject.org/coprs/maxamillion/origin-next/repo/epel-7/maxamillion-origin-next-epel-7.repo /etc/yum.repos.d/

RUN install_and_verify.sh openvswitch

ADD  scripts/* /usr/local/bin/
RUN chmod +x /usr/local/bin/*
VOLUME [ "/etc/openswitch" ]

ENV HOME /root
ENTRYPOINT [ "/usr/local/bin/ovs-run.sh" ]
