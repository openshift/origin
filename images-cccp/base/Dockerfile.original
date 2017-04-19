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

RUN INSTALL_PKGS="which git tar wget hostname sysvinit-tools util-linux bsdtar epel-release \
        socat ethtool device-mapper iptables e2fsprogs xfsprogs" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all
