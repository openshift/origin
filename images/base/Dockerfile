#
# This is the base image from which all OpenShift Origin images inherit. Only packages
# common to all downstream images should be here.
#
# The standard name for this image is openshift/origin-base
#
FROM centos:centos7

# components from EPEL must be installed in a separate yum install step
RUN yum install -y git tar wget socat hostname sysvinit-tools util-linux epel-release ethtool && \
    yum clean all
