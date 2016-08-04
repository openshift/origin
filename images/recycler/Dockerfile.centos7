#
# This is the default OpenShift Origin persistent volume recycler image.
#
# The standard name for this image is openshift/origin-recycler
#
FROM centos:centos7

## Install origin repo
RUN INSTALL_PKGS="centos-release-openshift-origin" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

## Install packages
RUN INSTALL_PKGS="origin-recycle" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

ENTRYPOINT ["/usr/bin/recycle"]
