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

COPY scripts/* /usr/local/bin/
COPY system-container/system-container-wrapper.sh /usr/local/bin/
COPY system-container/manifest.json system-container/config.json.template system-container/service.template system-container/tmpfiles.template /exports/

RUN INSTALL_PKGS="origin-sdn-ovs libmnl libnetfilter_conntrack conntrack-tools openvswitch \
      libnfnetlink iptables iproute bridge-utils procps-ng ethtool socat openssl \
      binutils xz kmod-libs kmod sysvinit-tools device-mapper-libs dbus \
      iscsi-initiator-utils bind-utils" && \
    yum --enablerepo=origin-local-release install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    mkdir -p /usr/lib/systemd/system/origin-node.service.d /usr/lib/systemd/system/docker.service.d

# Copy the cni plugins to the host file system when they are present, so that they can be shared with cri-o
RUN if test -e /opt/cni/bin; then mkdir -p /exports/hostfs/opt/cni/bin/ && cp -r /opt/cni/bin/* /exports/hostfs/opt/cni/bin/; fi

LABEL io.k8s.display-name="OpenShift Origin Node" \
      io.k8s.description="This is a component of OpenShift Origin and contains the software for individual nodes when using SDN." \
      io.openshift.tags="openshift,node"

VOLUME /etc/origin/node
ENV KUBECONFIG=/etc/origin/node/node.kubeconfig

ENTRYPOINT [ "/usr/local/bin/origin-node-run.sh" ]
