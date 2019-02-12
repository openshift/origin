#
# This is the OpenShift OpenvSwitch SDN image. It currently contains the remaining
# binaries for the node but will be split into a new image soon.
#
# The standard name for this image is openshift/origin-node
#
FROM openshift/origin-cli

RUN INSTALL_PKGS=" \
      origin-sdn-ovs origin-clients \
      socat ethtool iptables nmap-ncat dbus tcpdump \
      libmnl libnetfilter_conntrack conntrack-tools \
      libnfnetlink iproute bridge-utils procps-ng openssl \
      " && \
    yum --enablerepo=origin-local-release install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

LABEL io.k8s.display-name="OpenShift Node" \
      io.k8s.description="This is a component of OpenShift and contains the software for individual nodes when using SDN." \
      io.openshift.tags="openshift,node"

ENV OPENSHIFT_CONTAINERIZED=true

ENTRYPOINT [ "/usr/local/bin/openshift-sdn" ]
