#
# This image is used for running a host of an openshift dev cluster. This image is
# a development support image and should not be used in production environments.
#
# The standard name for this image is openshift/dind
#
FROM fedora:24

## Configure systemd to run in a container
ENV container=docker

RUN systemctl mask\
 auditd.service\
 console-getty.service\
 dev-hugepages.mount\
 dnf-makecache.service\
 docker-storage-setup.service\
 getty.target\
 lvm2-lvmetad.service\
 sys-fs-fuse-connections.mount\
 systemd-logind.service\
 systemd-remount-fs.service\
 systemd-udev-hwdb-update.service\
 systemd-udev-trigger.service\
 systemd-udevd.service\
 systemd-vconsole-setup.service

RUN cp /usr/lib/systemd/system/dbus.service /etc/systemd/system/; \
  sed -i 's/OOMScoreAdjust=-900//' /etc/systemd/system/dbus.service

VOLUME ["/run", "/tmp"]

## Install packages
RUN dnf -y update && dnf -y install git golang hg tar make findutils \
  gcc hostname bind-utils iproute iputils which procps-ng openssh-server \
  # Node-specific packages
  docker openvswitch bridge-utils ethtool iptables-services \
  && dnf clean all

# sshd should be enabled as needed
RUN systemctl disable sshd.service

# Default storage to vfs.  overlay will be enabled at runtime if available.
RUN echo "DOCKER_STORAGE_OPTIONS=--storage-driver vfs" >\
 /etc/sysconfig/docker-storage

RUN systemctl enable docker

VOLUME /var/lib/docker

## Hardlink init to another name to avoid having oci-systemd-hooks
## detect containers using this image as requiring read-only cgroup
## mounts.  dind containers should be run with --privileged to ensure
## cgroups mounted with read-write permissions.
RUN ln /usr/sbin/init /usr/sbin/dind_init

CMD ["/usr/sbin/dind_init"]
