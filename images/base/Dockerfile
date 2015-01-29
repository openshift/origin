#
# This is the base image from which all OpenShift Origin images inherit. Only packages
# common to all downstream images should be here.
#
# The standard name for this image is openshift3_beta/ose-base
#
FROM registry.access.redhat.com/rhel

RUN echo [brew] > /etc/yum.repos.d/brew.repo;\
    echo baseurl = http://buildvm-devops.usersys.redhat.com/puddle/build/OpenShiftEnterprise/3.0/latest/RH7-RHOSE-3.0/x86_64/os/ >> /etc/yum.repos.d/brew.repo;\
    echo ui_repoid_vars = releasever basearch >> /etc/yum.repos.d/brew.repo;\
    echo name = brew >> /etc/yum.repos.d/brew.repo;\
    echo gpgcheck = 0 >> /etc/yum.repos.d/brew.repo;\
    echo enabled = 1 >> /etc/yum.repos.d/brew.repo

RUN yum install -y git tar wget socat hostname yum-utils --disablerepo=\* --enablerepo=rhel-7-server-rpms && \
    yum-config-manager --disable rhel-7-server-rt-rpms && \
    yum clean all
