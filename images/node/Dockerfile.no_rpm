#
# This is an OpenShift SDN image.
#
# This image expects to have a volume mounted at /etc/origin/node that contains
# a KUBECONFIG file giving the node permission to talk to the master and a
# node configuration file.
#
# The standard name for this image is openshift/origin-node
#

# Build environment
FROM openshift/origin-release:golang-1.10 AS build
WORKDIR /go/src/github.com/openshift/origin
COPY Makefile .
COPY cmd      cmd/
COPY hack     hack/
COPY pkg      pkg/
COPY Godeps   Godeps/
COPY tools    tools/
COPY vendor   vendor/
# TODO: only needed for git info, can we improve this?
COPY .git     .git/
RUN  make build-network

# Runtime environment
FROM openshift/origin-cli AS runtime

COPY --from=build /go/src/github.com/openshift/origin/_output/local/bin/linux/amd64/openshift-sdn /usr/bin/
COPY --from=build /go/src/github.com/openshift/origin/_output/local/bin/linux/amd64/sdn-cni-plugin /opt/cni/bin/openshift-sdn
COPY --from=build /go/src/github.com/openshift/origin/_output/local/bin/linux/amd64/loopback /opt/cni/bin/
COPY --from=build /go/src/github.com/openshift/origin/_output/local/bin/linux/amd64/host-local /opt/cni/bin/

RUN INSTALL_PKGS=" \
      openvswitch socat ethtool iptables nmap-ncat \
      libmnl libnetfilter_conntrack conntrack-tools \
      libnfnetlink iproute bridge-utils procps-ng openssl \
      binutils xz sysvinit-tools dbus \
      " && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    mkdir -p /etc/sysconfig/cni/net.d && \
    yum clean all

LABEL io.k8s.display-name="OpenShift Node" \
      io.k8s.description="This is a component of OpenShift and contains the software for individual nodes when using SDN." \
      io.openshift.tags="openshift,sdn"

ENTRYPOINT [ "/usr/local/bin/openshift-sdn" ]
