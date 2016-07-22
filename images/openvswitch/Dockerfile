#
# This is an openvswitch image meant to enable OpenShift OVS based SDN
#
# The standard name for this image is openshift/openvswitch
#
FROM openshift/origin-base

COPY scripts/* /usr/local/bin/

RUN curl -L -o /etc/yum.repos.d/origin-next-epel-7.repo https://copr.fedoraproject.org/coprs/maxamillion/origin-next/repo/epel-7/maxamillion-origin-next-epel-7.repo && \
    INSTALL_PKGS="openvswitch" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    chmod +x /usr/local/bin/*

LABEL io.k8s.display-name="OpenShift Origin OpenVSwitch Daemon" \
      io.k8s.description="This is a component of OpenShift Origin and runs an OpenVSwitch daemon process."
VOLUME /etc/openswitch
ENV HOME /root
ENTRYPOINT ["/usr/local/bin/ovs-run.sh"]
