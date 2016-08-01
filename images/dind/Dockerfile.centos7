#
# This image is used for running a host of an openshift dev cluster. This image is
# a development support image and should not be used in production environments.
#
# The standard name for this image is openshift/dind
#
FROM centos:centos7

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

## Install origin repo
RUN INSTALL_PKGS="centos-release-openshift-origin" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

## Install packages
RUN INSTALL_PKGS="git golang mercurial tar make findutils \
      gcc hostname bind-utils iproute iputils which procps-ng openssh-server \
      docker openvswitch bridge-utils ethtool iptables-services" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V --nofiles $INSTALL_PKGS && \
    yum clean all

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

## Hardlink init to another name to avoid having oci-systemd-hooks
## detect containers using this image as requiring read-only cgroup
## mounts.  dind containers should be run with --privileged to ensure
## cgroups mounted with read-write permissions.
RUN ln /usr/sbin/init /usr/sbin/dind_init

CMD ["/usr/sbin/dind_init"]
