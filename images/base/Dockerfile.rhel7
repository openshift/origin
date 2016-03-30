#
# This is the rhel7 base image from which all rhel7 based OpenShift Origin images
# inherit. Only packages common to all downstream images should be here.
#
# The standard name for this image is openshift/ose-base
#
FROM rhel7

RUN INSTALL_PKGS="which git tar wget hostname sysvinit-tools util-linux bsdtar \
    socat ethtool device-mapper iptables e2fsprogs xfsprogs" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all
