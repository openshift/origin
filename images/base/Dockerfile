#
# This is the base image from which all OpenShift Origin images inherit. Only packages
# common to all downstream images should be here.
#
# The standard name for this image is openshift/origin-base
#
FROM centos:centos7

# components from EPEL must be installed in a separate yum install step

# TODO: systemd update from centos 7.1 -> 7.2 is broken, remove this once 7.2
# base images land
RUN yum swap -y -- remove systemd-container\* -- install systemd systemd-libs

# The install_and_verify.sh script installs packages using yum and verifies that
# the packages were really installed and not skipped.
RUN echo -e "#!/bin/sh -ex\nyum install --setopt=tsflags=nodocs -y \"\$@\" && \
      yum clean all && rpm -V \"\$@\"" > /usr/bin/install_and_verify.sh && \
      chmod +x /usr/bin/install_and_verify.sh

RUN install_and_verify.sh git tar wget hostname sysvinit-tools util-linux bsdtar \
                 epel-release socat ethtool device-mapper iptables
