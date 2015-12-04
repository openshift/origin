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
  console-getty.service dnf-makecache.service lvm2-lvmetad.service \
  systemd-udevd.service systemd-udev-hwdb-update.service \
  systemd-udev-trigger.service docker-storage-setup.service
RUN cp /usr/lib/systemd/system/dbus.service /etc/systemd/system/; \
  sed -i 's/OOMScoreAdjust=-900//' /etc/systemd/system/dbus.service

VOLUME ["/run", "/tmp"]

## Install packages
RUN yum -y update && yum -y install git golang hg tar make \
  hostname bind-utils iproute iputils which procps-ng openssh-server \
  # Node-specific packages
  docker openvswitch bridge-utils ethtool \
  && yum clean all

# sshd should be enabled as needed
RUN systemctl disable sshd.service

## Configure dind
ENV DIND_COMMIT 81aa1b507f51901eafcfaad70a656da376cf937d
RUN curl -fL "https://raw.githubusercontent.com/docker/docker/${DIND_COMMIT}/hack/dind" \
  -o /usr/local/bin/dind && chmod +x /usr/local/bin/dind
RUN mkdir -p /etc/systemd/system/docker.service.d
COPY dind.conf /etc/systemd/system/docker.service.d/

RUN systemctl enable docker

VOLUME /var/lib/docker

CMD ["/usr/sbin/init"]
