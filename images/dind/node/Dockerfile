#
# This is the base image for nodes of an openshift dind dev cluster.
#
# The standard name for this image is openshift/dind-node
#

FROM openshift/dind

## Install packages
RUN dnf -y update && dnf -y install\
 bind-utils\
 findutils\
 hostname\
 iproute\
 iputils\
 less\
 procps-ng\
 tar\
 which\
 # Node-specific packages
 ethtool\
 iptables-services\
 conntrack-tools\
 python-netaddr\
 python2-pyroute2\
 python2-requests\
 PyYAML\
 cri-o\
 cri-tools\
 openssl

# Remove the CRI-O CNI network configs so openshift-sdn's will be used instead
RUN rm -f /etc/cni/net.d/*

# A default deny firewall (either iptables or firewalld) is
# installed by default on non-cloud fedora and rhel, so all
# network plugins need to be able to work with one enabled.
RUN systemctl enable iptables.service

# Remove ratelimiting in the journal
COPY journald.conf /etc/systemd/journald.conf

# Ensure that master-to-kubelet communication will work with iptables
COPY iptables /etc/sysconfig/

COPY openshift-node.sh /usr/local/bin/
COPY openshift-dind-lib.sh /usr/local/bin/

COPY openshift-node.service /etc/systemd/system/
RUN systemctl enable openshift-node.service
# Ensure the working directory for the unit file exists
RUN mkdir -p /var/lib/origin

COPY openshift-enable-ssh-access.sh /usr/local/bin/
COPY openshift-enable-ssh-access.service /etc/systemd/system/
RUN systemctl enable openshift-enable-ssh-access.service

# SDN plugin setup
RUN mkdir -p /etc/cni/net.d
RUN mkdir -p /opt/cni/bin

# Symlink from the data path intended to be mounted as a volume to
# make reloading easy.  Revisit if/when dind becomes useful for more
# than dev/test.
RUN ln -sf /data/oc        /usr/local/bin/ && \
    ln -sf /data/hyperkube /usr/local/bin/ && \
    ln -sf /data/openshift-node-config /usr/local/bin/ && \
    ln -sf /data/openshift /usr/local/bin/openshift-deploy && \
    ln -sf /data/openshift /usr/local/bin/openshift-docker-build && \
    ln -sf /data/openshift /usr/local/bin/openshift-sti-build && \
    ln -sf /data/openshift /usr/local/bin/openshift-git-clone && \
    ln -sf /data/openshift /usr/local/bin/openshift-manage-dockerfile && \
    ln -sf /data/openshift /usr/local/bin/openshift-extract-image-content && \
    ln -sf /data/openshift /usr/local/bin/openshift-f5-router

ENV KUBECONFIG /data/openshift.local.config/master/admin.kubeconfig

RUN systemctl enable crio.service

# Default crio storage to vfs. overlay on overlayfs is not supported in crio.
RUN echo "CRIO_STORAGE_OPTIONS=--storage-driver vfs" > /etc/sysconfig/crio-storage
RUN sed -i 's/^plugin_dir .*/plugin_dir = "\/opt\/cni\/bin\/"/g' /etc/crio/crio.conf
RUN sed -i 's/^network_dir .*/network_dir = "\/etc\/kubernetes\/cni\/net.d\/"/g' /etc/crio/crio.conf
RUN sed -i 's/^registries .*/registries = [\n\t"docker.io",\n\t"registry.fedoraproject.org",\n\t"registry.access.redhat.com",\n\t"quay.io",/g' /etc/crio/crio.conf
