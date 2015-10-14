#
# This is an OpenShift Origin node image with integrated OpenvSwitch SDN
# If you do not require OVS SDN you should just use the origin image
#
# The standard name for this image is openshift/origin-node
#

FROM openshift/origin

# Based on work by: Peter Schiffer <pschiffe@redhat.com>
MAINTAINER Devan Goodwin <dgoodwin@redhat.com>

# We need to install openvswitch for the client portions, the daemons are expected
# to run in another container

ADD https://copr.fedoraproject.org/coprs/maxamillion/origin-next/repo/epel-7/maxamillion-origin-next-epel-7.repo /etc/yum.repos.d/
RUN yum install -y libmnl libnetfilter_conntrack openvswitch \
    libnfnetlink iptables iproute bridge-utils procps-ng ethtool socat openssl \
    binutils xz kmod-libs kmod sysvinit-tools device-mapper-libs dbus \
    && yum clean all

RUN mkdir -p /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet \
    /usr/lib/systemd/system/origin-node.service.d \
    /usr/lib/systemd/system/docker.service.d

# All of the following are placed in images/node by hack/build-images.sh
# They won't exist in the git checkout
ADD bin/* /usr/bin/
ADD libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/openshift-ovs-subnet /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/
ADD conf/openshift-sdn-ovs.conf /usr/lib/systemd/system/origin-node.service.d/
ADD lib/systemd/system/docker.service.d/docker-sdn-ovs.conf /usr/lib/systemd/system/docker.service.d/docker-sdn-ovs.conf
ADD scripts/* /usr/local/bin/
RUN chmod +x /usr/local/bin/* /usr/bin/openshift-* /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/openshift-ovs-subnet

VOLUME [ "/etc/origin/node", "/var/lib/origin" ]
WORKDIR /var/lib/origin

ENV HOME /root
ENV OPENSHIFT_CONTAINERIZED true
ENV KUBECONFIG /etc/origin/node/node.kubeconfig
ENTRYPOINT []
CMD [ "/usr/local/bin/origin-node-run.sh" ]
