#
# This is the base image from which all OpenShift Origin images inherit. Only packages
# common to all downstream images should be here.
#
# The standard name for this image is openshift/origin-base
#
FROM centos:centos7

RUN INSTALL_PKGS="which git tar wget hostname sysvinit-tools util-linux bsdtar \
        socat ethtool device-mapper iptables e2fsprogs xfsprogs centos-release-openshift-origin" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

RUN yum-config-manager --enable centos-openshift-origin-testing
