#
# This image is used for running a host of an openshift dev cluster.
#
# The standard name for this image is openshift/dind
#

FROM fedora:21

## Configure systemd to run in a container
ENV container=docker

RUN systemctl mask systemd-remount-fs.service dev-hugepages.mount \
  sys-fs-fuse-connections.mount systemd-logind.service getty.target \
  console-getty.service dnf-makecache.service
RUN cp /usr/lib/systemd/system/dbus.service /etc/systemd/system/; \
  sed -i 's/OOMScoreAdjust=-900//' /etc/systemd/system/dbus.service

VOLUME ["/run", "/tmp"]

## Install packages
RUN yum -y update && yum -y install git golang hg tar make \
  hostname bind-utils iproute iputils which procps-ng \
  # Node-specific packages
  docker openvswitch bridge-utils ethtool \
  && yum clean all

## Configure dind
ENV DIND_COMMIT 4e899d64e020a67ca05f913d354aa8d99a341a7b
RUN curl -fL "https://raw.githubusercontent.com/docker/docker/${DIND_COMMIT}/hack/dind" \
  -o /usr/local/bin/dind && chmod +x /usr/local/bin/dind

RUN mv /lib/systemd/system/docker.service /lib/systemd/system/docker.service.bak
COPY docker.service /lib/systemd/system/

RUN systemctl enable docker.service

VOLUME /var/lib/docker

CMD ["/usr/sbin/init"]
