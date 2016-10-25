#
# This is an OpenShift Origin node image with integrated OpenvSwitch SDN
# If you do not require OVS SDN use the openshift/origin image instead.
#
# This image expects to have a volume mounted at /etc/origin/node that contains
# a KUBECONFIG file giving the node permission to talk to the master and a
# node configuration file.
#
# The standard name for this image is openshift/node
#
FROM openshift/origin

COPY usr/bin/* /usr/bin/
COPY opt/cni/bin/* /opt/cni/bin/
COPY etc/cni/net.d/* /etc/cni/net.d/
COPY conf/openshift-sdn-ovs.conf /usr/lib/systemd/system/origin-node.service.d/
COPY scripts/* /usr/local/bin/

RUN curl -L -o /etc/yum.repos.d/origin-next-epel-7.repo https://copr.fedoraproject.org/coprs/maxamillion/origin-next/repo/epel-7/maxamillion-origin-next-epel-7.repo && \
    INSTALL_PKGS="libmnl libnetfilter_conntrack openvswitch \
      libnfnetlink iptables iproute bridge-utils procps-ng ethtool socat openssl \
      binutils xz kmod-libs kmod sysvinit-tools device-mapper-libs dbus \
      ceph-common iscsi-initiator-utils" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    mkdir -p /usr/lib/systemd/system/origin-node.service.d /usr/lib/systemd/system/docker.service.d && \
    chmod +x /usr/local/bin/* /usr/bin/openshift-* /opt/cni/bin/*

LABEL io.k8s.display-name="OpenShift Origin Node" \
      io.k8s.description="This is a component of OpenShift Origin and contains the software for individual nodes when using SDN."
VOLUME /etc/origin/node
ENV KUBECONFIG=/etc/origin/node/node.kubeconfig
ENTRYPOINT [ "/usr/local/bin/origin-node-run.sh" ]
